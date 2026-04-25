package scenarios

import (
	"context"
	"database/sql"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

func RunFull(ctx context.Context, tx *sql.Tx, opts Options) error {
	_, employees, err := insertUsers(ctx, tx, opts, 8)
	if err != nil {
		return err
	}

	positions, err := insertPositions(ctx, tx, 4)
	if err != nil {
		return err
	}
	qualified, err := qualifyRoundRobin(ctx, tx, employees, positions, func(index int) int { return 2 })
	if err != nil {
		return err
	}

	templateID, err := insertTemplate(ctx, tx, "Default Rota", true, opts.Now)
	if err != nil {
		return err
	}
	entries, err := insertSlots(ctx, tx, templateID, positions, fullSlotDefinitions(), opts.Now)
	if err != nil {
		return err
	}

	publicationID, err := insertPublication(
		ctx,
		tx,
		templateID,
		"Next Week Rota",
		model.PublicationStateDraft,
		opts.Now.Add(-14*24*time.Hour),
		opts.Now.Add(-7*24*time.Hour),
		opts.Now.Add(7*24*time.Hour),
		opts.Now.Add(9*7*24*time.Hour),
		nil,
		opts.Now,
	)
	if err != nil {
		return err
	}

	return insertAvailabilitySubmissions(ctx, tx, publicationID, employees, entries, qualified, 5, 3, opts.Now)
}

func fullSlotDefinitions() []slotDefinition {
	return []slotDefinition{
		{Weekday: 1, StartTime: "09:00", EndTime: "12:00", Positions: twoPositions(0, 1)},
		{Weekday: 1, StartTime: "14:00", EndTime: "17:00", Positions: twoPositions(0, 1)},
		{Weekday: 2, StartTime: "09:00", EndTime: "12:00", Positions: twoPositions(0, 1)},
		{Weekday: 2, StartTime: "14:00", EndTime: "17:00", Positions: twoPositions(0, 1)},
		{Weekday: 3, StartTime: "09:00", EndTime: "12:00", Positions: twoPositions(0, 1)},
		{Weekday: 3, StartTime: "19:00", EndTime: "21:00", Positions: twoPositions(2, 3)},
		{Weekday: 4, StartTime: "09:00", EndTime: "12:00", Positions: twoPositions(0, 2)},
		{Weekday: 4, StartTime: "14:00", EndTime: "17:00", Positions: twoPositions(1, 3)},
		{Weekday: 5, StartTime: "09:00", EndTime: "12:00", Positions: twoPositions(0, 1)},
		{Weekday: 5, StartTime: "14:00", EndTime: "17:00", Positions: twoPositions(2, 3)},
	}
}

func twoPositions(first, second int) []positionHeadcount {
	return []positionHeadcount{
		{PositionIndex: first, RequiredHeadcount: 1},
		{PositionIndex: second, RequiredHeadcount: 1},
	}
}
