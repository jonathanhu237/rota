package scenarios

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	seedinternal "github.com/jonathanhu237/rota/backend/cmd/seed/internal"
	"github.com/jonathanhu237/rota/backend/internal/model"
)

const SeedPassword = "pa55word"

type Options struct {
	BootstrapEmail    string
	BootstrapName     string
	BootstrapPassword string
	Now               time.Time
}

type slotPosition struct {
	SlotID     int64
	PositionID int64
}

type assignmentRecord struct {
	ID         int64
	UserID     int64
	SlotID     int64
	PositionID int64
}

type positionHeadcount struct {
	PositionIndex     int
	RequiredHeadcount int
}

type slotDefinition struct {
	Weekday   int
	StartTime string
	EndTime   string
	Positions []positionHeadcount
}

func IsValid(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "basic", "full", "stress":
		return true
	default:
		return false
	}
}

func Run(ctx context.Context, tx *sql.Tx, name string, opts Options) error {
	opts = normalizeOptions(opts)
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "basic":
		return RunBasic(ctx, tx, opts)
	case "full":
		return RunFull(ctx, tx, opts)
	case "stress":
		return RunStress(ctx, tx, opts)
	default:
		return fmt.Errorf("unknown scenario %q", name)
	}
}

func normalizeOptions(opts Options) Options {
	if opts.BootstrapEmail == "" {
		opts.BootstrapEmail = "admin@example.com"
	}
	if opts.BootstrapName == "" {
		opts.BootstrapName = "Administrator"
	}
	if opts.BootstrapPassword == "" {
		opts.BootstrapPassword = SeedPassword
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	return opts
}

func insertUsers(ctx context.Context, tx *sql.Tx, opts Options, employeeCount int) (int64, []int64, error) {
	adminID, err := seedinternal.InsertUser(
		ctx,
		tx,
		opts.BootstrapEmail,
		opts.BootstrapName,
		opts.BootstrapPassword,
		true,
	)
	if err != nil {
		return 0, nil, fmt.Errorf("insert bootstrap admin: %w", err)
	}

	employees := make([]int64, 0, employeeCount)
	for i := 1; i <= employeeCount; i++ {
		id, err := seedinternal.InsertUser(
			ctx,
			tx,
			fmt.Sprintf("employee%d@example.com", i),
			fmt.Sprintf("Employee %d", i),
			SeedPassword,
			false,
		)
		if err != nil {
			return 0, nil, fmt.Errorf("insert employee %d: %w", i, err)
		}
		employees = append(employees, id)
	}

	return adminID, employees, nil
}

func insertPositions(ctx context.Context, tx *sql.Tx, count int) ([]int64, error) {
	positions := make([]int64, 0, count)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("Position %s", positionLabel(i))
		var id int64
		err := tx.QueryRowContext(
			ctx,
			`
				INSERT INTO positions (name, description)
				VALUES ($1, $2)
				RETURNING id;
			`,
			name,
			name+" seeded for local development",
		).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("insert %s: %w", name, err)
		}
		positions = append(positions, id)
	}
	return positions, nil
}

func positionLabel(index int) string {
	if index >= 0 && index < 26 {
		return string(rune('A' + index))
	}
	return fmt.Sprintf("%02d", index+1)
}

func insertTemplate(ctx context.Context, tx *sql.Tx, name string, locked bool, now time.Time) (int64, error) {
	var id int64
	err := tx.QueryRowContext(
		ctx,
		`
			INSERT INTO templates (name, description, is_locked, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $4)
			RETURNING id;
		`,
		name,
		name+" seeded template",
		locked,
		now,
	).Scan(&id)
	return id, err
}

func insertSlots(
	ctx context.Context,
	tx *sql.Tx,
	templateID int64,
	positionIDs []int64,
	definitions []slotDefinition,
	now time.Time,
) ([]slotPosition, error) {
	entries := make([]slotPosition, 0)
	for _, definition := range definitions {
		var slotID int64
		err := tx.QueryRowContext(
			ctx,
			`
				INSERT INTO template_slots (template_id, weekday, start_time, end_time, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $5)
				RETURNING id;
			`,
			templateID,
			definition.Weekday,
			definition.StartTime,
			definition.EndTime,
			now,
		).Scan(&slotID)
		if err != nil {
			return nil, fmt.Errorf("insert slot weekday=%d %s-%s: %w", definition.Weekday, definition.StartTime, definition.EndTime, err)
		}

		for _, position := range definition.Positions {
			if position.PositionIndex < 0 || position.PositionIndex >= len(positionIDs) {
				return nil, fmt.Errorf("position index %d out of range", position.PositionIndex)
			}
			headcount := position.RequiredHeadcount
			if headcount == 0 {
				headcount = 1
			}
			positionID := positionIDs[position.PositionIndex]
			if _, err := tx.ExecContext(
				ctx,
				`
					INSERT INTO template_slot_positions (slot_id, position_id, required_headcount, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $4);
				`,
				slotID,
				positionID,
				headcount,
				now,
			); err != nil {
				return nil, fmt.Errorf("insert slot-position slot=%d position=%d: %w", slotID, positionID, err)
			}
			entries = append(entries, slotPosition{SlotID: slotID, PositionID: positionID})
		}
	}
	return entries, nil
}

