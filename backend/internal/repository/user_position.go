package repository

import (
	"context"
	"database/sql"
	"sort"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/lib/pq"
)

type UserPositionRepository struct {
	db *sql.DB
}

func NewUserPositionRepository(db *sql.DB) *UserPositionRepository {
	return &UserPositionRepository{db: db}
}

func (r *UserPositionRepository) ListPositionsByUserID(ctx context.Context, userID int64) ([]*model.Position, error) {
	exists, err := r.userExists(ctx, r.db, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrUserNotFound
	}

	const query = `
		SELECT p.id, p.name, p.description, p.created_at, p.updated_at
		FROM positions p
		INNER JOIN user_positions up ON up.position_id = p.id
		WHERE up.user_id = $1
		ORDER BY p.id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		positions = append(positions, position)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return positions, nil
}

func (r *UserPositionRepository) ReplacePositionsByUserID(ctx context.Context, userID int64, positionIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	exists, err := r.userExists(ctx, tx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrUserNotFound
	}

	uniquePositionIDs := uniqueSortedPositionIDs(positionIDs)
	if len(uniquePositionIDs) > 0 {
		const query = `
			SELECT id
			FROM positions
			WHERE id = ANY($1);
		`

		rows, err := tx.QueryContext(ctx, query, pq.Array(uniquePositionIDs))
		if err != nil {
			return err
		}

		foundPositionIDs := make(map[int64]struct{}, len(uniquePositionIDs))
		for rows.Next() {
			var positionID int64
			if err := rows.Scan(&positionID); err != nil {
				rows.Close()
				return err
			}
			foundPositionIDs[positionID] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}

		if len(foundPositionIDs) != len(uniquePositionIDs) {
			return ErrPositionNotFound
		}
	}

	const deleteQuery = `
		DELETE FROM user_positions
		WHERE user_id = $1;
	`

	if _, err := tx.ExecContext(ctx, deleteQuery, userID); err != nil {
		return err
	}

	if len(uniquePositionIDs) > 0 {
		const insertQuery = `
			INSERT INTO user_positions (user_id, position_id)
			SELECT $1, UNNEST($2::BIGINT[]);
		`

		if _, err := tx.ExecContext(ctx, insertQuery, userID, pq.Array(uniquePositionIDs)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (r *UserPositionRepository) userExists(ctx context.Context, rower queryRower, userID int64) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM users
			WHERE id = $1
		);
	`

	var exists bool
	if err := rower.QueryRowContext(ctx, query, userID).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func uniqueSortedPositionIDs(positionIDs []int64) []int64 {
	seen := make(map[int64]struct{}, len(positionIDs))
	unique := make([]int64, 0, len(positionIDs))
	for _, positionID := range positionIDs {
		if _, ok := seen[positionID]; ok {
			continue
		}
		seen[positionID] = struct{}{}
		unique = append(unique, positionID)
	}

	sort.Slice(unique, func(i, j int) bool {
		return unique[i] < unique[j]
	})

	return unique
}
