package service

import "testing"

func TestAutoAssignSolver(t *testing.T) {
	t.Run("simple 1 to 1 equalization", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignSlotPosition{
			{SlotID: 21, PositionID: 101, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{SlotID: 22, PositionID: 102, Weekday: 2, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
		}, []AutoAssignCandidate{
			{UserID: 1, SlotID: 21, PositionID: 101},
			{UserID: 1, SlotID: 22, PositionID: 102},
			{UserID: 2, SlotID: 21, PositionID: 101},
			{UserID: 2, SlotID: 22, PositionID: 102},
		})
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		if len(assignments) != 2 {
			t.Fatalf("expected 2 assignments, got %d", len(assignments))
		}

		counts := countAssignmentsByUser(assignments)
		if counts[1] != 1 || counts[2] != 1 {
			t.Fatalf("expected one slot-position per user, got %+v", counts)
		}
	})

	t.Run("equalization across three employees", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignSlotPosition{
			{SlotID: 21, PositionID: 101, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{SlotID: 22, PositionID: 102, Weekday: 2, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{SlotID: 23, PositionID: 103, Weekday: 3, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
		}, []AutoAssignCandidate{
			{UserID: 1, SlotID: 21, PositionID: 101},
			{UserID: 1, SlotID: 22, PositionID: 102},
			{UserID: 1, SlotID: 23, PositionID: 103},
			{UserID: 2, SlotID: 21, PositionID: 101},
			{UserID: 2, SlotID: 22, PositionID: 102},
			{UserID: 2, SlotID: 23, PositionID: 103},
			{UserID: 3, SlotID: 21, PositionID: 101},
			{UserID: 3, SlotID: 22, PositionID: 102},
			{UserID: 3, SlotID: 23, PositionID: 103},
		})
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		counts := countAssignmentsByUser(assignments)
		if counts[1] != 1 || counts[2] != 1 || counts[3] != 1 {
			t.Fatalf("expected one slot-position per user, got %+v", counts)
		}
	})

	t.Run("uses scarce candidates to maximize coverage", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignSlotPosition{
			{SlotID: 21, PositionID: 101, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{SlotID: 22, PositionID: 102, Weekday: 2, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
		}, []AutoAssignCandidate{
			{UserID: 1, SlotID: 21, PositionID: 101},
			{UserID: 1, SlotID: 22, PositionID: 102},
			{UserID: 2, SlotID: 21, PositionID: 101},
		})
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		slotID, positionID := assignedSlotPositionForUser(assignments, 2)
		if slotID != 21 || positionID != 101 {
			t.Fatalf("expected scarce user 2 to get slot-position 21/101, got %+v", assignments)
		}
		slotID, positionID = assignedSlotPositionForUser(assignments, 1)
		if slotID != 22 || positionID != 102 {
			t.Fatalf("expected user 1 to get slot-position 22/102, got %+v", assignments)
		}
	})

	t.Run("fills headcount up to capacity", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignSlotPosition{
			{SlotID: 21, PositionID: 101, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 3},
		}, []AutoAssignCandidate{
			{UserID: 1, SlotID: 21, PositionID: 101},
			{UserID: 2, SlotID: 21, PositionID: 101},
			{UserID: 3, SlotID: 21, PositionID: 101},
			{UserID: 4, SlotID: 21, PositionID: 101},
			{UserID: 5, SlotID: 21, PositionID: 101},
		})
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		if got := countAssignmentsForSlotPosition(assignments, 21, 101); got != 3 {
			t.Fatalf("expected 3 assignments, got %d", got)
		}
	})

	t.Run("prevents overlap conflicts for one employee", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignSlotPosition{
			{SlotID: 21, PositionID: 101, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{SlotID: 22, PositionID: 102, Weekday: 1, StartTime: "11:00", EndTime: "14:00", RequiredHeadcount: 1},
		}, []AutoAssignCandidate{
			{UserID: 1, SlotID: 21, PositionID: 101},
			{UserID: 1, SlotID: 22, PositionID: 102},
		})
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		if got := countAssignmentsByUser(assignments)[1]; got != 1 {
			t.Fatalf("expected one assignment due to overlap, got %d", got)
		}
	})

	t.Run("prevents assigning two positions in the same slot to one user", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignSlotPosition{
			{SlotID: 21, PositionID: 101, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{SlotID: 21, PositionID: 102, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
		}, []AutoAssignCandidate{
			{UserID: 1, SlotID: 21, PositionID: 101},
			{UserID: 1, SlotID: 21, PositionID: 102},
		})
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		if got := countAssignmentsByUser(assignments)[1]; got != 1 {
			t.Fatalf("expected one assignment in the slot, got %d", got)
		}
	})

	t.Run("returns empty assignments when there are no candidates", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignSlotPosition{
			{SlotID: 21, PositionID: 101, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
		}, nil)
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}
		if len(assignments) != 0 {
			t.Fatalf("expected no assignments, got %+v", assignments)
		}
	})

	t.Run("under supply is best effort", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignSlotPosition{
			{SlotID: 21, PositionID: 101, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 3},
		}, []AutoAssignCandidate{
			{UserID: 1, SlotID: 21, PositionID: 101},
			{UserID: 2, SlotID: 21, PositionID: 101},
		})
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		if got := countAssignmentsForSlotPosition(assignments, 21, 101); got != 2 {
			t.Fatalf("expected 2 assignments, got %d", got)
		}
	})

	t.Run("fairness prefers balance across varying availability", func(t *testing.T) {
		t.Parallel()

		slotPositions := []AutoAssignSlotPosition{
			{SlotID: 21, PositionID: 101, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{SlotID: 22, PositionID: 102, Weekday: 2, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{SlotID: 23, PositionID: 103, Weekday: 3, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{SlotID: 24, PositionID: 104, Weekday: 4, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{SlotID: 25, PositionID: 105, Weekday: 5, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{SlotID: 26, PositionID: 106, Weekday: 6, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
		}
		candidates := []AutoAssignCandidate{
			{UserID: 1, SlotID: 21, PositionID: 101},
			{UserID: 1, SlotID: 22, PositionID: 102},
			{UserID: 1, SlotID: 23, PositionID: 103},
			{UserID: 1, SlotID: 24, PositionID: 104},
			{UserID: 1, SlotID: 25, PositionID: 105},
			{UserID: 1, SlotID: 26, PositionID: 106},
			{UserID: 2, SlotID: 21, PositionID: 101},
			{UserID: 2, SlotID: 22, PositionID: 102},
			{UserID: 2, SlotID: 23, PositionID: 103},
			{UserID: 2, SlotID: 24, PositionID: 104},
			{UserID: 3, SlotID: 21, PositionID: 101},
			{UserID: 3, SlotID: 22, PositionID: 102},
		}

		assignments, err := SolveAutoAssignments(slotPositions, candidates)
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		counts := countAssignmentsByUser(assignments)
		if counts[1] != 2 || counts[2] != 2 || counts[3] != 2 {
			t.Fatalf("expected balanced [2,2,2], got %+v", counts)
		}
	})
}

func countAssignmentsByUser(assignments []AutoAssignment) map[int64]int {
	result := make(map[int64]int)
	for _, assignment := range assignments {
		result[assignment.UserID]++
	}

	return result
}

func countAssignmentsForSlotPosition(assignments []AutoAssignment, slotID, positionID int64) int {
	count := 0
	for _, assignment := range assignments {
		if assignment.SlotID == slotID && assignment.PositionID == positionID {
			count++
		}
	}

	return count
}

func assignedSlotPositionForUser(assignments []AutoAssignment, userID int64) (int64, int64) {
	for _, assignment := range assignments {
		if assignment.UserID == userID {
			return assignment.SlotID, assignment.PositionID
		}
	}

	return 0, 0
}
