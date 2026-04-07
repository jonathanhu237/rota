package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

var ErrPositionNotFound = errors.New("position not found")

type ListPositionsParams struct {
	Offset int
	Limit  int
}

type CreatePositionParams struct {
	Name        string
	Description string
}

type UpdatePositionParams struct {
	ID          int64
	Name        string
	Description string
}

type PositionRepository struct {
	db *sql.DB
}

func NewPositionRepository(db *sql.DB) *PositionRepository {
	return &PositionRepository{db: db}
}

func (r *PositionRepository) ListPaginated(ctx context.Context, params ListPositionsParams) ([]*model.Position, int, error) {
	const countQuery = `
		SELECT COUNT(*)
		FROM positions;
	`

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	const query = `
		SELECT id, name, description, created_at, updated_at
		FROM positions
		ORDER BY id ASC
		LIMIT $1 OFFSET $2;
	`

	limit := params.Limit
	if limit <= 0 {
		limit = total
		if limit == 0 {
			limit = 1
		}
	}

	rows, err := r.db.QueryContext(ctx, query, limit, params.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	positions := make([]*model.Position, 0)
	for rows.Next() {
		position := &model.Position{}
		if err := rows.Scan(
			&position.ID,
			&position.Name,
			&position.Description,
			&position.CreatedAt,
			&position.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		positions = append(positions, position)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return positions, total, nil
}

func (r *PositionRepository) GetByID(ctx context.Context, id int64) (*model.Position, error) {
	const query = `
		SELECT id, name, description, created_at, updated_at
		FROM positions
		WHERE id = $1;
	`

	position := &model.Position{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&position.ID,
		&position.Name,
		&position.Description,
		&position.CreatedAt,
		&position.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPositionNotFound
	}
	if err != nil {
		return nil, err
	}

	return position, nil
}

func (r *PositionRepository) Create(ctx context.Context, params CreatePositionParams) (*model.Position, error) {
	const query = `
		INSERT INTO positions (name, description)
		VALUES ($1, $2)
		RETURNING id, name, description, created_at, updated_at;
	`

	position := &model.Position{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.Name,
		params.Description,
	).Scan(
		&position.ID,
		&position.Name,
		&position.Description,
		&position.CreatedAt,
		&position.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return position, nil
}

func (r *PositionRepository) Update(ctx context.Context, params UpdatePositionParams) (*model.Position, error) {
	const query = `
		UPDATE positions
		SET name = $2, description = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, description, created_at, updated_at;
	`

	position := &model.Position{}
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.ID,
		params.Name,
		params.Description,
	).Scan(
		&position.ID,
		&position.Name,
		&position.Description,
		&position.CreatedAt,
		&position.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPositionNotFound
	}
	if err != nil {
		return nil, err
	}

	return position, nil
}

func (r *PositionRepository) Delete(ctx context.Context, id int64) error {
	const query = `
		DELETE FROM positions
		WHERE id = $1;
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
		return ErrPositionNotFound
	}

	return nil
}
