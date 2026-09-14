package service

import "testing"

func TestSolveAutoAssignmentsPreservesQualificationAndAvoidsOverlap(t *testing.T) {
	slots := []AutoAssignSlotPosition{
		{SlotID: 1, Weekday: 1, StartTime: "09:00", EndTime: "10:00", PositionID: 1, RequiredHeadcount: 1},
		{SlotID: 2, Weekday: 1, StartTime: "09:30", EndTime: "10:30", PositionID: 1, RequiredHeadcount: 1},
		{SlotID: 3, Weekday: 1, StartTime: "11:00", EndTime: "12:00", PositionID: 2, RequiredHeadcount: 1},
	}
	assignments, err := SolveAutoAssignments(slots, []AutoAssignCandidate{
		{UserID: "019535d9-3df7-79fb-b466-fa907fa17f9e", SlotID: 1, Weekday: 1, PositionID: 1},
		{UserID: "019535d9-3df7-79fb-b466-fa907fa17f9e", SlotID: 2, Weekday: 1, PositionID: 1},
		{UserID: "019535d9-3df7-79fb-b466-fa907fa17f90", SlotID: 2, Weekday: 1, PositionID: 1},
		{UserID: "019535d9-3df7-79fb-b466-fa907fa17f90", SlotID: 3, Weekday: 1, PositionID: 2},
		// This candidate is not qualified for any requested seat and must be ignored.
		{UserID: "019535d9-3df7-79fb-b466-fa907fa17f90", SlotID: 3, Weekday: 1, PositionID: 99},
	})
	if err != nil {
		t.Fatalf("SolveAutoAssignments() error = %v", err)
	}
	if len(assignments) != 3 {
		t.Fatalf("assignments = %#v, want all three non-overlapping seats covered", assignments)
	}

	byUser := make(map[string][]AutoAssignment)
	for _, assignment := range assignments {
		byUser[assignment.UserID] = append(byUser[assignment.UserID], assignment)
		if assignment.PositionID != 1 && assignment.PositionID != 2 {
			t.Fatalf("assignment used unqualified position: %#v", assignment)
		}
	}
	if len(byUser["019535d9-3df7-79fb-b466-fa907fa17f9e"]) > 1 {
		t.Fatalf("overlapping user received multiple assignments: %#v", assignments)
	}
}

func TestSolveAutoAssignmentsRejectsMalformedSlotDefinitions(t *testing.T) {
	for _, slots := range [][]AutoAssignSlotPosition{
		{{SlotID: 0, Weekday: 1, StartTime: "09:00", EndTime: "10:00", PositionID: 1, RequiredHeadcount: 1}},
		{{SlotID: 1, Weekday: 1, StartTime: "not-a-time", EndTime: "10:00", PositionID: 1, RequiredHeadcount: 1}},
		{{SlotID: 1, Weekday: 1, StartTime: "10:00", EndTime: "09:00", PositionID: 1, RequiredHeadcount: 1}},
		{{SlotID: 1, Weekday: 8, StartTime: "09:00", EndTime: "10:00", PositionID: 1, RequiredHeadcount: 1}},
		{{SlotID: 1, Weekday: 1, StartTime: "09:00", EndTime: "10:00", PositionID: 1, RequiredHeadcount: 0}},
	} {
		if _, err := SolveAutoAssignments(slots, []AutoAssignCandidate{{
			UserID:     "019535d9-3df7-79fb-b466-fa907fa17f9e",
			SlotID:     1,
			Weekday:    1,
			PositionID: 1,
		}}); err == nil {
			t.Fatalf("SolveAutoAssignments(%#v) returned nil error", slots)
		}
	}
}

func TestSolveAutoAssignmentsReturnsEmptyForNoCandidates(t *testing.T) {
	assignments, err := SolveAutoAssignments([]AutoAssignSlotPosition{{
		SlotID: 1, Weekday: 1, StartTime: "09:00", EndTime: "10:00", PositionID: 1, RequiredHeadcount: 1,
	}}, nil)
	if err != nil || len(assignments) != 0 {
		t.Fatalf("empty candidates = %#v, %v; want empty success", assignments, err)
	}

	invalidCandidates, err := SolveAutoAssignments([]AutoAssignSlotPosition{{
		SlotID: 1, Weekday: 1, StartTime: "09:00", EndTime: "10:00", PositionID: 1, RequiredHeadcount: 1,
	}}, []AutoAssignCandidate{{UserID: "not-a-uuid", SlotID: 1, Weekday: 1, PositionID: 1}})
	if err != nil || len(invalidCandidates) != 0 {
		// Invalid candidates are deliberately filtered rather than making the whole
		// batch fail.
		t.Fatalf("invalid candidates = %#v, %v; want empty success", invalidCandidates, err)
	}
}
