package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/lib/pq"
)

var (
	ErrTemplateLocked      = model.ErrTemplateLocked
	ErrTemplateNotFound    = errors.New("template not found")
	ErrTemplateSlotOverlap = model.ErrTemplateSlotOverlap
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

type CreateTemplateSlotParams struct {
	TemplateID int64
	Weekday    int
	StartTime  string
	EndTime    string
}

type UpdateTemplateSlotParams struct {
	TemplateID int64
	SlotID     int64
	Weekday    int
	StartTime  string
	EndTime    string
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
		LEFT JOIN template_slots ts ON ts.template_id = t.id
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

	return template, populateTemplateSlots(ctx, r.db, template)
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

	sourceSlots, err := listTemplateSlots(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	template.Slots = make([]*model.TemplateSlot, 0, len(sourceSlots))

	for _, sourceSlot := range sourceSlots {
		const insertSlotQuery = `
			INSERT INTO template_slots (
				template_id,
				weekday,
				start_time,
				end_time
			)
			VALUES ($1, $2, $3, $4)
			RETURNING
				id,
				template_id,
				weekday,
				TO_CHAR(start_time, 'HH24:MI'),
				TO_CHAR(end_time, 'HH24:MI'),
				created_at,
				updated_at;
		`

		clonedSlot := &model.TemplateSlot{}
		err := tx.QueryRowContext(
			ctx,
			insertSlotQuery,
			template.ID,
			sourceSlot.Weekday,
			sourceSlot.StartTime,
			sourceSlot.EndTime,
		).Scan(
			&clonedSlot.ID,
			&clonedSlot.TemplateID,
			&clonedSlot.Weekday,
			&clonedSlot.StartTime,
			&clonedSlot.EndTime,
			&clonedSlot.CreatedAt,
			&clonedSlot.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		sourcePositions, err := listTemplateSlotPositions(ctx, tx, sourceSlot.ID)
		if err != nil {
			return nil, err
		}
		clonedSlot.Positions = make([]*model.TemplateSlotPosition, 0, len(sourcePositions))

		for _, sourcePosition := range sourcePositions {
			const insertSlotPositionQuery = `
				INSERT INTO template_slot_positions (
					slot_id,
					position_id,
					required_headcount
				)
				VALUES ($1, $2, $3)
				RETURNING
					id,
					slot_id,
					position_id,
					required_headcount,
					created_at,
					updated_at;
			`

			clonedPosition := &model.TemplateSlotPosition{}
			err := tx.QueryRowContext(
				ctx,
				insertSlotPositionQuery,
				clonedSlot.ID,
				sourcePosition.PositionID,
				sourcePosition.RequiredHeadcount,
			).Scan(
				&clonedPosition.ID,
				&clonedPosition.SlotID,
				&clonedPosition.PositionID,
				&clonedPosition.RequiredHeadcount,
				&clonedPosition.CreatedAt,
				&clonedPosition.UpdatedAt,
			)
			if err != nil {
				return nil, err
			}

			clonedSlot.Positions = append(clonedSlot.Positions, clonedPosition)
		}

		template.Slots = append(template.Slots, clonedSlot)
	}
	template.ShiftCount = len(template.Slots)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return template, nil
}

func (r *TemplateRepository) CreateSlot(
	ctx context.Context,
	params CreateTemplateSlotParams,
) (*model.TemplateSlot, error) {
	const query = `
		INSERT INTO template_slots (
			template_id,
			weekday,
			start_time,
			end_time
		)
		SELECT
			t.id,
			$2,
			$3,
			$4
		FROM templates t
		WHERE t.id = $1 AND t.is_locked = FALSE
		RETURNING
			id,
			template_id,
			weekday,
			TO_CHAR(start_time, 'HH24:MI'),
			TO_CHAR(end_time, 'HH24:MI'),
			created_at,
			updated_at;
	`

	slot := &model.TemplateSlot{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.TemplateID,
		params.Weekday,
		params.StartTime,
		params.EndTime,
	).Scan(
		&slot.ID,
		&slot.TemplateID,
		&slot.Weekday,
		&slot.StartTime,
		&slot.EndTime,
		&slot.CreatedAt,
		&slot.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, r.resolveTemplateWriteState(ctx, params.TemplateID)
	}
	if err != nil {
		return nil, mapTemplateSlotWriteError(err)
	}

	return slot, nil
}

func (r *TemplateRepository) UpdateSlot(
	ctx context.Context,
	params UpdateTemplateSlotParams,
) (*model.TemplateSlot, error) {
	const query = `
		UPDATE template_slots
		SET
			weekday = $3,
			start_time = $4,
			end_time = $5,
			updated_at = NOW()
		FROM templates
		WHERE
			template_slots.template_id = $1 AND
			template_slots.id = $2 AND
			templates.id = template_slots.template_id AND
			templates.is_locked = FALSE
		RETURNING
			template_slots.id,
			template_slots.template_id,
			template_slots.weekday,
			TO_CHAR(template_slots.start_time, 'HH24:MI'),
			TO_CHAR(template_slots.end_time, 'HH24:MI'),
			template_slots.created_at,
			template_slots.updated_at;
	`

	slot := &model.TemplateSlot{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.TemplateID,
		params.SlotID,
		params.Weekday,
		params.StartTime,
		params.EndTime,
	).Scan(
		&slot.ID,
		&slot.TemplateID,
		&slot.Weekday,
		&slot.StartTime,
		&slot.EndTime,
		&slot.CreatedAt,
		&slot.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, r.resolveTemplateSlotWriteState(ctx, params.TemplateID, params.SlotID)
	}
	if err != nil {
		return nil, mapTemplateSlotWriteError(err)
	}

	return slot, nil
}

func (r *TemplateRepository) DeleteSlot(ctx context.Context, templateID, slotID int64) error {
	const query = `
		DELETE FROM template_slots
		USING templates
		WHERE
			template_slots.template_id = $1 AND
			template_slots.id = $2 AND
			templates.id = template_slots.template_id AND
			templates.is_locked = FALSE;
	`

	result, err := r.db.ExecContext(ctx, query, templateID, slotID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return r.resolveTemplateSlotWriteState(ctx, templateID, slotID)
	}

	return nil
}

func (r *TemplateRepository) GetSlot(
	ctx context.Context,
	templateID, slotID int64,
) (*model.TemplateSlot, error) {
	return getTemplateSlotByID(ctx, r.db, templateID, slotID)
}

func (r *TemplateRepository) ListSlotsByTemplate(
	ctx context.Context,
	templateID int64,
) ([]*model.TemplateSlot, error) {
	return listTemplateSlots(ctx, r.db, templateID)
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
		LEFT JOIN template_slots ts ON ts.template_id = t.id
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

func populateTemplateSlots(ctx context.Context, db queryer, template *model.Template) error {
	slots, err := listTemplateSlots(ctx, db, template.ID)
	if err != nil {
		return err
	}

	for _, slot := range slots {
		slotPositions, err := listTemplateSlotPositions(ctx, db, slot.ID)
		if err != nil {
			return err
		}
		slot.Positions = slotPositions
	}

	template.Slots = slots
	template.ShiftCount = len(slots)

	return nil
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

func mapTemplateSlotWriteError(err error) error {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return err
	}

	if pqErr.Code == "23P01" {
		return ErrTemplateSlotOverlap
	}

	return err
}
