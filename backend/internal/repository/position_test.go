//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
)

func TestPositionRepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("Create GetByID Update and ListPaginated round-trip", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPositionRepository(db)

		created, err := repo.Create(ctx, CreatePositionParams{
			Name:        "Front Desk",
			Description: "Receives visitors",
		})
		if err != nil {
			t.Fatalf("create position: %v", err)
		}

		loaded, err := repo.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("get position: %v", err)
		}
		if loaded.Name != created.Name {
			t.Fatalf("expected name %q, got %q", created.Name, loaded.Name)
		}

		updated, err := repo.Update(ctx, UpdatePositionParams{
			ID:          created.ID,
			Name:        "Updated Desk",
			Description: "Updated description",
		})
		if err != nil {
			t.Fatalf("update position: %v", err)
		}
		if updated.Name != "Updated Desk" {
			t.Fatalf("expected updated name, got %q", updated.Name)
		}

		seedPosition(t, db, positionSeed{Name: "Backup Desk"})

		positions, total, err := repo.ListPaginated(ctx, ListPositionsParams{Offset: 0, Limit: 10})
		if err != nil {
			t.Fatalf("list positions: %v", err)
		}
		if total != 2 {
			t.Fatalf("expected total 2, got %d", total)
		}
		if len(positions) != 2 {
			t.Fatalf("expected 2 positions, got %d", len(positions))
		}
	})

	t.Run("Delete maps not found", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPositionRepository(db)

		err := repo.Delete(ctx, 999)
		if !errors.Is(err, ErrPositionNotFound) {
			t.Fatalf("expected ErrPositionNotFound, got %v", err)
		}
	})

	t.Run("Delete maps foreign key violations to ErrPositionInUse", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPositionRepository(db)

		position := seedPosition(t, db, positionSeed{})
		template := seedTemplate(t, db, templateSeed{})
		seedQualifiedShift(t, db, qualifiedShiftSeed{
			TemplateID:        template.ID,
			PositionID:        position.ID,
			RequiredHeadcount: 2,
		})

		err := repo.Delete(ctx, position.ID)
		if !errors.Is(err, ErrPositionInUse) {
			t.Fatalf("expected ErrPositionInUse, got %v", err)
		}
	})
}
