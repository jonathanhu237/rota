package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/lib/pq"
)

var (
	ErrInvalidPublicationWindow = model.ErrInvalidPublicationWindow
	ErrPublicationAlreadyExists = model.ErrPublicationAlreadyExists
	ErrPublicationNotFound      = model.ErrPublicationNotFound
)

const (
	publicationsSingleNonEndedIndex = "publications_single_non_ended_idx"
	publicationsWindowCheckName     = "publications_submission_window_check"
)

type ListPublicationsParams struct {
	Offset int
	Limit  int
}

type CreatePublicationParams struct {
	TemplateID        int64
	Name              string
	State             model.PublicationState
	SubmissionStartAt time.Time
	SubmissionEndAt   time.Time
	PlannedActiveFrom time.Time
	CreatedAt         time.Time
}

type DeletePublicationParams struct {
	ID  int64
	Now time.Time
}

type UpsertAvailabilitySubmissionParams struct {
	PublicationID    int64
	UserID           int64
	TemplateShiftID  int64
	PublicationState *model.PublicationState
	Now              time.Time
}

type DeleteAvailabilitySubmissionParams struct {
	PublicationID    int64
	UserID           int64
	TemplateShiftID  int64
	PublicationState *model.PublicationState
	Now              time.Time
}

type PublicationRepository struct {
	db *sql.DB
}

func NewPublicationRepository(db *sql.DB) *PublicationRepository {
	return &PublicationRepository{db: db}
}

func (r *PublicationRepository) ListPaginated(
	ctx context.Context,
	params ListPublicationsParams,
) ([]*model.Publication, int, error) {
	const countQuery = `
		SELECT COUNT(*)
		FROM publications;
	`

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	const query = `
		SELECT
			p.id,
			p.template_id,
			t.name,
			p.name,
			p.state,
			p.submission_start_at,
			p.submission_end_at,
			p.planned_active_from,
			p.activated_at,
			p.ended_at,
			p.created_at,
			p.updated_at
		FROM publications p
		INNER JOIN templates t ON t.id = p.template_id
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $1 OFFSET $2;
	`

	rows, err := r.db.QueryContext(ctx, query, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	publications := make([]*model.Publication, 0)
	for rows.Next() {
		publication, err := scanPublication(rows)
		if err != nil {
			return nil, 0, err
		}
		publications = append(publications, publication)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return publications, total, nil
}

func (r *PublicationRepository) GetByID(ctx context.Context, id int64) (*model.Publication, error) {
	return getPublicationByID(ctx, r.db, id)
}

func (r *PublicationRepository) GetCurrent(ctx context.Context) (*model.Publication, error) {
	const query = `
		SELECT
			p.id,
			p.template_id,
			t.name,
			p.name,
			p.state,
			p.submission_start_at,
			p.submission_end_at,
			p.planned_active_from,
			p.activated_at,
			p.ended_at,
			p.created_at,
			p.updated_at
		FROM publications p
		INNER JOIN templates t ON t.id = p.template_id
		WHERE p.state != 'ENDED'
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT 1;
	`

	publication, err := scanPublication(r.db.QueryRowContext(ctx, query))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	default:
		return publication, nil
	}
}

func (r *PublicationRepository) CreatePublication(
	ctx context.Context,
	params CreatePublicationParams,
) (*model.Publication, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	template, err := getTemplateByIDForUpdate(ctx, tx, params.TemplateID)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}

	if !template.IsLocked {
		const lockTemplateQuery = `
			UPDATE templates
			SET is_locked = TRUE, updated_at = $2
			WHERE id = $1;
		`

		if _, err := tx.ExecContext(ctx, lockTemplateQuery, params.TemplateID, params.CreatedAt); err != nil {
			return nil, err
		}
	}

	const query = `
		INSERT INTO publications (
			template_id,
			name,
			state,
			submission_start_at,
			submission_end_at,
			planned_active_from,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		RETURNING
			id,
			template_id,
			$8 AS template_name,
			name,
			state,
			submission_start_at,
			submission_end_at,
			planned_active_from,
			activated_at,
			ended_at,
			created_at,
			updated_at;
	`

	publication, err := scanPublication(tx.QueryRowContext(
		ctx,
		query,
		params.TemplateID,
		params.Name,
		params.State,
		params.SubmissionStartAt,
		params.SubmissionEndAt,
		params.PlannedActiveFrom,
		params.CreatedAt,
		template.Name,
	))
	if err != nil {
		return nil, mapPublicationWriteError(err)
	}

	if err := tx.Commit(); err != nil {
		return nil, mapPublicationWriteError(err)
	}

	return publication, nil
}

