//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
)

func TestBrandingRepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("upserts default and updates with version", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewBrandingRepository(db)

		defaults, err := repo.UpsertDefault(ctx)
		if err != nil {
			t.Fatalf("upsert default branding: %v", err)
		}
		if defaults.ProductName != "Rota" || defaults.OrganizationName != "" || defaults.Version != 1 {
			t.Fatalf("unexpected defaults: %+v", defaults)
		}

		updated, err := repo.Update(ctx, UpdateBrandingParams{
			ProductName:      "排班系统",
			OrganizationName: "Acme",
			Version:          defaults.Version,
		})
		if err != nil {
			t.Fatalf("update branding: %v", err)
		}
		if updated.ProductName != "排班系统" ||
			updated.OrganizationName != "Acme" ||
			updated.Version != defaults.Version+1 {
			t.Fatalf("unexpected update: %+v", updated)
		}

		loaded, err := repo.Get(ctx)
		if err != nil {
			t.Fatalf("get branding: %v", err)
		}
		if loaded.ProductName != updated.ProductName || loaded.Version != updated.Version {
			t.Fatalf("loaded branding does not match updated: loaded=%+v updated=%+v", loaded, updated)
		}
	})

	t.Run("stale version is a conflict", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewBrandingRepository(db)
		defaults, err := repo.UpsertDefault(ctx)
		if err != nil {
			t.Fatalf("upsert default branding: %v", err)
		}

		_, err = repo.Update(ctx, UpdateBrandingParams{
			ProductName:      "Rota",
			OrganizationName: "",
			Version:          defaults.Version + 1,
		})
		if !errors.Is(err, ErrVersionConflict) {
			t.Fatalf("expected ErrVersionConflict, got %v", err)
		}
	})

	t.Run("update inserts missing singleton from fallback version", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewBrandingRepository(db)

		defaults, err := repo.Get(ctx)
		if err != nil {
			t.Fatalf("get fallback branding: %v", err)
		}

		updated, err := repo.Update(ctx, UpdateBrandingParams{
			ProductName:      "排班系统",
			OrganizationName: "Acme",
			Version:          defaults.Version,
		})
		if err != nil {
			t.Fatalf("update missing branding row: %v", err)
		}
		if updated.ProductName != "排班系统" ||
			updated.OrganizationName != "Acme" ||
			updated.Version != defaults.Version+1 {
			t.Fatalf("unexpected inserted update: %+v", updated)
		}
	})

	t.Run("missing singleton rejects stale fallback version", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewBrandingRepository(db)

		_, err := repo.Update(ctx, UpdateBrandingParams{
			ProductName:      "排班系统",
			OrganizationName: "Acme",
			Version:          2,
		})
		if !errors.Is(err, ErrVersionConflict) {
			t.Fatalf("expected ErrVersionConflict, got %v", err)
		}
	})

	t.Run("database rejects untrimmed or blank product names", func(t *testing.T) {
		db := openIntegrationDB(t)

		_, err := db.ExecContext(
			ctx,
			`INSERT INTO app_branding (id, product_name, organization_name) VALUES (1, $1, $2);`,
			"   ",
			"",
		)
		if err == nil {
			t.Fatalf("expected blank product name to violate database check")
		}

		_, err = db.ExecContext(
			ctx,
			`INSERT INTO app_branding (id, product_name, organization_name) VALUES (1, $1, $2);`,
			" Rota ",
			"",
		)
		if err == nil {
			t.Fatalf("expected untrimmed product name to violate database check")
		}
	})
}
