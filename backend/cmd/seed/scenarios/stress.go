package scenarios

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

func RunStress(ctx context.Context, tx *sql.Tx, opts Options) error {
	_, employees, err := insertUsers(ctx, tx, opts, 50)
	if err != nil {
		return err
	}

	positions, err := insertPositions(ctx, tx, 8)
	if err != nil {
		return err
	}
	qualified, err := qualifyRoundRobin(ctx, tx, employees, positions, func(index int) int {
		if index%3 == 0 {
			return 3
		}
		return 2
	})
	if err != nil {
		return err
	}

	templateID, err := insertTemplate(ctx, tx, "Stress Rota", true, opts.Now)
	if err != nil {
		return err
	}
	entries, err := insertSlots(ctx, tx, templateID, positions, stressSlotDefinitions(), opts.Now)
	if err != nil {
		return err
	}

	endedAtOne := opts.Now.Add(-21 * 24 * time.Hour)
	endedAtTwo := opts.Now.Add(-7 * 24 * time.Hour)
	endedAtSubmissionFixture := opts.Now.Add(-24 * time.Hour)
	if _, err := insertPublication(
		ctx,
		tx,
		templateID,
		"Historical Rota 1",
		model.PublicationStateEnded,
		opts.Now.Add(-42*24*time.Hour),
		opts.Now.Add(-35*24*time.Hour),
		opts.Now.Add(-28*24*time.Hour),
		&endedAtOne,
		&endedAtOne,
		opts.Now.Add(-42*24*time.Hour),
	); err != nil {
		return err
	}
	if _, err := insertPublication(
		ctx,
		tx,
		templateID,
		"Historical Rota 2",
		model.PublicationStateEnded,
		opts.Now.Add(-28*24*time.Hour),
		opts.Now.Add(-21*24*time.Hour),
		opts.Now.Add(-14*24*time.Hour),
		&endedAtTwo,
		&endedAtTwo,
		opts.Now.Add(-28*24*time.Hour),
	); err != nil {
		return err
	}

	// The schema enforces publications_single_non_ended_idx, so stress keeps
	// only the current active publication non-ENDED. This ended fixture still
	// carries next-week submissions for dense read-path data without violating D2.
	submissionFixtureID, err := insertPublication(
		ctx,
		tx,
		templateID,
		"Next Week Submission Fixture",
		model.PublicationStateEnded,
		opts.Now.Add(-14*24*time.Hour),
		opts.Now.Add(-7*24*time.Hour),
		opts.Now.Add(7*24*time.Hour),
		&endedAtSubmissionFixture,
		&endedAtSubmissionFixture,
		opts.Now.Add(-14*24*time.Hour),
	)
	if err != nil {
		return err
	}
	if err := insertAvailabilitySubmissions(ctx, tx, submissionFixtureID, employees, entries, qualified, 5, 2, opts.Now); err != nil {
		return err
	}

	activeAt := opts.Now.Add(-12 * time.Hour)
	activeID, err := insertPublication(
		ctx,
		tx,
		templateID,
		"Current Active Rota",
		model.PublicationStateActive,
		opts.Now.Add(-14*24*time.Hour),
		opts.Now.Add(-7*24*time.Hour),
		opts.Now.Add(-24*time.Hour),
		&activeAt,
		nil,
		opts.Now.Add(-14*24*time.Hour),
	)
	if err != nil {
		return err
	}
	assignments, err := insertAssignments(ctx, tx, activeID, employees, entries, opts.Now)
	if err != nil {
		return err
	}
	if len(assignments) < 5 {
		return fmt.Errorf("stress scenario needs at least 5 assignments, got %d", len(assignments))
	}
	return insertStressShiftChanges(ctx, tx, activeID, assignments, employees, opts.Now)
}

func stressSlotDefinitions() []slotDefinition {
	definitions := make([]slotDefinition, 0, 27)
	for weekday := 1; weekday <= 5; weekday++ {
		definitions = append(definitions,
			slotDefinition{Weekday: weekday, StartTime: "08:00", EndTime: "10:00", Positions: threePositions(0, 1, 2)},
			slotDefinition{Weekday: weekday, StartTime: "10:30", EndTime: "12:30", Positions: threePositions(1, 2, 3)},
			slotDefinition{Weekday: weekday, StartTime: "13:30", EndTime: "15:30", Positions: threePositions(2, 3, 4)},
			slotDefinition{Weekday: weekday, StartTime: "16:00", EndTime: "18:00", Positions: threePositions(3, 4, 5)},
		)
	}
	for weekday := 1; weekday <= 7; weekday++ {
		definitions = append(definitions, slotDefinition{
			Weekday:   weekday,
			StartTime: "19:00",
			EndTime:   "21:00",
			Positions: twoPositions(6, 7),
		})
	}
	return definitions
}

func threePositions(first, second, third int) []positionHeadcount {
	return []positionHeadcount{
		{PositionIndex: first, RequiredHeadcount: 1},
		{PositionIndex: second, RequiredHeadcount: 1},
		{PositionIndex: third, RequiredHeadcount: 1},
	}
}

func insertStressShiftChanges(
	ctx context.Context,
	tx *sql.Tx,
	publicationID int64,
	assignments []assignmentRecord,
	users []int64,
	now time.Time,
) error {
	swapLeft := assignments[0]
	swapRight := assignments[1]
	giveDirect := assignments[2]
	giveDirectReceiver := users[len(users)-1]
	givePool := assignments[3]

	requests := []struct {
		changeType              model.ShiftChangeType
		requesterUserID         int64
		requesterAssignmentID   int64
		counterpartUserID       any
		counterpartAssignmentID any
	}{
		{
			changeType:              model.ShiftChangeTypeSwap,
			requesterUserID:         swapLeft.UserID,
			requesterAssignmentID:   swapLeft.ID,
			counterpartUserID:       swapRight.UserID,
			counterpartAssignmentID: swapRight.ID,
		},
		{
			changeType:              model.ShiftChangeTypeGiveDirect,
			requesterUserID:         giveDirect.UserID,
			requesterAssignmentID:   giveDirect.ID,
			counterpartUserID:       giveDirectReceiver,
			counterpartAssignmentID: nil,
		},
		{
			changeType:              model.ShiftChangeTypeGivePool,
			requesterUserID:         givePool.UserID,
			requesterAssignmentID:   givePool.ID,
			counterpartUserID:       nil,
			counterpartAssignmentID: nil,
		},
	}

	for _, request := range requests {
		if _, err := tx.ExecContext(
			ctx,
			`
				INSERT INTO shift_change_requests (
					publication_id,
					type,
					requester_user_id,
					requester_assignment_id,
					counterpart_user_id,
					counterpart_assignment_id,
					state,
					created_at,
					expires_at
				)
				VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, $8);
			`,
			publicationID,
			request.changeType,
			request.requesterUserID,
			request.requesterAssignmentID,
			request.counterpartUserID,
			request.counterpartAssignmentID,
			now,
			now.Add(48*time.Hour),
		); err != nil {
			return fmt.Errorf("insert %s shift-change request: %w", request.changeType, err)
		}
	}
	return nil
}