func (r *PublicationRepository) DeletePublication(ctx context.Context, params DeletePublicationParams) error {
	const query = `
		DELETE FROM publications
		WHERE id = $1
			AND state = 'DRAFT'
			AND submission_start_at > $2;
	`

	result, err := r.db.ExecContext(ctx, query, params.ID, params.Now)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrPublicationNotFound
	}

	return nil
}

func (r *PublicationRepository) ListSubmissionShiftIDs(
	ctx context.Context,
	publicationID, userID int64,
) ([]int64, error) {
	exists, err := publicationExists(ctx, r.db, publicationID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrPublicationNotFound
	}

	const query = `
		SELECT template_shift_id
		FROM availability_submissions
		WHERE publication_id = $1 AND user_id = $2
		ORDER BY template_shift_id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, publicationID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	shiftIDs := make([]int64, 0)
	for rows.Next() {
		var shiftID int64
		if err := rows.Scan(&shiftID); err != nil {
			return nil, err
		}
		shiftIDs = append(shiftIDs, shiftID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return shiftIDs, nil
}

func (r *PublicationRepository) UpsertSubmission(
	ctx context.Context,
	params UpsertAvailabilitySubmissionParams,
) (*model.AvailabilitySubmission, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := updatePublicationStateIfNeeded(ctx, tx, params.PublicationID, params.PublicationState, params.Now); err != nil {
		return nil, err
	}

	const query = `
		INSERT INTO availability_submissions (
			publication_id,
			user_id,
			template_shift_id,
			created_at
		)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (publication_id, user_id, template_shift_id) DO NOTHING
		RETURNING id, publication_id, user_id, template_shift_id, created_at;
	`

	submission, err := scanSubmission(tx.QueryRowContext(
		ctx,
		query,
		params.PublicationID,
		params.UserID,
		params.TemplateShiftID,
		params.Now,
	))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		submission, err = getSubmissionByKey(ctx, tx, params.PublicationID, params.UserID, params.TemplateShiftID)
		if err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return submission, nil
}

func (r *PublicationRepository) DeleteSubmission(
	ctx context.Context,
	params DeleteAvailabilitySubmissionParams,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := updatePublicationStateIfNeeded(ctx, tx, params.PublicationID, params.PublicationState, params.Now); err != nil {
		return err
	}

	const query = `
		DELETE FROM availability_submissions
		WHERE publication_id = $1 AND user_id = $2 AND template_shift_id = $3;
	`

	if _, err := tx.ExecContext(ctx, query, params.PublicationID, params.UserID, params.TemplateShiftID); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PublicationRepository) GetTemplateShift(
	ctx context.Context,
	templateID, shiftID int64,
) (*model.TemplateShift, error) {
	const query = `
		SELECT
			id,
			template_id,
			weekday,
			TO_CHAR(start_time, 'HH24:MI'),
			TO_CHAR(end_time, 'HH24:MI'),
			position_id,
			required_headcount,
			created_at,
			updated_at
		FROM template_shifts
		WHERE template_id = $1 AND id = $2;
	`

	shift := &model.TemplateShift{}
	err := r.db.QueryRowContext(ctx, query, templateID, shiftID).Scan(
		&shift.ID,
		&shift.TemplateID,
		&shift.Weekday,
		&shift.StartTime,
		&shift.EndTime,
		&shift.PositionID,
		&shift.RequiredHeadcount,
		&shift.CreatedAt,
		&shift.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTemplateShiftNotFound
	}
	if err != nil {
		return nil, err
	}

	return shift, nil
}

func (r *PublicationRepository) IsUserQualifiedForPosition(
	ctx context.Context,
	userID, positionID int64,
) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM user_positions
			WHERE user_id = $1 AND position_id = $2
		);
	`

	var qualified bool
	if err := r.db.QueryRowContext(ctx, query, userID, positionID).Scan(&qualified); err != nil {
		return false, err
	}

	return qualified, nil
}

