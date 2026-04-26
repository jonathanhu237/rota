package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/lib/pq"
)

var (
	ErrTemplateSlotNotFound         = model.ErrTemplateSlotNotFound
	ErrTemplateSlotPositionNotFound = model.ErrTemplateSlotPositionNotFound
)

type CreateTemplateSlotPositionParams struct {
	TemplateID        int64
	SlotID            int64
	PositionID        int64
	RequiredHeadcount int
}

type UpdateTemplateSlotPositionParams struct {
	TemplateID        int64
	SlotID            int64
	SlotPositionID    int64
	PositionID        int64
	RequiredHeadcount int
}

func (r *TemplateRepository) CreateSlotPosition(
	ctx context.Context,
	params CreateTemplateSlotPositionParams,
) (*model.TemplateSlotPosition, error) {
	const query = `
		INSERT INTO template_slot_positions (
			slot_id,
			position_id,
			required_headcount
		)
		SELECT
			ts.id,
			$3,
			$4
		FROM template_slots ts
		INNER JOIN templates t ON t.id = ts.template_id
		WHERE ts.id = $2 AND ts.template_id = $1 AND t.is_locked = FALSE
		RETURNING
			id,
			slot_id,
			position_id,
			required_headcount,
			created_at,
			updated_at;
	`

	slotPosition := &model.TemplateSlotPosition{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.TemplateID,
		params.SlotID,
		params.PositionID,
		params.RequiredHeadcount,
	).Scan(
		&slotPosition.ID,
		&slotPosition.SlotID,
		&slotPosition.PositionID,
		&slotPosition.RequiredHeadcount,
		&slotPosition.CreatedAt,
		&slotPosition.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, r.resolveTemplateSlotWriteState(ctx, params.TemplateID, params.SlotID)
	}
	if err != nil {
		return nil, err
	}

	return slotPosition, nil
}

func (r *TemplateRepository) UpdateSlotPosition(
	ctx context.Context,
	params UpdateTemplateSlotPositionParams,
) (*model.TemplateSlotPosition, error) {
	const query = `
		UPDATE template_slot_positions
		SET
			position_id = $4,
			required_headcount = $5,
			updated_at = NOW()
		FROM template_slots, templates
		WHERE
			template_slot_positions.slot_id = $2 AND
			template_slot_positions.id = $3 AND
			template_slots.id = template_slot_positions.slot_id AND
			template_slots.template_id = $1 AND
			templates.id = template_slots.template_id AND
			templates.is_locked = FALSE
		RETURNING
			template_slot_positions.id,
			template_slot_positions.slot_id,
			template_slot_positions.position_id,
			template_slot_positions.required_headcount,
			template_slot_positions.created_at,
			template_slot_positions.updated_at;
	`

	slotPosition := &model.TemplateSlotPosition{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.TemplateID,
		params.SlotID,
		params.SlotPositionID,
		params.PositionID,
		params.RequiredHeadcount,
	).Scan(
		&slotPosition.ID,
		&slotPosition.SlotID,
		&slotPosition.PositionID,
		&slotPosition.RequiredHeadcount,
		&slotPosition.CreatedAt,
		&slotPosition.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, r.resolveTemplateSlotPositionWriteState(ctx, params.TemplateID, params.SlotID, params.SlotPositionID)
	}
	if err != nil {
		return nil, err
	}

	return slotPosition, nil
}

func (r *TemplateRepository) DeleteSlotPosition(
	ctx context.Context,
	templateID, slotID, slotPositionID int64,
) error {
	const query = `
		DELETE FROM template_slot_positions
		USING template_slots, templates
		WHERE
			template_slot_positions.slot_id = $2 AND
			template_slot_positions.id = $3 AND
			template_slots.id = template_slot_positions.slot_id AND
			template_slots.template_id = $1 AND
			templates.id = template_slots.template_id AND
			templates.is_locked = FALSE;
	`

	result, err := r.db.ExecContext(ctx, query, templateID, slotID, slotPositionID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return r.resolveTemplateSlotPositionWriteState(ctx, templateID, slotID, slotPositionID)
	}

	return nil
}

func (r *TemplateRepository) GetSlotPosition(
	ctx context.Context,
	templateID, slotID, slotPositionID int64,
) (*model.TemplateSlotPosition, error) {
	return getTemplateSlotPositionByID(ctx, r.db, templateID, slotID, slotPositionID)
}

func (r *TemplateRepository) ListSlotPositions(
	ctx context.Context,
	slotID int64,
) ([]*model.TemplateSlotPosition, error) {
	return listTemplateSlotPositions(ctx, r.db, slotID)
}

