package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

var (
	ErrTemplateLocked        = model.ErrTemplateLocked
	ErrTemplateNotFound      = errors.New("template not found")
	ErrTemplateShiftNotFound = errors.New("template shift not found")
)

type ListTemplatesParams struct {
	Offset int
	Limit  int
}

type CreateTemplateParams struct {
	Name        string
	Description string
}

type UpdateTemplateParams struct {
	ID          int64
	Name        string
	Description string
}

type CreateTemplateShiftParams struct {
	TemplateID        int64
	Weekday           int
	StartTime         string
	EndTime           string
	PositionID        int64
	RequiredHeadcount int
}

type UpdateTemplateShiftParams struct {
	TemplateID        int64
	ShiftID           int64
	Weekday           int
	StartTime         string
	EndTime           string
	PositionID        int64
	RequiredHeadcount int
}

type TemplateRepository struct {
	db *sql.DB
}

func NewTemplateRepository(db *sql.DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

func (r *TemplateRepository) ListPaginated(ctx context.Context, params ListTemplatesParams) ([]*model.Template, int, error) {
	const countQuery = `
		SELECT COUNT(*)
		FROM templates;
	`

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	const query = `
		SELECT
			t.id,
			t.name,
			t.description,
			t.is_locked,
			t.created_at,
			t.updated_at,
			COALESCE(COUNT(ts.id), 0) AS shift_count
		FROM templates t
		LEFT JOIN template_shifts ts ON ts.template_id = t.id
		GROUP BY t.id
		ORDER BY t.updated_at DESC, t.id DESC
		LIMIT $1 OFFSET $2;
	`

	rows, err := r.db.QueryContext(ctx, query, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	templates := make([]*model.Template, 0)
	for rows.Next() {
		template := &model.Template{}
		if err := rows.Scan(
			&template.ID,
			&template.Name,
			&template.Description,
			&template.IsLocked,
			&template.CreatedAt,
			&template.UpdatedAt,
			&template.ShiftCount,
		); err != nil {
			return nil, 0, err
		}
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return templates, total, nil
}

func (r *TemplateRepository) GetByID(ctx context.Context, id int64) (*model.Template, error) {
	template, err := getTemplateByID(ctx, r.db, id)
	if err != nil {
		return nil, err
	}

	shifts, err := listTemplateShifts(ctx, r.db, id)
	if err != nil {
		return nil, err
	}
	template.Shifts = shifts
	template.ShiftCount = len(shifts)

	return template, nil
}

func (r *TemplateRepository) Create(ctx context.Context, params CreateTemplateParams) (*model.Template, error) {
	const query = `
		INSERT INTO templates (name, description)
		VALUES ($1, $2)
		RETURNING id, name, description, is_locked, created_at, updated_at;
	`

	template := &model.Template{}
	err := r.db.QueryRowContext(ctx, query, params.Name, params.Description).Scan(
		&template.ID,
		&template.Name,
		&template.Description,
		&template.IsLocked,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return template, nil
}

func (r *TemplateRepository) Update(ctx context.Context, params UpdateTemplateParams) (*model.Template, error) {
	const query = `
		UPDATE templates
		SET name = $2, description = $3, updated_at = NOW()
		WHERE id = $1 AND is_locked = FALSE
		RETURNING id, name, description, is_locked, created_at, updated_at;
	`

	template := &model.Template{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.ID,
		params.Name,
		params.Description,
	).Scan(
		&template.ID,
		&template.Name,
		&template.Description,
		&template.IsLocked,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, r.resolveTemplateWriteState(ctx, params.ID)
	}
	if err != nil {
		return nil, err
	}

	return template, nil
}

func (r *TemplateRepository) Delete(ctx context.Context, id int64) error {
	const query = `
		DELETE FROM templates
		WHERE id = $1 AND is_locked = FALSE;
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return r.resolveTemplateWriteState(ctx, id)
	}

	return nil
}

func (r *TemplateRepository) Clone(ctx context.Context, id int64, name string) (*model.Template, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	const cloneTemplateQuery = `
		INSERT INTO templates (name, description, is_locked)
		SELECT $2, description, FALSE
		FROM templates
		WHERE id = $1
		RETURNING id, name, description, is_locked, created_at, updated_at;
	`

	template := &model.Template{}
	err = tx.QueryRowContext(ctx, cloneTemplateQuery, id, name).Scan(
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

	const cloneShiftsQuery = `
		INSERT INTO template_shifts (
			template_id,
			weekday,
			start_time,
			end_time,
			position_id,
			required_headcount
		)
		SELECT
			$2,
			weekday,
			start_time,
			end_time,
			position_id,
			required_headcount
		FROM template_shifts
		WHERE template_id = $1;
	`

	if _, err := tx.ExecContext(ctx, cloneShiftsQuery, id, template.ID); err != nil {
		return nil, err
	}

	shifts, err := listTemplateShifts(ctx, tx, template.ID)
	if err != nil {
		return nil, err
	}
	template.Shifts = shifts
	template.ShiftCount = len(shifts)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return template, nil
}

func (r *TemplateRepository) CreateShift(ctx context.Context, params CreateTemplateShiftParams) (*model.TemplateShift, error) {
	const query = `
		INSERT INTO template_shifts (
			template_id,
			weekday,
			start_time,
			end_time,
			position_id,
			required_headcount
		)
		SELECT
			t.id,
			$2,
			$3,
			$4,
			$5,
			$6
		FROM templates t
		WHERE t.id = $1 AND t.is_locked = FALSE
		RETURNING
			template_shifts.id,
			template_shifts.template_id,
			template_shifts.weekday,
			TO_CHAR(template_shifts.start_time, 'HH24:MI'),
			TO_CHAR(template_shifts.end_time, 'HH24:MI'),
			template_shifts.position_id,
			template_shifts.required_headcount,
			template_shifts.created_at,
			template_shifts.updated_at;
	`

	shift := &model.TemplateShift{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.TemplateID,
		params.Weekday,
		params.StartTime,
		params.EndTime,
		params.PositionID,
		params.RequiredHeadcount,
	).Scan(
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
		return nil, r.resolveTemplateWriteState(ctx, params.TemplateID)
	}
	if err != nil {
		return nil, err
	}

	return shift, nil
}

func (r *TemplateRepository) UpdateShift(ctx context.Context, params UpdateTemplateShiftParams) (*model.TemplateShift, error) {
	const query = `
		UPDATE template_shifts
		SET
			weekday = $3,
			start_time = $4,
			end_time = $5,
			position_id = $6,
			required_headcount = $7,
			updated_at = NOW()
		FROM templates
		WHERE
			template_shifts.template_id = $1 AND
			template_shifts.id = $2 AND
			templates.id = template_shifts.template_id AND
			templates.is_locked = FALSE
		RETURNING
			template_shifts.id,
			template_shifts.template_id,
			template_shifts.weekday,
			TO_CHAR(template_shifts.start_time, 'HH24:MI'),
			TO_CHAR(template_shifts.end_time, 'HH24:MI'),
			template_shifts.position_id,
			template_shifts.required_headcount,
			template_shifts.created_at,
			template_shifts.updated_at;
	`

	shift := &model.TemplateShift{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.TemplateID,
		params.ShiftID,
		params.Weekday,
		params.StartTime,
		params.EndTime,
		params.PositionID,
		params.RequiredHeadcount,
	).Scan(
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
		return nil, r.resolveTemplateShiftWriteState(ctx, params.TemplateID, params.ShiftID)
	}
	if err != nil {
		return nil, err
	}

	return shift, nil
}

func (r *TemplateRepository) DeleteShift(ctx context.Context, templateID, shiftID int64) error {
	const query = `
		DELETE FROM template_shifts
		USING templates
		WHERE
			template_shifts.template_id = $1 AND
			template_shifts.id = $2 AND
			templates.id = template_shifts.template_id AND
			templates.is_locked = FALSE;
	`

	result, err := r.db.ExecContext(ctx, query, templateID, shiftID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return r.resolveTemplateShiftWriteState(ctx, templateID, shiftID)
	}

	return nil
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func getTemplateByID(ctx context.Context, db queryer, id int64) (*model.Template, error) {
	const query = `
		SELECT
			t.id,
			t.name,
			t.description,
			t.is_locked,
			t.created_at,
			t.updated_at,
			COALESCE(COUNT(ts.id), 0) AS shift_count
		FROM templates t
		LEFT JOIN template_shifts ts ON ts.template_id = t.id
		WHERE t.id = $1
		GROUP BY t.id;
	`

	template := &model.Template{}
	err := db.QueryRowContext(ctx, query, id).Scan(
		&template.ID,
		&template.Name,
		&template.Description,
		&template.IsLocked,
		&template.CreatedAt,
		&template.UpdatedAt,
		&template.ShiftCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}

	return template, nil
}

func listTemplateShifts(ctx context.Context, db queryer, templateID int64) ([]*model.TemplateShift, error) {
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
		WHERE template_id = $1
		ORDER BY weekday ASC, start_time ASC, id ASC;
	`

	rows, err := db.QueryContext(ctx, query, templateID)
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

func (r *TemplateRepository) resolveTemplateWriteState(ctx context.Context, templateID int64) error {
	template, err := getTemplateByID(ctx, r.db, templateID)
	switch {
	case errors.Is(err, ErrTemplateNotFound):
		return ErrTemplateNotFound
	case err != nil:
		return err
	case template.IsLocked:
		return ErrTemplateLocked
	default:
		return ErrTemplateNotFound
	}
}

func (r *TemplateRepository) resolveTemplateShiftWriteState(ctx context.Context, templateID, shiftID int64) error {
	template, err := getTemplateByID(ctx, r.db, templateID)
	switch {
	case errors.Is(err, ErrTemplateNotFound):
		return ErrTemplateNotFound
	case err != nil:
		return err
	case template.IsLocked:
		return ErrTemplateLocked
	}

	exists, err := templateShiftExists(ctx, r.db, templateID, shiftID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrTemplateShiftNotFound
	}

	return ErrTemplateShiftNotFound
}

func templateShiftExists(ctx context.Context, db queryer, templateID, shiftID int64) (bool, error) {
	const query = `
		SELECT 1
		FROM template_shifts
		WHERE template_id = $1 AND id = $2;
	`

	var exists int
	err := db.QueryRowContext(ctx, query, templateID, shiftID).Scan(&exists)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, err
	default:
		return true, nil
	}
}