func (r *PublicationRepository) ListQualifiedShifts(
	ctx context.Context,
	publicationID, userID int64,
) ([]*model.TemplateShift, error) {
	exists, err := publicationExists(ctx, r.db, publicationID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrPublicationNotFound
	}

	const query = `
		SELECT
			ts.id,
			ts.template_id,
			ts.weekday,
			TO_CHAR(ts.start_time, 'HH24:MI'),
			TO_CHAR(ts.end_time, 'HH24:MI'),
			ts.position_id,
			ts.required_headcount,
			ts.created_at,
			ts.updated_at
		FROM publications p
		INNER JOIN template_shifts ts ON ts.template_id = p.template_id
		INNER JOIN user_positions up ON up.position_id = ts.position_id
		WHERE p.id = $1 AND up.user_id = $2
		ORDER BY ts.weekday ASC, ts.start_time ASC, ts.id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, publicationID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	shifts := make([]*model.TemplateShift, 0)
	for rows.Next() {
		shift := &model.TemplateShift{}
		if err := rows.Scan(
			&shift.ID,
			&shift.TemplateID,
			&shift.Weekday,
			&shift.StartTime,
			&shift.EndTime,
			&shift.PositionID,
			&shift.RequiredHeadcount,
			&shift.CreatedAt,
			&shift.UpdatedAt,
		); err != nil {
			return nil, err
		}
		shifts = append(shifts, shift)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return shifts, nil
}

type dbtx interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type scanner interface {
	Scan(dest ...any) error
}

func getPublicationByID(ctx context.Context, db dbtx, id int64) (*model.Publication, error) {
	const query = `
		SELECT
			p.id,
			p.template_id,
			t.name,
			p.name,
			p.state,
			p.submission_start_at,
			p.submission_end_at,
			p.planned_active_from,
			p.activated_at,
			p.ended_at,
			p.created_at,
			p.updated_at
		FROM publications p
		INNER JOIN templates t ON t.id = p.template_id
		WHERE p.id = $1;
	`

	publication, err := scanPublication(db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPublicationNotFound
	}
	if err != nil {
		return nil, err
	}

	return publication, nil
}

func getTemplateByIDForUpdate(ctx context.Context, tx *sql.Tx, id int64) (*model.Template, error) {
	const query = `
		SELECT id, name, description, is_locked, created_at, updated_at
		FROM templates
		WHERE id = $1
		FOR UPDATE;
	`

	template := &model.Template{}
	err := tx.QueryRowContext(ctx, query, id).Scan(
		&template.ID,
		&template.Name,
		&template.Description,
		&template.IsLocked,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}

	return template, nil
}

func updatePublicationStateIfNeeded(
	ctx context.Context,
	tx *sql.Tx,
	publicationID int64,
	state *model.PublicationState,
	now time.Time,
) error {
	if state == nil {
		return nil
	}

	const query = `
		UPDATE publications
		SET state = $2, updated_at = $3
		WHERE id = $1;
	`

	result, err := tx.ExecContext(ctx, query, publicationID, *state, now)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrPublicationNotFound
	}

	return nil
}

func getSubmissionByKey(
	ctx context.Context,
	db dbtx,
	publicationID, userID, templateShiftID int64,
) (*model.AvailabilitySubmission, error) {
	const query = `
		SELECT id, publication_id, user_id, template_shift_id, created_at
		FROM availability_submissions
		WHERE publication_id = $1 AND user_id = $2 AND template_shift_id = $3;
	`

	return submissionFromRow(db.QueryRowContext(ctx, query, publicationID, userID, templateShiftID))
}

func publicationExists(ctx context.Context, db dbtx, publicationID int64) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM publications
			WHERE id = $1
		);
	`

	var exists bool
	if err := db.QueryRowContext(ctx, query, publicationID).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func scanPublication(row scanner) (*model.Publication, error) {
	publication := &model.Publication{}
	err := row.Scan(
		&publication.ID,
		&publication.TemplateID,
		&publication.TemplateName,
		&publication.Name,
		&publication.State,
		&publication.SubmissionStartAt,
		&publication.SubmissionEndAt,
		&publication.PlannedActiveFrom,
		&publication.ActivatedAt,
		&publication.EndedAt,
		&publication.CreatedAt,
		&publication.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return publication, nil
}

func scanSubmission(row scanner) (*model.AvailabilitySubmission, error) {
	submission := &model.AvailabilitySubmission{}
	err := row.Scan(
		&submission.ID,
		&submission.PublicationID,
		&submission.UserID,
		&submission.TemplateShiftID,
		&submission.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return submission, nil
}

func submissionFromRow(row scanner) (*model.AvailabilitySubmission, error) {
	submission, err := scanSubmission(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}

	return submission, nil
}

func mapPublicationWriteError(err error) error {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return err
	}

	switch {
	case pqErr.Code == "23505" && pqErr.Constraint == publicationsSingleNonEndedIndex:
		return ErrPublicationAlreadyExists
	case pqErr.Code == "23514" && pqErr.Constraint == publicationsWindowCheckName:
		return ErrInvalidPublicationWindow
	default:
		return err
	}
}
