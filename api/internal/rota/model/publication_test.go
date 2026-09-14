package model

import (
	"errors"
	"testing"
	"time"
)

func TestResolvePublicationStateUsesEffectiveWindowsWithoutMutatingStoredState(t *testing.T) {
	start := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	publication := &Publication{
		State:              PublicationStateDraft,
		SubmissionStartAt:  start,
		SubmissionEndAt:    start.Add(2 * time.Hour),
		PlannedActiveFrom:  start.Add(3 * time.Hour),
		PlannedActiveUntil: end,
	}

	if got := ResolvePublicationState(publication, start.Add(-time.Minute)); got != PublicationStateDraft {
		t.Fatalf("before submission state = %q, want DRAFT", got)
	}
	if got := ResolvePublicationState(publication, start.Add(time.Hour)); got != PublicationStateCollecting {
		t.Fatalf("collecting state = %q, want COLLECTING", got)
	}
	if got := ResolvePublicationState(publication, start.Add(4*time.Hour)); got != PublicationStateAssigning {
		t.Fatalf("assigning state = %q, want ASSIGNING", got)
	}
	if publication.State != PublicationStateDraft {
		t.Fatalf("ResolvePublicationState mutated stored state to %q", publication.State)
	}

	active := *publication
	active.State = PublicationStateActive
	if got := ResolvePublicationState(&active, end.Add(-time.Second)); got != PublicationStateActive {
		t.Fatalf("active state before end = %q, want ACTIVE", got)
	}
	if got := ResolvePublicationState(&active, end); got != PublicationStateEnded {
		t.Fatalf("active state at end = %q, want ENDED", got)
	}
}

func TestIsValidOccurrenceRejectsWrongWeekdayWindowAndPast(t *testing.T) {
	publication := &Publication{
		PlannedActiveFrom:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PlannedActiveUntil: time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
	}
	slot := &TemplateSlot{
		Weekdays:  []int{1},
		StartTime: "09:00",
		EndTime:   "10:00",
	}

	monday := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	if err := IsValidOccurrence(publication, slot, monday, time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("valid occurrence rejected: %v", err)
	}
	for _, test := range []struct {
		name string
		date time.Time
		now  time.Time
		want error
	}{
		{
			name: "wrong weekday",
			date: time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC),
			now:  time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
			want: ErrOccurrenceWeekday,
		},
		{
			name: "outside window",
			date: time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
			now:  time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
			want: ErrOccurrenceOutsideWindow,
		},
		{
			name: "already started",
			date: monday,
			now:  time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC),
			want: ErrOccurrenceInPast,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := IsValidOccurrence(publication, slot, test.date, test.now)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}