func qualifyRoundRobin(
	ctx context.Context,
	tx *sql.Tx,
	userIDs []int64,
	positionIDs []int64,
	countForUser func(index int) int,
) (map[int64]map[int64]bool, error) {
	qualified := make(map[int64]map[int64]bool, len(userIDs))
	for userIndex, userID := range userIDs {
		count := countForUser(userIndex)
		if count > len(positionIDs) {
			count = len(positionIDs)
		}
		qualified[userID] = make(map[int64]bool, count)
		for offset := 0; offset < count; offset++ {
			positionID := positionIDs[(userIndex+offset)%len(positionIDs)]
			if qualified[userID][positionID] {
				continue
			}
			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO user_positions (user_id, position_id) VALUES ($1, $2);`,
				userID,
				positionID,
			); err != nil {
				return nil, fmt.Errorf("qualify user=%d position=%d: %w", userID, positionID, err)
			}
			qualified[userID][positionID] = true
		}
	}
	return qualified, nil
}

func insertPublication(
	ctx context.Context,
	tx *sql.Tx,
	templateID int64,
	name string,
	state model.PublicationState,
	submissionStartAt, submissionEndAt, plannedActiveFrom, plannedActiveUntil time.Time,
	activatedAt *time.Time,
	now time.Time,
) (int64, error) {
	var id int64
	err := tx.QueryRowContext(
		ctx,
		`
			INSERT INTO publications (
				template_id,
				name,
				description,
				state,
				submission_start_at,
				submission_end_at,
				planned_active_from,
				planned_active_until,
				activated_at,
				created_at,
				updated_at
			)
			VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8, $9, $9)
			RETURNING id;
		`,
		templateID,
		name,
		state,
		submissionStartAt,
		submissionEndAt,
		plannedActiveFrom,
		plannedActiveUntil,
		activatedAt,
		now,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert publication %q: %w", name, err)
	}
	return id, nil
}

func insertAvailabilitySubmissions(
	ctx context.Context,
	tx *sql.Tx,
	publicationID int64,
	userIDs []int64,
	entries []slotPosition,
	qualified map[int64]map[int64]bool,
	modulus, threshold int64,
	now time.Time,
) error {
	for _, userID := range userIDs {
		for _, entry := range entries {
			if !qualified[userID][entry.PositionID] {
				continue
			}
			if (userID+entry.SlotID+entry.PositionID)%modulus >= threshold {
				continue
			}
			if _, err := tx.ExecContext(
				ctx,
				`
					INSERT INTO availability_submissions (
						publication_id,
						user_id,
						slot_id,
						position_id,
						created_at
					)
					VALUES ($1, $2, $3, $4, $5);
				`,
				publicationID,
				userID,
				entry.SlotID,
				entry.PositionID,
				now,
			); err != nil {
				return fmt.Errorf("insert submission publication=%d user=%d slot=%d position=%d: %w", publicationID, userID, entry.SlotID, entry.PositionID, err)
			}
		}
	}
	return nil
}

func insertAssignments(
	ctx context.Context,
	tx *sql.Tx,
	publicationID int64,
	userIDs []int64,
	entries []slotPosition,
	now time.Time,
) ([]assignmentRecord, error) {
	assignments := make([]assignmentRecord, 0, len(entries))
	slotOrdinal := make(map[int64]int)
	slotBase := make(map[int64]int)
	nextSlotBase := 0

	for _, entry := range entries {
		base, ok := slotBase[entry.SlotID]
		if !ok {
			base = nextSlotBase
			slotBase[entry.SlotID] = base
			nextSlotBase++
		}
		ordinal := slotOrdinal[entry.SlotID]
		slotOrdinal[entry.SlotID] = ordinal + 1
		userID := userIDs[(base+ordinal)%len(userIDs)]

		var id int64
		err := tx.QueryRowContext(
			ctx,
			`
				INSERT INTO assignments (
					publication_id,
					user_id,
					slot_id,
					position_id,
					created_at
				)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id;
			`,
			publicationID,
			userID,
			entry.SlotID,
			entry.PositionID,
			now,
		).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("insert assignment publication=%d user=%d slot=%d position=%d: %w", publicationID, userID, entry.SlotID, entry.PositionID, err)
		}
		assignments = append(assignments, assignmentRecord{
			ID:         id,
			UserID:     userID,
			SlotID:     entry.SlotID,
			PositionID: entry.PositionID,
		})
	}
	return assignments, nil
}
