package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"github.com/jonathanhu237/rota/backend/internal/session"
)

type authUserRepositoryMock struct {
	getByIDFunc    func(ctx context.Context, id int64) (*model.User, error)
	getByEmailFunc func(ctx context.Context, email string) (*model.User, error)
}

func (m *authUserRepositoryMock) GetByID(ctx context.Context, id int64) (*model.User, error) {
	return m.getByIDFunc(ctx, id)
}

func (m *authUserRepositoryMock) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return m.getByEmailFunc(ctx, email)
}

type authSessionStoreMock struct {
	createFunc  func(ctx context.Context, userID int64) (string, int64, error)
	getFunc     func(ctx context.Context, sessionID string) (int64, error)
	refreshFunc func(ctx context.Context, sessionID string) (int64, error)
	deleteFunc  func(ctx context.Context, sessionID string) error
}

func (m *authSessionStoreMock) Create(ctx context.Context, userID int64) (string, int64, error) {
	return m.createFunc(ctx, userID)
}

func (m *authSessionStoreMock) Get(ctx context.Context, sessionID string) (int64, error) {
	return m.getFunc(ctx, sessionID)
}

func (m *authSessionStoreMock) Refresh(ctx context.Context, sessionID string) (int64, error) {
	return m.refreshFunc(ctx, sessionID)
}

func (m *authSessionStoreMock) Delete(ctx context.Context, sessionID string) error {
	return m.deleteFunc(ctx, sessionID)
}

