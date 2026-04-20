//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

func TestSetupTokenRepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("Create and GetByTokenHash round-trip", func(t *testing.T) {
		db := openIntegrationDB(t)
		user := seedUser(t, db, userSeed{})
		repo := NewSetupTokenRepository(db)
		expiresAt := testTime().Add(72 * time.Hour)

		created, err := repo.Create(ctx, CreateSetupTokenParams{
			UserID:    user.ID,
			TokenHash: "token-hash-1",
			Purpose:   model.SetupTokenPurposeInvitation,
			ExpiresAt: expiresAt,
		})
		if err != nil {
			t.Fatalf("create setup token: %v", err)
		}
		if created.UserID != user.ID {
			t.Fatalf("expected user ID %d, got %d", user.ID, created.UserID)
		}
		if created.Purpose != model.SetupTokenPurposeInvitation {
			t.Fatalf("expected purpose %q, got %q", model.SetupTokenPurposeInvitation, created.Purpose)
		}

		found, err := repo.GetByTokenHash(ctx, "token-hash-1")
		if err != nil {
			t.Fatalf("get setup token by hash: %v", err)
		}
		if found.ID != created.ID {
			t.Fatalf("expected token ID %d, got %d", created.ID, found.ID)
		}
		if !found.ExpiresAt.Equal(expiresAt) {
			t.Fatalf("expected expiry %s, got %s", expiresAt, found.ExpiresAt)
		}
		if found.UsedAt != nil {
			t.Fatalf("expected unused token, got used_at=%v", *found.UsedAt)
		}
	})

	t.Run("Create maps duplicate token hash", func(t *testing.T) {
		db := openIntegrationDB(t)
		user := seedUser(t, db, userSeed{})
		repo := NewSetupTokenRepository(db)

		_, err := repo.Create(ctx, CreateSetupTokenParams{
			UserID:    user.ID,
			TokenHash: "duplicate-token-hash",
			Purpose:   model.SetupTokenPurposeInvitation,
			ExpiresAt: testTime().Add(72 * time.Hour),
		})
		if err != nil {
			t.Fatalf("create setup token: %v", err)
		}

		_, err = repo.Create(ctx, CreateSetupTokenParams{
			UserID:    user.ID,
			TokenHash: "duplicate-token-hash",
			Purpose:   model.SetupTokenPurposePasswordReset,
			ExpiresAt: testTime().Add(time.Hour),
		})
		if !errors.Is(err, ErrSetupTokenAlreadyExists) {
			t.Fatalf("expected ErrSetupTokenAlreadyExists, got %v", err)
		}
	})

	t.Run("Cascade delete removes setup tokens with parent user", func(t *testing.T) {
		db := openIntegrationDB(t)
		user := seedUser(t, db, userSeed{})
		repo := NewSetupTokenRepository(db)

		_, err := repo.Create(ctx, CreateSetupTokenParams{
			UserID:    user.ID,
			TokenHash: "cascade-token-hash",
			Purpose:   model.SetupTokenPurposeInvitation,
			ExpiresAt: testTime().Add(72 * time.Hour),
		})
		if err != nil {
			t.Fatalf("create setup token: %v", err)
		}

		if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = $1;`, user.ID); err != nil {
			t.Fatalf("delete user: %v", err)
		}

		_, err = repo.GetByTokenHash(ctx, "cascade-token-hash")
		if !errors.Is(err, model.ErrTokenNotFound) {
			t.Fatalf("expected ErrTokenNotFound, got %v", err)
		}
	})

	t.Run("InvalidateUnusedTokens marks only matching unused tokens", func(t *testing.T) {
		db := openIntegrationDB(t)
		user := seedUser(t, db, userSeed{})
		repo := NewSetupTokenRepository(db)
		usedAt := testTime()

		invalidationTarget, err := repo.Create(ctx, CreateSetupTokenParams{
			UserID:    user.ID,
			TokenHash: "token-hash-invalidate-target",
			Purpose:   model.SetupTokenPurposeInvitation,
			ExpiresAt: testTime().Add(72 * time.Hour),
		})
		if err != nil {
			t.Fatalf("create invalidation target: %v", err)
		}

		otherPurpose, err := repo.Create(ctx, CreateSetupTokenParams{
			UserID:    user.ID,
			TokenHash: "token-hash-other-purpose",
			Purpose:   model.SetupTokenPurposePasswordReset,
			ExpiresAt: testTime().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("create other purpose token: %v", err)
		}

		alreadyUsed, err := repo.Create(ctx, CreateSetupTokenParams{
			UserID:    user.ID,
			TokenHash: "token-hash-already-used",
			Purpose:   model.SetupTokenPurposeInvitation,
			ExpiresAt: testTime().Add(72 * time.Hour),
		})
		if err != nil {
			t.Fatalf("create used token: %v", err)
		}
		if err := repo.MarkUsed(ctx, alreadyUsed.ID, usedAt.Add(-time.Minute)); err != nil {
			t.Fatalf("mark token used: %v", err)
		}

		if err := repo.InvalidateUnusedTokens(ctx, user.ID, model.SetupTokenPurposeInvitation, usedAt); err != nil {
			t.Fatalf("invalidate unused invitation tokens: %v", err)
		}

		targetAfter, err := repo.GetByTokenHash(ctx, "token-hash-invalidate-target")
		if err != nil {
			t.Fatalf("get invalidated token: %v", err)
		}
		if targetAfter.ID != invalidationTarget.ID {
			t.Fatalf("expected token ID %d, got %d", invalidationTarget.ID, targetAfter.ID)
		}
		if targetAfter.UsedAt == nil || !targetAfter.UsedAt.Equal(usedAt) {
			t.Fatalf("expected used_at=%v, got %v", usedAt, targetAfter.UsedAt)
		}

		otherPurposeAfter, err := repo.GetByTokenHash(ctx, "token-hash-other-purpose")
		if err != nil {
			t.Fatalf("get other purpose token: %v", err)
		}
		if otherPurposeAfter.ID != otherPurpose.ID {
			t.Fatalf("expected token ID %d, got %d", otherPurpose.ID, otherPurposeAfter.ID)
		}
		if otherPurposeAfter.UsedAt != nil {
			t.Fatalf("expected other purpose token to remain unused, got %v", otherPurposeAfter.UsedAt)
		}

		alreadyUsedAfter, err := repo.GetByTokenHash(ctx, "token-hash-already-used")
		if err != nil {
			t.Fatalf("get already used token: %v", err)
		}
		if alreadyUsedAfter.UsedAt == nil || !alreadyUsedAfter.UsedAt.Equal(usedAt.Add(-time.Minute)) {
			t.Fatalf("expected already-used token timestamp to remain unchanged, got %v", alreadyUsedAfter.UsedAt)
		}
	})
}
