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
	TemplateID         int64
	Name               string
	Description        string
	State              model.PublicationState
	SubmissionStartAt  time.Time
	SubmissionEndAt    time.Time
	PlannedActiveFrom  time.Time
	PlannedActiveUntil time.Time
	CreatedAt          time.Time
}

type UpdatePublicationFieldsParams struct {
	ID                 int64
	Name               *string
	Description        *string
	PlannedActiveUntil *time.Time
	UpdatedAt          time.Time
}

type DeletePublicationParams struct {
	ID  int64
	Now time.Time
}

type UpsertAvailabilitySubmissionParams struct {
	PublicationID    int64
	UserID           int64
	SlotID           int64
	Weekday          int
	PublicationState *model.PublicationState
	Now              time.Time
}

type DeleteAvailabilitySubmissionParams struct {
	PublicationID    int64
	UserID           int64
	SlotID           int64
	Weekday          int
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
			p.description,
			p.state,
			p.submission_start_at,
			p.submission_end_at,
			p.planned_active_from,
			p.planned_active_until,
			p.activated_at,
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
			p.description,
			p.state,
			p.submission_start_at,
			p.submission_end_at,
			p.planned_active_from,
			p.planned_active_until,
			p.activated_at,
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

	const sweepQuery = `
		UPDATE publications
		SET state = 'ENDED', updated_at = $1
		WHERE state = 'ACTIVE' AND planned_active_until <= $1;
	`
	if _, err := tx.ExecContext(ctx, sweepQuery, params.CreatedAt); err != nil {
		return nil, err
	}

	const query = `
		INSERT INTO publications (
			template_id,
			name,
			description,
			state,
			submission_start_at,
			submission_end_at,
			planned_active_from,
			planned_active_until,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		RETURNING
			id,
			template_id,
			$10 AS template_name,
			name,
			description,
			state,
			submission_start_at,
			submission_end_at,
			planned_active_from,
			planned_active_until,
			activated_at,
			created_at,
			updated_at;
	`

	publication, err := scanPublication(tx.QueryRowContext(
		ctx,
		query,
		params.TemplateID,
		params.Name,
		params.Description,
		params.State,
		params.SubmissionStartAt,
		params.SubmissionEndAt,
		params.PlannedActiveFrom,
		params.PlannedActiveUntil,
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

func (r *PublicationRepository) UpdatePublicationFields(
	ctx context.Context,
	params UpdatePublicationFieldsParams,
) (*model.Publication, error) {
	const query = `
		UPDATE publications p
		SET
			name = COALESCE($2, p.name),
			description = COALESCE($3, p.description),
			planned_active_until = COALESCE($4, p.planned_active_until),
			updated_at = $5
		FROM templates t
		WHERE p.id = $1
			AND p.template_id = t.id
		RETURNING
			p.id,
			p.template_id,
			t.name,
			p.name,
			p.description,
			p.state,
			p.submission_start_at,
			p.submission_end_at,
			p.planned_active_from,
			p.planned_active_until,
			p.activated_at,
			p.created_at,
			p.updated_at;
	`

	var name sql.NullString
	if params.Name != nil {
		name = sql.NullString{String: *params.Name, Valid: true}
	}
	var description sql.NullString
	if params.Description != nil {
		description = sql.NullString{String: *params.Description, Valid: true}
	}
	var plannedActiveUntil sql.NullTime
	if params.PlannedActiveUntil != nil {
		plannedActiveUntil = sql.NullTime{Time: *params.PlannedActiveUntil, Valid: true}
	}

	publication, err := scanPublication(r.db.QueryRowContext(
		ctx,
		query,
		params.ID,
		name,
		description,
		plannedActiveUntil,
		params.UpdatedAt,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPublicationNotFound
	}
	if err != nil {
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

func (r *PublicationRepository) ListSubmissionSlots(
	ctx context.Context,
	publicationID, userID int64,
) ([]model.SlotRef, error) {
	exists, err := publicationExists(ctx, r.db, publicationID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrPublicationNotFound
	}

	const query = `
		SELECT asub.slot_id, asub.weekday
		FROM availability_submissions asub
		WHERE asub.publication_id = $1 AND asub.user_id = $2
		ORDER BY asub.weekday ASC, asub.slot_id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, publicationID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	slots := make([]model.SlotRef, 0)
	for rows.Next() {
		var slot model.SlotRef
		if err := rows.Scan(&slot.SlotID, &slot.Weekday); err != nil {
			return nil, err
		}
		slots = append(slots, slot)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return slots, nil
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

	if err := ensureSlotWeekdayBelongsToPublicationTemplate(
		ctx,
		tx,
		params.PublicationID,
		params.SlotID,
		params.Weekday,
	); err != nil {
		return nil, err
	}

	const query = `
		INSERT INTO availability_submissions (
			publication_id,
			user_id,
			slot_id,
			weekday,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (publication_id, user_id, slot_id, weekday) DO NOTHING
		RETURNING id, publication_id, user_id, slot_id, weekday, created_at;
	`

	submission, err := scanSubmission(tx.QueryRowContext(
		ctx,
		query,
		params.PublicationID,
		params.UserID,
		params.SlotID,
		params.Weekday,
		params.Now,
	))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		submission, err = getSubmissionByKey(
			ctx,
			tx,
			params.PublicationID,
			params.UserID,
			params.SlotID,
			params.Weekday,
		)
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
		WHERE publication_id = $1 AND user_id = $2 AND slot_id = $3 AND weekday = $4;
	`

	if _, err := tx.ExecContext(
		ctx,
		query,
		params.PublicationID,
		params.UserID,
		params.SlotID,
		params.Weekday,
	); err != nil {
		return err
	}

	return tx.Commit()
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

func (r *PublicationRepository) ListQualifiedPublicationSlotPositions(
	ctx context.Context,
	publicationID, userID int64,
) ([]*model.QualifiedShift, error) {
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
			tsw.weekday,
			TO_CHAR(ts.start_time, 'HH24:MI'),
			TO_CHAR(ts.end_time, 'HH24:MI'),
			tsp.position_id,
			pos.name,
			tsp.required_headcount
		FROM publications p
		INNER JOIN template_slots ts ON ts.template_id = p.template_id
		INNER JOIN template_slot_weekdays tsw ON tsw.slot_id = ts.id
		INNER JOIN template_slot_positions tsp ON tsp.slot_id = ts.id
		INNER JOIN positions pos ON pos.id = tsp.position_id
		WHERE p.id = $1
			AND EXISTS (
				SELECT 1
				FROM template_slot_positions qualified_tsp
				INNER JOIN user_positions up ON up.position_id = qualified_tsp.position_id
				WHERE qualified_tsp.slot_id = ts.id
					AND up.user_id = $2
			)
		ORDER BY tsw.weekday ASC, ts.start_time ASC, ts.id ASC, tsp.position_id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, publicationID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	shifts := make([]*model.QualifiedShift, 0)
	shiftsBySlotRef := make(map[model.SlotRef]*model.QualifiedShift)
	for rows.Next() {
		var (
			slotID            int64
			weekday           int
			startTime         string
			endTime           string
			positionID        int64
			positionName      string
			requiredHeadcount int
		)
		if err := rows.Scan(
			&slotID,
			&weekday,
			&startTime,
			&endTime,
			&positionID,
			&positionName,
			&requiredHeadcount,
		); err != nil {
			return nil, err
		}
		slotRef := model.SlotRef{SlotID: slotID, Weekday: weekday}
		shift := shiftsBySlotRef[slotRef]
		if shift == nil {
			shift = &model.QualifiedShift{
				SlotID:      slotID,
				Weekday:     weekday,
				StartTime:   startTime,
				EndTime:     endTime,
				Composition: make([]model.QualifiedShiftComposition, 0),
			}
			shiftsBySlotRef[slotRef] = shift
			shifts = append(shifts, shift)
		}
		shift.Composition = append(shift.Composition, model.QualifiedShiftComposition{
			PositionID:        positionID,
			PositionName:      positionName,
			RequiredHeadcount: requiredHeadcount,
		})
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
			p.description,
			p.state,
			p.submission_start_at,
			p.submission_end_at,
			p.planned_active_from,
			p.planned_active_until,
			p.activated_at,
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

func ensureSlotBelongsToPublicationTemplate(
	ctx context.Context,
	db dbtx,
	publicationID, slotID int64,
) error {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM publications p
			INNER JOIN template_slots ts ON ts.template_id = p.template_id
			WHERE p.id = $1
				AND ts.id = $2
		);
	`

	var exists bool
	if err := db.QueryRowContext(ctx, query, publicationID, slotID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrTemplateSlotNotFound
	}

	return nil
}

func ensureSlotWeekdayBelongsToPublicationTemplate(
	ctx context.Context,
	db dbtx,
	publicationID, slotID int64,
	weekday int,
) error {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM publications p
			INNER JOIN template_slots ts ON ts.template_id = p.template_id
			INNER JOIN template_slot_weekdays tsw ON tsw.slot_id = ts.id
			WHERE p.id = $1
				AND ts.id = $2
				AND tsw.weekday = $3
		);
	`

	var exists bool
	if err := db.QueryRowContext(ctx, query, publicationID, slotID, weekday).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrTemplateSlotNotFound
	}

	return nil
}

func getSubmissionByKey(
	ctx context.Context,
	db dbtx,
	publicationID, userID, slotID int64,
	weekday int,
) (*model.AvailabilitySubmission, error) {
	const query = `
		SELECT
			asub.id,
			asub.publication_id,
			asub.user_id,
			asub.slot_id,
			asub.weekday,
			asub.created_at
		FROM availability_submissions asub
		WHERE asub.publication_id = $1
			AND asub.user_id = $2
			AND asub.slot_id = $3
			AND asub.weekday = $4;
	`

	return submissionFromRow(db.QueryRowContext(ctx, query, publicationID, userID, slotID, weekday))
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
		&publication.Description,
		&publication.State,
		&publication.SubmissionStartAt,
		&publication.SubmissionEndAt,
		&publication.PlannedActiveFrom,
		&publication.PlannedActiveUntil,
		&publication.ActivatedAt,
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
		&submission.SlotID,
		&submission.Weekday,
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