func TestAuthServiceLogin(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		const (
			email     = "worker@example.com"
			password  = "pa55word"
			sessionID = "session-123"
			expiresIn = int64(3600)
		)

		user := &model.User{
			ID:           42,
			Email:        email,
			Name:         "Worker",
			PasswordHash: mustHashPassword(t, password),
			Status:       model.UserStatusActive,
		}

		var createdUserID int64
		service := NewAuthService(
			&authUserRepositoryMock{
				getByEmailFunc: func(ctx context.Context, lookupEmail string) (*model.User, error) {
					if lookupEmail != email {
						t.Fatalf("unexpected email lookup: %s", lookupEmail)
					}
					return user, nil
				},
				getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
					t.Fatalf("unexpected GetByID call: %d", id)
					return nil, nil
				},
			},
			&authSessionStoreMock{
				createFunc: func(ctx context.Context, userID int64) (string, int64, error) {
					createdUserID = userID
					return sessionID, expiresIn, nil
				},
				getFunc: func(ctx context.Context, sessionID string) (int64, error) {
					t.Fatalf("unexpected Get call: %s", sessionID)
					return 0, nil
				},
				refreshFunc: func(ctx context.Context, sessionID string) (int64, error) {
					t.Fatalf("unexpected Refresh call: %s", sessionID)
					return 0, nil
				},
				deleteFunc: func(ctx context.Context, sessionID string) error {
					t.Fatalf("unexpected Delete call: %s", sessionID)
					return nil
				},
			},
		)

		result, err := service.Login(context.Background(), email, password)
		if err != nil {
			t.Fatalf("Login returned error: %v", err)
		}
		if result.SessionID != sessionID {
			t.Fatalf("expected session ID %q, got %q", sessionID, result.SessionID)
		}
		if result.ExpiresIn != expiresIn {
			t.Fatalf("expected expiresIn %d, got %d", expiresIn, result.ExpiresIn)
		}
		if result.User != user {
			t.Fatalf("expected returned user pointer to match mock user")
		}
		if createdUserID != user.ID {
			t.Fatalf("expected session to be created for user ID %d, got %d", user.ID, createdUserID)
		}
	})

	t.Run("user not found returns invalid credentials", func(t *testing.T) {
		t.Parallel()

		service := NewAuthService(
			&authUserRepositoryMock{
				getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
					return nil, repository.ErrUserNotFound
				},
				getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
					return nil, nil
				},
			},
			&authSessionStoreMock{},
		)

		_, err := service.Login(context.Background(), "missing@example.com", "pa55word")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("wrong password returns invalid credentials", func(t *testing.T) {
		t.Parallel()

		sessionCreateCalled := false
		service := NewAuthService(
			&authUserRepositoryMock{
				getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
					return &model.User{
						ID:           7,
						Email:        email,
						PasswordHash: mustHashPassword(t, "different-password"),
						Status:       model.UserStatusActive,
					}, nil
				},
				getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
					return nil, nil
				},
			},
			&authSessionStoreMock{
				createFunc: func(ctx context.Context, userID int64) (string, int64, error) {
					sessionCreateCalled = true
					return "", 0, nil
				},
			},
		)

		_, err := service.Login(context.Background(), "worker@example.com", "pa55word")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
		if sessionCreateCalled {
			t.Fatalf("session should not be created when password comparison fails")
		}
	})

	t.Run("disabled user returns ErrUserDisabled", func(t *testing.T) {
		t.Parallel()

		sessionCreateCalled := false
		service := NewAuthService(
			&authUserRepositoryMock{
				getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
					return &model.User{
						ID:           9,
						Email:        email,
						PasswordHash: mustHashPassword(t, "pa55word"),
						Status:       model.UserStatusDisabled,
					}, nil
				},
				getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
					return nil, nil
				},
			},
			&authSessionStoreMock{
				createFunc: func(ctx context.Context, userID int64) (string, int64, error) {
					sessionCreateCalled = true
					return "", 0, nil
				},
			},
		)

		_, err := service.Login(context.Background(), "disabled@example.com", "pa55word")
		if !errors.Is(err, ErrUserDisabled) {
			t.Fatalf("expected ErrUserDisabled, got %v", err)
		}
		if sessionCreateCalled {
			t.Fatalf("session should not be created for a disabled user")
		}
	})

	t.Run("pending user returns ErrUserPending", func(t *testing.T) {
		t.Parallel()

		sessionCreateCalled := false
		service := NewAuthService(
			&authUserRepositoryMock{
				getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
					return &model.User{
						ID:           10,
						Email:        email,
						PasswordHash: "",
						Status:       model.UserStatusPending,
					}, nil
				},
				getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
					return nil, nil
				},
			},
			&authSessionStoreMock{
				createFunc: func(ctx context.Context, userID int64) (string, int64, error) {
					sessionCreateCalled = true
					return "", 0, nil
				},
			},
		)

		_, err := service.Login(context.Background(), "pending@example.com", "pa55word")
		if !errors.Is(err, ErrUserPending) {
			t.Fatalf("expected ErrUserPending, got %v", err)
		}
		if sessionCreateCalled {
			t.Fatalf("session should not be created for a pending user")
		}
	})

	t.Run("session creation error is returned", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("session create failed")
		service := NewAuthService(
			&authUserRepositoryMock{
				getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
					return &model.User{
						ID:           11,
						Email:        email,
						PasswordHash: mustHashPassword(t, "pa55word"),
						Status:       model.UserStatusActive,
					}, nil
				},
				getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
					return nil, nil
				},
			},
			&authSessionStoreMock{
				createFunc: func(ctx context.Context, userID int64) (string, int64, error) {
					return "", 0, expectedErr
				},
			},
		)

		_, err := service.Login(context.Background(), "worker@example.com", "pa55word")
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected session create error, got %v", err)
		}
	})
}

