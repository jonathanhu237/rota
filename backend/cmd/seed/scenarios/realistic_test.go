package scenarios

import "testing"

func TestRealisticFixtureShape(t *testing.T) {
	if got := realisticAssignmentSeed; got != 20260428 {
		t.Fatalf("realisticAssignmentSeed = %d, want 20260428", got)
	}
	if got := len(realisticEmployees); got != 42 {
		t.Fatalf("len(realisticEmployees) = %d, want 42", got)
	}
	if got := len(realisticTimeSlots); got != 5 {
		t.Fatalf("len(realisticTimeSlots) = %d, want 5", got)
	}
	if got := len(realisticSlotWeekdays); got != len(realisticTimeSlots) {
		t.Fatalf("len(realisticSlotWeekdays) = %d, want %d", got, len(realisticTimeSlots))
	}

	archetypeCounts := map[archetype]int{}
	insertedSubmissions := 0
	domainFilteredDrops := 0
	scheduleFilteredDrops := 0
	for _, employee := range realisticEmployees {
		archetypeCounts[employee.Arch]++
		for timeIndex, weekdays := range employee.Weekdays {
			for _, weekday := range weekdays {
				if weekday < 1 || weekday > 7 {
					t.Fatalf("%s has invalid weekday %d", employee.Slug, weekday)
				}
				if !employee.Arch.matchesTimeSlot(timeIndex) {
					domainFilteredDrops++
					continue
				}
				if !realisticSlotRunsOnWeekday(timeIndex, weekday) {
					scheduleFilteredDrops++
					continue
				}
				insertedSubmissions++
			}
		}
	}

	wantArchetypeCounts := map[archetype]int{
		FrontSenior: 10,
		FrontJunior: 22,
		FieldSenior: 4,
		FieldJunior: 6,
	}
	for arch, want := range wantArchetypeCounts {
		if got := archetypeCounts[arch]; got != want {
			t.Fatalf("archetype count %d = %d, want %d", arch, got, want)
		}
	}
	if got := insertedSubmissions; got != realisticExpectedAvailabilitySubmits {
		t.Fatalf("inserted submissions = %d, want %d", got, realisticExpectedAvailabilitySubmits)
	}
	if got := domainFilteredDrops; got != realisticDomainFilteredSubmissionDrops {
		t.Fatalf("domain filtered drops = %d, want %d", got, realisticDomainFilteredSubmissionDrops)
	}
	if got := scheduleFilteredDrops; got != realisticScheduleFilteredSubmissionDrops {
		t.Fatalf("schedule filtered drops = %d, want %d", got, realisticScheduleFilteredSubmissionDrops)
	}
}

func TestRealisticSlotDefinitions(t *testing.T) {
	insertedWeekdays := 0
	for timeIndex, weekdays := range realisticSlotWeekdays {
		if timeIndex < 4 {
			assertIntSlice(t, weekdays, []int{1, 2, 3, 4, 5})
		} else {
			assertIntSlice(t, weekdays, []int{1, 2, 3, 4, 5, 6, 7})
		}
		insertedWeekdays += len(weekdays)

		positions := realisticSlotPositions(timeIndex)
		if len(positions) != 2 {
			t.Fatalf("slot %d position count = %d, want 2", timeIndex, len(positions))
		}
		if timeIndex < 4 {
			assertPositionHeadcount(t, positions[0], realisticFrontLeadPosition, 1)
			assertPositionHeadcount(t, positions[1], realisticFrontAssistantPosition, 2)
		} else {
			assertPositionHeadcount(t, positions[0], realisticFieldLeadPosition, 1)
			assertPositionHeadcount(t, positions[1], realisticFieldAssistantPosition, 1)
		}
	}
	if insertedWeekdays != realisticExpectedSlotWeekdays {
		t.Fatalf("slot weekday count = %d, want %d", insertedWeekdays, realisticExpectedSlotWeekdays)
	}
}

func realisticSlotRunsOnWeekday(timeIndex, weekday int) bool {
	for _, candidate := range realisticSlotWeekdays[timeIndex] {
		if candidate == weekday {
			return true
		}
	}
	return false
}

func assertIntSlice(t *testing.T, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len(%v) = %d, want %d", got, len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("slice %v differs from %v at index %d", got, want, i)
		}
	}
}

func assertPositionHeadcount(t *testing.T, got positionHeadcount, wantPositionIndex, wantHeadcount int) {
	t.Helper()
	if got.PositionIndex != wantPositionIndex || got.RequiredHeadcount != wantHeadcount {
		t.Fatalf(
			"positionHeadcount = {PositionIndex:%d RequiredHeadcount:%d}, want {PositionIndex:%d RequiredHeadcount:%d}",
			got.PositionIndex,
			got.RequiredHeadcount,
			wantPositionIndex,
			wantHeadcount,
		)
	}
}
