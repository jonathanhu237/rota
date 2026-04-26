package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

type InsertAssignmentOverrideParams struct {
	AssignmentID   int64
	OccurrenceDate time.Time
	UserID         int64
	CreatedAt      time.Time
}

func (r *PublicationRepository) InsertAssignmentOverride(
	ctx context.Context,
	params InsertAssignmentOverrideParams,
) (*model.AssignmentOverride, error) {
	const query = `
		INSERT INTO assignment_overrides (
			assignment_id,
			occurrence_date,
			user_id,
			created_at
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, assignment_id, occurrence_date, user_id, created_at;
	`

	return scanAssignmentOverride(r.db.QueryRowContext(
		ctx,
		query,
		params.AssignmentID,
		model.NormalizeOccurrenceDate(params.OccurrenceDate),
		params.UserID,
		params.CreatedAt,
	))
}

func (r *PublicationRepository) DeleteAssignmentOverridesByAssignment(
	ctx context.Context,
	assignmentID int64,
) (int64, error) {
	const query = `
		DELETE FROM assignment_overrides
		WHERE assignment_id = $1;
	`

	result, err := r.db.ExecContext(ctx, query, assignmentID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *PublicationRepository) CountAssignmentOverridesByAssignment(
	ctx context.Context,
	assignmentID int64,
) (int64, error) {
	const query = `
		SELECT COUNT(*)
		FROM assignment_overrides
		WHERE assignment_id = $1;
	`

	var count int64
	if err := r.db.QueryRowContext(ctx, query, assignmentID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *PublicationRepository) ListPublicationAssignmentsForWeek(
	ctx context.Context,
	publicationID int64,
	weekStart time.Time,
) ([]*model.AssignmentParticipant, error) {
	const query = `
		SELECT
			a.id,
			a.slot_id,
			a.weekday,
			a.position_id,
			u.id,
			u.name,
			u.email,
			a.created_at
		FROM assignments a
		LEFT JOIN assignment_overrides ao
			ON ao.assignment_id = a.id
			AND ao.occurrence_date = ($2::date + ((a.weekday - 1) * INTERVAL '1 day'))::date
		INNER JOIN users u ON u.id = COALESCE(ao.user_id, a.user_id)
		WHERE a.publication_id = $1
		ORDER BY a.weekday ASC, a.slot_id ASC, a.position_id ASC, u.id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, publicationID, model.NormalizeOccurrenceDate(weekStart))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assignments := make([]*model.AssignmentParticipant, 0)
	for rows.Next() {
		assignment := &model.AssignmentParticipant{}
		if err := rows.Scan(
			&assignment.AssignmentID,
			&assignment.SlotID,
			&assignment.Weekday,
			&assignment.PositionID,
			&assignment.UserID,
			&assignment.Name,
			&assignment.Email,
			&assignment.CreatedAt,
		); err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return assignments, nil
}

func scanAssignmentOverride(row scanner) (*model.AssignmentOverride, error) {
	override := &model.AssignmentOverride{}
	if err := row.Scan(
		&override.ID,
		&override.AssignmentID,
		&override.OccurrenceDate,
		&override.UserID,
		&override.CreatedAt,
	); err != nil {
		return nil, err
	}
	override.OccurrenceDate = model.NormalizeOccurrenceDate(override.OccurrenceDate)
	return override, nil
}

func insertAssignmentOverrideTx(
	ctx context.Context,
	tx *sql.Tx,
	params InsertAssignmentOverrideParams,
) error {
	const query = `
		INSERT INTO assignment_overrides (
			assignment_id,
			occurrence_date,
			user_id,
			created_at
		)
		VALUES ($1, $2, $3, $4);
	`
	_, err := tx.ExecContext(
		ctx,
		query,
		params.AssignmentID,
		model.NormalizeOccurrenceDate(params.OccurrenceDate),
		params.UserID,
		params.CreatedAt,
	)
	return err
}

type AssignmentOverrideRepository struct {
	db *sql.DB
}

func NewAssignmentOverrideRepository(db *sql.DB) *AssignmentOverrideRepository {
	return &AssignmentOverrideRepository{db: db}
}

func (r *AssignmentOverrideRepository) Insert(
	ctx context.Context,
	params InsertAssignmentOverrideParams,
) (*model.AssignmentOverride, error) {
	return (&PublicationRepository{db: r.db}).InsertAssignmentOverride(ctx, params)
}

func (r *AssignmentOverrideRepository) DeleteByAssignment(
	ctx context.Context,
	assignmentID int64,
) (int64, error) {
	return (&PublicationRepository{db: r.db}).DeleteAssignmentOverridesByAssignment(ctx, assignmentID)
}

func (r *AssignmentOverrideRepository) ListForPublicationWeek(
	ctx context.Context,
	publicationID int64,
	weekStart time.Time,
) ([]*model.AssignmentParticipant, error) {
	return (&PublicationRepository{db: r.db}).ListPublicationAssignmentsForWeek(ctx, publicationID, weekStart)
}
