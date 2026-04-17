//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
)

func TestUserPositionRepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("ReplacePositionsByUserID fully replaces and deduplicates positions", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewUserPositionRepository(db)

		user := seedUser(t, db, userSeed{})
		first := seedPosition(t, db, positionSeed{Name: "First"})
		second := seedPosition(t, db, positionSeed{Name: "Second"})
		third := seedPosition(t, db, positionSeed{Name: "Third"})
		seedUserPosition(t, db, user.ID, first.ID)

		if err := repo.ReplacePositionsByUserID(ctx, user.ID, []int64{second.ID, second.ID, third.ID}); err != nil {
			t.Fatalf("replace positions: %v", err)
		}

		positions, err := repo.ListPositionsByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("list positions by user: %v", err)
		}
		if len(positions) != 2 {
			t.Fatalf("expected 2 positions, got %d", len(positions))
		}
		if positions[0].ID != second.ID || positions[1].ID != third.ID {
			t.Fatalf("unexpected replacement result: %+v", positions)
		}
	})

	t.Run("ReplacePositionsByUserID maps user and position not found", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewUserPositionRepository(db)

		user := seedUser(t, db, userSeed{})
		position := seedPosition(t, db, positionSeed{})

		err := repo.ReplacePositionsByUserID(ctx, 999, []int64{position.ID})
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}

		err = repo.ReplacePositionsByUserID(ctx, user.ID, []int64{position.ID, 999})
		if !errors.Is(err, ErrPositionNotFound) {
			t.Fatalf("expected ErrPositionNotFound, got %v", err)
		}
	})
}
