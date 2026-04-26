package model

import (
	"errors"
	"time"
)

var (
	ErrInvalidPublicationWindow = errors.New("invalid publication window")
	ErrInvalidOccurrenceDate    = errors.New("invalid occurrence date")
	ErrOccurrenceWeekday        = errors.New("occurrence weekday mismatch")
	ErrOccurrenceOutsideWindow  = errors.New("occurrence outside publication window")
	ErrOccurrenceInPast         = errors.New("occurrence start is not in the future")
	ErrPublicationAlreadyExists = errors.New("publication already exists")
	ErrPublicationNotFound      = errors.New("publication not found")
	ErrPublicationNotDeletable  = errors.New("publication not deletable")
	ErrPublicationNotCollecting = errors.New("publication not collecting")
	ErrPublicationNotMutable    = errors.New("publication not mutable")
	ErrPublicationNotAssigning  = errors.New("publication not assigning")
	ErrPublicationNotPublished  = errors.New("publication not published")
	ErrPublicationNotActive     = errors.New("publication not active")
	ErrNotQualified             = errors.New("not qualified")
)

type PublicationState string

const (
	PublicationStateDraft      PublicationState = "DRAFT"
	PublicationStateCollecting PublicationState = "COLLECTING"
	PublicationStateAssigning  PublicationState = "ASSIGNING"
	PublicationStatePublished  PublicationState = "PUBLISHED"
	PublicationStateActive     PublicationState = "ACTIVE"
	PublicationStateEnded      PublicationState = "ENDED"
)

type Publication struct {
	ID                 int64
	TemplateID         int64
	TemplateName       string
	Name               string
	Description        string
	State              PublicationState
	SubmissionStartAt  time.Time
	SubmissionEndAt    time.Time
	PlannedActiveFrom  time.Time
	PlannedActiveUntil time.Time
	ActivatedAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type AvailabilitySubmission struct {
	ID            int64
	PublicationID int64
	UserID        int64
	SlotID        int64
	PositionID    int64
	CreatedAt     time.Time
}

type SlotPositionRef struct {
	SlotID     int64
	PositionID int64
}

func ResolvePublicationState(publication *Publication, now time.Time) PublicationState {
	if publication == nil {
		return PublicationStateDraft
	}

	switch publication.State {
	case PublicationStateEnded:
		return PublicationStateEnded
	case PublicationStateActive:
		if !now.Before(publication.PlannedActiveUntil) {
			return PublicationStateEnded
		}
		return PublicationStateActive
	case PublicationStatePublished:
		return publication.State
	case "":
		return PublicationStateDraft
	}

	if !now.Before(publication.SubmissionEndAt) {
		return PublicationStateAssigning
	}
	if !now.Before(publication.SubmissionStartAt) {
		return PublicationStateCollecting
	}

	return PublicationStateDraft
}

func OccurrenceStart(slot *TemplateSlot, occurrenceDate time.Time) (time.Time, error) {
	if slot == nil {
		return time.Time{}, ErrInvalidOccurrenceDate
	}

	startClock, err := time.Parse("15:04", slot.StartTime)
	if err != nil {
		return time.Time{}, err
	}
	date := NormalizeOccurrenceDate(occurrenceDate)
	return time.Date(
		date.Year(),
		date.Month(),
		date.Day(),
		startClock.Hour(),
		startClock.Minute(),
		0,
		0,
		time.UTC,
	), nil
}

func NormalizeOccurrenceDate(date time.Time) time.Time {
	utc := date.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func IsValidOccurrence(
	publication *Publication,
	slot *TemplateSlot,
	occurrenceDate time.Time,
	now time.Time,
) error {
	if publication == nil || slot == nil || occurrenceDate.IsZero() {
		return ErrInvalidOccurrenceDate
	}

	date := NormalizeOccurrenceDate(occurrenceDate)
	if weekdayToSlotValue(date.Weekday()) != slot.Weekday {
		return errors.Join(ErrInvalidOccurrenceDate, ErrOccurrenceWeekday)
	}

	start, err := OccurrenceStart(slot, date)
	if err != nil {
		return errors.Join(ErrInvalidOccurrenceDate, err)
	}
	if start.Before(publication.PlannedActiveFrom) || !start.Before(publication.PlannedActiveUntil) {
		return errors.Join(ErrInvalidOccurrenceDate, ErrOccurrenceOutsideWindow)
	}
	if !start.After(now) {
		return errors.Join(ErrInvalidOccurrenceDate, ErrOccurrenceInPast)
	}

	return nil
}

func weekdayToSlotValue(weekday time.Weekday) int {
	if weekday == time.Sunday {
		return 7
	}
	return int(weekday)
}