func TestAuthServiceAuthenticate(t *testing.T) {
	t.Run("success refreshes session", func(t *testing.T) {
		t.Parallel()

		const (
			sessionID = "session-abc"
			userID    = int64(25)
			expiresIn = int64(7200)
		)

		refreshCalled := false
		service := NewAuthService(
			&authUserRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
					if id != userID {
						t.Fatalf("expected user ID %d, got %d", userID, id)
					}
					return &model.User{
						ID:     id,
						Email:  "worker@example.com",
						Status: model.UserStatusActive,
					}, nil
				},
				getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
					return nil, nil
				},
			},
			&authSessionStoreMock{
				getFunc: func(ctx context.Context, inputSessionID string) (int64, error) {
					if inputSessionID != sessionID {
						t.Fatalf("expected session ID %q, got %q", sessionID, inputSessionID)
					}
					return userID, nil
				},
				refreshFunc: func(ctx context.Context, inputSessionID string) (int64, error) {
					refreshCalled = true
					if inputSessionID != sessionID {
						t.Fatalf("expected refresh session ID %q, got %q", sessionID, inputSessionID)
					}
					return expiresIn, nil
				},
				createFunc: func(ctx context.Context, userID int64) (string, int64, error) {
					return "", 0, nil
				},
				deleteFunc: func(ctx context.Context, sessionID string) error {
					return nil
				},
			},
		)

		result, err := service.Authenticate(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("Authenticate returned error: %v", err)
		}
		if !refreshCalled {
			t.Fatalf("expected Refresh to be called")
		}
		if result.User.ID != userID {
			t.Fatalf("expected user ID %d, got %d", userID, result.User.ID)
		}
		if result.ExpiresIn != expiresIn {
			t.Fatalf("expected expiresIn %d, got %d", expiresIn, result.ExpiresIn)
		}
	})

	t.Run("session not found returns unauthorized", func(t *testing.T) {
		t.Parallel()

		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{
				getFunc: func(ctx context.Context, sessionID string) (int64, error) {
					return 0, session.ErrSessionNotFound
				},
			},
		)

		_, err := service.Authenticate(context.Background(), "missing-session")
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("refresh session not found returns unauthorized", func(t *testing.T) {
		t.Parallel()

		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{
				getFunc: func(ctx context.Context, sessionID string) (int64, error) {
					return 12, nil
				},
				refreshFunc: func(ctx context.Context, sessionID string) (int64, error) {
					return 0, session.ErrSessionNotFound
				},
			},
		)

		_, err := service.Authenticate(context.Background(), "stale-session")
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("user not found returns unauthorized", func(t *testing.T) {
		t.Parallel()

		service := NewAuthService(
			&authUserRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
					return nil, repository.ErrUserNotFound
				},
			},
			&authSessionStoreMock{
				getFunc: func(ctx context.Context, sessionID string) (int64, error) {
					return 12, nil
				},
				refreshFunc: func(ctx context.Context, sessionID string) (int64, error) {
					return 100, nil
				},
			},
		)

		_, err := service.Authenticate(context.Background(), "session")
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("disabled user returns unauthorized", func(t *testing.T) {
		t.Parallel()

		service := NewAuthService(
			&authUserRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
					return &model.User{ID: id, Status: model.UserStatusDisabled}, nil
				},
			},
			&authSessionStoreMock{
				getFunc: func(ctx context.Context, sessionID string) (int64, error) {
					return 12, nil
				},
				refreshFunc: func(ctx context.Context, sessionID string) (int64, error) {
					return 100, nil
				},
			},
		)

		_, err := service.Authenticate(context.Background(), "session")
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})
}

func TestAuthServiceLogout(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var deletedSessionID string
		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{
				deleteFunc: func(ctx context.Context, sessionID string) error {
					deletedSessionID = sessionID
					return nil
				},
			},
		)

		if err := service.Logout(context.Background(), "session-1"); err != nil {
			t.Fatalf("Logout returned error: %v", err)
		}
		if deletedSessionID != "session-1" {
			t.Fatalf("expected deleted session ID %q, got %q", "session-1", deletedSessionID)
		}
	})

	t.Run("store error passthrough", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("delete failed")
		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{
				deleteFunc: func(ctx context.Context, sessionID string) error {
					return expectedErr
				},
			},
		)

		err := service.Logout(context.Background(), "session-2")
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected delete error, got %v", err)
		}
	})
}
