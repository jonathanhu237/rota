package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type bootstrapUserRepositoryMock struct {
	countAdminsFunc func(ctx context.Context) (int, error)
	createFunc      func(ctx context.Context, params repository.CreateUserParams) (*model.User, error)
}

func (m *bootstrapUserRepositoryMock) CountAdmins(ctx context.Context) (int, error) {
	return m.countAdminsFunc(ctx)
}

func (m *bootstrapUserRepositoryMock) Create(ctx context.Context, params repository.CreateUserParams) (*model.User, error) {
	return m.createFunc(ctx, params)
}

func TestEnsureBootstrapAdmin(t *testing.T) {
	t.Run("skips when admins already exist", func(t *testing.T) {
		t.Parallel()

		createCalled := false
		err := EnsureBootstrapAdmin(context.Background(), BootstrapAdminInput{
			Email:    "admin@example.com",
			Password: "pa55word",
			Name:     "Administrator",
		}, &bootstrapUserRepositoryMock{
			countAdminsFunc: func(ctx context.Context) (int, error) {
				return 1, nil
			},
			createFunc: func(ctx context.Context, params repository.CreateUserParams) (*model.User, error) {
				createCalled = true
				return nil, nil
			},
		})
		if err != nil {
			t.Fatalf("EnsureBootstrapAdmin returned error: %v", err)
		}
		if createCalled {
			t.Fatalf("Create should not be called when an admin already exists")
		}
	})

	t.Run("creates admin when no admins exist", func(t *testing.T) {
		t.Parallel()

		const password = "pa55word"

		err := EnsureBootstrapAdmin(context.Background(), BootstrapAdminInput{
			Email:    "admin@example.com",
			Password: password,
			Name:     "Administrator",
		}, &bootstrapUserRepositoryMock{
			countAdminsFunc: func(ctx context.Context) (int, error) {
				return 0, nil
			},
			createFunc: func(ctx context.Context, params repository.CreateUserParams) (*model.User, error) {
				if params.Email != "admin@example.com" {
					t.Fatalf("expected email to be passed through, got %q", params.Email)
				}
				if params.Name != "Administrator" {
					t.Fatalf("expected name to be passed through, got %q", params.Name)
				}
				if !params.IsAdmin {
					t.Fatalf("expected bootstrap admin to be created as admin")
				}
				if params.Status != model.UserStatusActive {
					t.Fatalf("expected status %q, got %q", model.UserStatusActive, params.Status)
				}
				if params.PasswordHash == nil {
					t.Fatalf("expected bootstrap admin password hash to be set")
				}
				if err := bcrypt.CompareHashAndPassword([]byte(*params.PasswordHash), []byte(password)); err != nil {
					t.Fatalf("expected password hash to match input password: %v", err)
				}
				return &model.User{ID: 1}, nil
			},
		})
		if err != nil {
			t.Fatalf("EnsureBootstrapAdmin returned error: %v", err)
		}
	})

	t.Run("returns ErrConfigInvalid when email password or name is missing", func(t *testing.T) {
		t.Parallel()

		testCases := []BootstrapAdminInput{
			{Password: "pa55word", Name: "Administrator"},
			{Email: "admin@example.com", Name: "Administrator"},
			{Email: "admin@example.com", Password: "pa55word"},
		}

		for _, input := range testCases {
			err := EnsureBootstrapAdmin(context.Background(), input, &bootstrapUserRepositoryMock{
				countAdminsFunc: func(ctx context.Context) (int, error) {
					return 0, nil
				},
				createFunc: func(ctx context.Context, params repository.CreateUserParams) (*model.User, error) {
					t.Fatalf("Create should not be called when config is invalid")
					return nil, nil
				},
			})
			if !errors.Is(err, ErrConfigInvalid) {
				t.Fatalf("expected ErrConfigInvalid for input %+v, got %v", input, err)
			}
		}
	})

	t.Run("returns ErrConfigInvalid when password is too short", func(t *testing.T) {
		t.Parallel()

		err := EnsureBootstrapAdmin(context.Background(), BootstrapAdminInput{
			Email:    "admin@example.com",
			Password: "short",
			Name:     "Administrator",
		}, &bootstrapUserRepositoryMock{
			countAdminsFunc: func(ctx context.Context) (int, error) {
				return 0, nil
			},
			createFunc: func(ctx context.Context, params repository.CreateUserParams) (*model.User, error) {
				t.Fatalf("Create should not be called when password is too short")
				return nil, nil
			},
		})
		if !errors.Is(err, ErrConfigInvalid) {
			t.Fatalf("expected ErrConfigInvalid, got %v", err)
		}
	})
}
