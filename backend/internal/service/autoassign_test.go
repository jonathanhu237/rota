package service

import (
	"testing"
)

func TestAutoAssignSolver(t *testing.T) {
	t.Run("simple 1 to 1 equalization", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignShift{
			{ID: 11, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{ID: 12, Weekday: 2, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
		}, []AutoAssignCandidate{
			{UserID: 1, TemplateShiftID: 11},
			{UserID: 1, TemplateShiftID: 12},
			{UserID: 2, TemplateShiftID: 11},
			{UserID: 2, TemplateShiftID: 12},
		})
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		if len(assignments) != 2 {
			t.Fatalf("expected 2 assignments, got %d", len(assignments))
		}

		counts := countAssignmentsByUser(assignments)
		if counts[1] != 1 || counts[2] != 1 {
			t.Fatalf("expected one shift per user, got %+v", counts)
		}
	})

	t.Run("equalization across three employees", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignShift{
			{ID: 11, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{ID: 12, Weekday: 2, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{ID: 13, Weekday: 3, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
		}, []AutoAssignCandidate{
			{UserID: 1, TemplateShiftID: 11},
			{UserID: 1, TemplateShiftID: 12},
			{UserID: 1, TemplateShiftID: 13},
			{UserID: 2, TemplateShiftID: 11},
			{UserID: 2, TemplateShiftID: 12},
			{UserID: 2, TemplateShiftID: 13},
			{UserID: 3, TemplateShiftID: 11},
			{UserID: 3, TemplateShiftID: 12},
			{UserID: 3, TemplateShiftID: 13},
		})
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		counts := countAssignmentsByUser(assignments)
		if counts[1] != 1 || counts[2] != 1 || counts[3] != 1 {
			t.Fatalf("expected one shift per user, got %+v", counts)
		}
	})

	t.Run("uses scarce candidates to maximize coverage", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignShift{
			{ID: 11, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{ID: 12, Weekday: 2, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
		}, []AutoAssignCandidate{
			{UserID: 1, TemplateShiftID: 11},
			{UserID: 1, TemplateShiftID: 12},
			{UserID: 2, TemplateShiftID: 11},
		})
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		if assignedShiftForUser(assignments, 2) != 11 {
			t.Fatalf("expected scarce user 2 to get shift 11, got %+v", assignments)
		}
		if assignedShiftForUser(assignments, 1) != 12 {
			t.Fatalf("expected user 1 to get shift 12, got %+v", assignments)
		}
	})

	t.Run("fills headcount up to capacity", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignShift{
			{ID: 11, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 3},
		}, []AutoAssignCandidate{
			{UserID: 1, TemplateShiftID: 11},
			{UserID: 2, TemplateShiftID: 11},
			{UserID: 3, TemplateShiftID: 11},
			{UserID: 4, TemplateShiftID: 11},
			{UserID: 5, TemplateShiftID: 11},
		})
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		if got := countAssignmentsForShift(assignments, 11); got != 3 {
			t.Fatalf("expected 3 assignments, got %d", got)
		}
	})

	t.Run("prevents overlap conflicts for one employee", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignShift{
			{ID: 11, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{ID: 12, Weekday: 1, StartTime: "11:00", EndTime: "14:00", RequiredHeadcount: 1},
		}, []AutoAssignCandidate{
			{UserID: 1, TemplateShiftID: 11},
			{UserID: 1, TemplateShiftID: 12},
		})
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		if got := countAssignmentsByUser(assignments)[1]; got != 1 {
			t.Fatalf("expected one assignment due to overlap, got %d", got)
		}
	})

	t.Run("returns empty assignments when there are no candidates", func(t *testing.T) {
		t.Parallel()

		assignments, err := SolveAutoAssignments([]AutoAssignShift{
			{ID: 11, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
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

		assignments, err := SolveAutoAssignments([]AutoAssignShift{
			{ID: 11, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 3},
		}, []AutoAssignCandidate{
			{UserID: 1, TemplateShiftID: 11},
			{UserID: 2, TemplateShiftID: 11},
		})
		if err != nil {
			t.Fatalf("SolveAutoAssignments returned error: %v", err)
		}

		if got := countAssignmentsForShift(assignments, 11); got != 2 {
			t.Fatalf("expected 2 assignments, got %d", got)
		}
	})

	t.Run("fairness prefers balance across varying availability", func(t *testing.T) {
		t.Parallel()

		shifts := []AutoAssignShift{
			{ID: 11, Weekday: 1, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{ID: 12, Weekday: 2, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{ID: 13, Weekday: 3, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{ID: 14, Weekday: 4, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{ID: 15, Weekday: 5, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
			{ID: 16, Weekday: 6, StartTime: "09:00", EndTime: "12:00", RequiredHeadcount: 1},
		}
		candidates := []AutoAssignCandidate{
			{UserID: 1, TemplateShiftID: 11},
			{UserID: 1, TemplateShiftID: 12},
			{UserID: 1, TemplateShiftID: 13},
			{UserID: 1, TemplateShiftID: 14},
			{UserID: 1, TemplateShiftID: 15},
			{UserID: 1, TemplateShiftID: 16},
			{UserID: 2, TemplateShiftID: 11},
			{UserID: 2, TemplateShiftID: 12},
			{UserID: 2, TemplateShiftID: 13},
			{UserID: 2, TemplateShiftID: 14},
			{UserID: 3, TemplateShiftID: 11},
			{UserID: 3, TemplateShiftID: 12},
		}

		assignments, err := SolveAutoAssignments(shifts, candidates)
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

func countAssignmentsForShift(assignments []AutoAssignment, shiftID int64) int {
	count := 0
	for _, assignment := range assignments {
		if assignment.TemplateShiftID == shiftID {
			count++
		}
	}

	return count
}

func assignedShiftForUser(assignments []AutoAssignment, userID int64) int64 {
	for _, assignment := range assignments {
		if assignment.UserID == userID {
			return assignment.TemplateShiftID
		}
	}

	return 0
}
