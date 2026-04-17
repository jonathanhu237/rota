//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

func TestUserRepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("Create GetByEmail GetByID and ListPaginated round-trip", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewUserRepository(db)

		created, err := repo.Create(ctx, CreateUserParams{
			Email:        "worker@example.com",
			PasswordHash: "hash-1",
			Name:         "Worker",
			IsAdmin:      true,
			Status:       model.UserStatusActive,
		})
		if err != nil {
			t.Fatalf("create user: %v", err)
		}

		byEmail, err := repo.GetByEmail(ctx, created.Email)
		if err != nil {
			t.Fatalf("get by email: %v", err)
		}
		if byEmail.ID != created.ID {
			t.Fatalf("expected user ID %d, got %d", created.ID, byEmail.ID)
		}

		byID, err := repo.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("get by id: %v", err)
		}
		if byID.Email != created.Email {
			t.Fatalf("expected email %q, got %q", created.Email, byID.Email)
		}

		seedUser(t, db, userSeed{Name: "Other"})

		users, total, err := repo.ListPaginated(ctx, ListUsersParams{Offset: 0, Limit: 1})
		if err != nil {
			t.Fatalf("list paginated: %v", err)
		}
		if total != 2 {
			t.Fatalf("expected total 2, got %d", total)
		}
		if len(users) != 1 {
			t.Fatalf("expected 1 paginated result, got %d", len(users))
		}
		if users[0].ID != created.ID {
			t.Fatalf("expected first user ID %d, got %d", created.ID, users[0].ID)
		}
	})

	t.Run("Create maps duplicate email", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewUserRepository(db)

		existing := seedUser(t, db, userSeed{Email: "duplicate@example.com"})

		_, err := repo.Create(ctx, CreateUserParams{
			Email:        existing.Email,
			PasswordHash: "hash-2",
			Name:         "Duplicate",
			IsAdmin:      false,
			Status:       model.UserStatusActive,
		})
		if !errors.Is(err, ErrEmailAlreadyExists) {
			t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
		}
	})

	t.Run("Update UpdatePassword and UpdateStatus bump version", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewUserRepository(db)

		user := seedUser(t, db, userSeed{
			Email:        "versioned@example.com",
			PasswordHash: "hash-3",
			Name:         "Versioned",
			IsAdmin:      false,
			Status:       model.UserStatusActive,
		})

		updated, err := repo.Update(ctx, UpdateUserParams{
			ID:      user.ID,
			Email:   "updated@example.com",
			Name:    "Updated",
			IsAdmin: true,
			Version: user.Version,
		})
		if err != nil {
			t.Fatalf("update user: %v", err)
		}
		if updated.Version != user.Version+1 {
			t.Fatalf("expected version %d, got %d", user.Version+1, updated.Version)
		}

		withPassword, err := repo.UpdatePassword(ctx, UpdateUserPasswordParams{
			ID:           updated.ID,
			PasswordHash: "hash-4",
			Version:      updated.Version,
		})
		if err != nil {
			t.Fatalf("update password: %v", err)
		}
		if withPassword.PasswordHash != "hash-4" {
			t.Fatalf("expected password hash update, got %q", withPassword.PasswordHash)
		}

		withStatus, err := repo.UpdateStatus(ctx, UpdateUserStatusParams{
			ID:      withPassword.ID,
			Status:  model.UserStatusDisabled,
			Version: withPassword.Version,
		})
		if err != nil {
			t.Fatalf("update status: %v", err)
		}
		if withStatus.Status != model.UserStatusDisabled {
			t.Fatalf("expected disabled status, got %q", withStatus.Status)
		}
	})

	t.Run("Update resolves version conflict and not found", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewUserRepository(db)

		user := seedUser(t, db, userSeed{})

		_, err := repo.Update(ctx, UpdateUserParams{
			ID:      user.ID,
			Email:   "stale@example.com",
			Name:    "Stale",
			IsAdmin: false,
			Version: user.Version + 1,
		})
		if !errors.Is(err, ErrVersionConflict) {
			t.Fatalf("expected ErrVersionConflict, got %v", err)
		}

		_, err = repo.UpdateStatus(ctx, UpdateUserStatusParams{
			ID:      999,
			Status:  model.UserStatusDisabled,
			Version: 1,
		})
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})
}