func listTemplateSlotPositions(
	ctx context.Context,
	db queryer,
	slotID int64,
) ([]*model.TemplateSlotPosition, error) {
	const query = `
		SELECT
			id,
			slot_id,
			position_id,
			required_headcount,
			created_at,
			updated_at
		FROM template_slot_positions
		WHERE slot_id = $1
		ORDER BY position_id ASC, id ASC;
	`

	rows, err := db.QueryContext(ctx, query, slotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	slotPositions := make([]*model.TemplateSlotPosition, 0)
	for rows.Next() {
		slotPosition := &model.TemplateSlotPosition{}
		if err := rows.Scan(
			&slotPosition.ID,
			&slotPosition.SlotID,
			&slotPosition.PositionID,
			&slotPosition.RequiredHeadcount,
			&slotPosition.CreatedAt,
			&slotPosition.UpdatedAt,
		); err != nil {
			return nil, err
		}
		slotPositions = append(slotPositions, slotPosition)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return slotPositions, nil
}

func getTemplateSlotByID(
	ctx context.Context,
	db queryer,
	templateID, slotID int64,
) (*model.TemplateSlot, error) {
	const query = `
		SELECT
			ts.id,
			ts.template_id,
			COALESCE(
				array_agg(tsw.weekday ORDER BY tsw.weekday)
					FILTER (WHERE tsw.weekday IS NOT NULL),
				ARRAY[]::integer[]
			),
			TO_CHAR(ts.start_time, 'HH24:MI'),
			TO_CHAR(ts.end_time, 'HH24:MI'),
			ts.created_at,
			ts.updated_at
		FROM template_slots ts
		LEFT JOIN template_slot_weekdays tsw ON tsw.slot_id = ts.id
		WHERE ts.template_id = $1 AND ts.id = $2
		GROUP BY ts.id;
	`

	slot, err := scanTemplateSlot(db.QueryRowContext(ctx, query, templateID, slotID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTemplateSlotNotFound
	}
	if err != nil {
		return nil, err
	}

	return slot, nil
}

func listTemplateSlots(
	ctx context.Context,
	db queryer,
	templateID int64,
) ([]*model.TemplateSlot, error) {
	const query = `
		SELECT
			ts.id,
			ts.template_id,
			COALESCE(
				array_agg(tsw.weekday ORDER BY tsw.weekday)
					FILTER (WHERE tsw.weekday IS NOT NULL),
				ARRAY[]::integer[]
			),
			TO_CHAR(ts.start_time, 'HH24:MI'),
			TO_CHAR(ts.end_time, 'HH24:MI'),
			ts.created_at,
			ts.updated_at
		FROM template_slots ts
		LEFT JOIN template_slot_weekdays tsw ON tsw.slot_id = ts.id
		WHERE ts.template_id = $1
		GROUP BY ts.id
		ORDER BY ts.start_time ASC, ts.end_time ASC, ts.id ASC;
	`

	rows, err := db.QueryContext(ctx, query, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	slots := make([]*model.TemplateSlot, 0)
	for rows.Next() {
		slot, err := scanTemplateSlot(rows)
		if err != nil {
			return nil, err
		}
		slots = append(slots, slot)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return slots, nil
}

func scanTemplateSlot(row scanner) (*model.TemplateSlot, error) {
	slot := &model.TemplateSlot{}
	var weekdays pq.Int64Array
	if err := row.Scan(
		&slot.ID,
		&slot.TemplateID,
		&weekdays,
		&slot.StartTime,
		&slot.EndTime,
		&slot.CreatedAt,
		&slot.UpdatedAt,
	); err != nil {
		return nil, err
	}
	slot.Weekdays = make([]int, 0, len(weekdays))
	for _, weekday := range weekdays {
		slot.Weekdays = append(slot.Weekdays, int(weekday))
	}
	return slot, nil
}

func getTemplateSlotPositionByID(
	ctx context.Context,
	db queryer,
	templateID, slotID, slotPositionID int64,
) (*model.TemplateSlotPosition, error) {
	const query = `
		SELECT
			tsp.id,
			tsp.slot_id,
			tsp.position_id,
			tsp.required_headcount,
			tsp.created_at,
			tsp.updated_at
		FROM template_slot_positions tsp
		INNER JOIN template_slots ts ON ts.id = tsp.slot_id
		WHERE ts.template_id = $1 AND tsp.slot_id = $2 AND tsp.id = $3;
	`

	slotPosition := &model.TemplateSlotPosition{}
	err := db.QueryRowContext(ctx, query, templateID, slotID, slotPositionID).Scan(
		&slotPosition.ID,
		&slotPosition.SlotID,
		&slotPosition.PositionID,
		&slotPosition.RequiredHeadcount,
		&slotPosition.CreatedAt,
		&slotPosition.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTemplateSlotPositionNotFound
	}
	if err != nil {
		return nil, err
	}

	return slotPosition, nil
}

func (r *TemplateRepository) resolveTemplateSlotWriteState(
	ctx context.Context,
	templateID, slotID int64,
) error {
	template, err := getTemplateRecordByID(ctx, r.db, templateID)
	switch {
	case errors.Is(err, ErrTemplateNotFound):
		return ErrTemplateNotFound
	case err != nil:
		return err
	case template.IsLocked:
		return ErrTemplateLocked
	}

	_, err = getTemplateSlotByID(ctx, r.db, templateID, slotID)
	switch {
	case errors.Is(err, ErrTemplateSlotNotFound):
		return ErrTemplateSlotNotFound
	case err != nil:
		return err
	default:
		return ErrTemplateSlotNotFound
	}
}

func (r *TemplateRepository) resolveTemplateSlotPositionWriteState(
	ctx context.Context,
	templateID, slotID, slotPositionID int64,
) error {
	if err := r.resolveTemplateSlotWriteState(ctx, templateID, slotID); err != nil {
		return err
	}

	_, err := getTemplateSlotPositionByID(ctx, r.db, templateID, slotID, slotPositionID)
	switch {
	case errors.Is(err, ErrTemplateSlotPositionNotFound):
		return ErrTemplateSlotPositionNotFound
	case err != nil:
		return err
	default:
		return ErrTemplateSlotPositionNotFound
	}
}

func getTemplateRecordByID(ctx context.Context, db queryer, id int64) (*model.Template, error) {
	const query = `
		SELECT id, name, description, is_locked, created_at, updated_at
		FROM templates
		WHERE id = $1;
	`

	template := &model.Template{}
	err := db.QueryRowContext(ctx, query, id).Scan(
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
