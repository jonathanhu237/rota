package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
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
	createFunc             func(ctx context.Context, userID int64) (string, int64, error)
	getFunc                func(ctx context.Context, sessionID string) (int64, error)
	refreshFunc            func(ctx context.Context, sessionID string) (int64, error)
	deleteFunc             func(ctx context.Context, sessionID string) error
	deleteUserSessionsFunc func(ctx context.Context, userID int64) error
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

func (m *authSessionStoreMock) DeleteUserSessions(ctx context.Context, userID int64) error {
	if m.deleteUserSessionsFunc == nil {
		return nil
	}
	return m.deleteUserSessionsFunc(ctx, userID)
}

type authPasswordTxRunnerMock struct {
	userRepo    repository.AuthPasswordUserRepository
	sessionRepo repository.AuthPasswordSessionRepository
}

func (m *authPasswordTxRunnerMock) WithinTx(
	ctx context.Context,
	fn func(
		ctx context.Context,
		userRepo repository.AuthPasswordUserRepository,
		sessionRepo repository.AuthPasswordSessionRepository,
	) error,
) error {
	return fn(ctx, m.userRepo, m.sessionRepo)
}

type authPasswordUserRepositoryMock struct {
	getByIDForUpdateFunc func(ctx context.Context, id int64) (*model.User, error)
	updatePasswordFunc   func(ctx context.Context, id int64, passwordHash string) (*model.User, error)
}

func (m *authPasswordUserRepositoryMock) GetByIDForUpdate(ctx context.Context, id int64) (*model.User, error) {
	return m.getByIDForUpdateFunc(ctx, id)
}

func (m *authPasswordUserRepositoryMock) UpdatePasswordByID(ctx context.Context, id int64, passwordHash string) (*model.User, error) {
	return m.updatePasswordFunc(ctx, id, passwordHash)
}

type authPasswordSessionRepositoryMock struct {
	deleteOtherSessionsFunc func(ctx context.Context, userID int64, currentSessionID string) (int, error)
}

func (m *authPasswordSessionRepositoryMock) DeleteOtherSessions(ctx context.Context, userID int64, currentSessionID string) (int, error) {
	return m.deleteOtherSessionsFunc(ctx, userID, currentSessionID)
}

type emailChangeSessionRepositoryMock struct {
	deleteAllSessionsFunc func(ctx context.Context, userID int64) (int, error)
}

func (m *emailChangeSessionRepositoryMock) DeleteAllSessions(ctx context.Context, userID int64) (int, error) {
	return m.deleteAllSessionsFunc(ctx, userID)
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

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		result, err := service.Login(ctx, email, password)
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

		event := stub.FindByAction(audit.ActionAuthLoginSuccess)
		if event == nil {
			t.Fatalf("expected %s audit event, got actions=%v", audit.ActionAuthLoginSuccess, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeUser {
			t.Fatalf("expected target type %q, got %q", audit.TargetTypeUser, event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != user.ID {
			t.Fatalf("expected target ID %d, got %v", user.ID, event.TargetID)
		}
		if event.Metadata["email"] != email {
			t.Fatalf("expected email metadata %q, got %v", email, event.Metadata["email"])
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

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.Login(ctx, "missing@example.com", "pa55word")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
		assertLoginFailure(t, stub, "missing@example.com", "invalid_credentials")
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

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.Login(ctx, "worker@example.com", "pa55word")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
		if sessionCreateCalled {
			t.Fatalf("session should not be created when password comparison fails")
		}
		assertLoginFailure(t, stub, "worker@example.com", "invalid_credentials")
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

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.Login(ctx, "disabled@example.com", "pa55word")
		if !errors.Is(err, ErrUserDisabled) {
			t.Fatalf("expected ErrUserDisabled, got %v", err)
		}
		if sessionCreateCalled {
			t.Fatalf("session should not be created for a disabled user")
		}
		assertLoginFailure(t, stub, "disabled@example.com", "user_disabled")
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

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.Login(ctx, "pending@example.com", "pa55word")
		if !errors.Is(err, ErrUserPending) {
			t.Fatalf("expected ErrUserPending, got %v", err)
		}
		if sessionCreateCalled {
			t.Fatalf("session should not be created for a pending user")
		}
		assertLoginFailure(t, stub, "pending@example.com", "user_pending")
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

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.Login(ctx, "worker@example.com", "pa55word")
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected session create error, got %v", err)
		}
		if events := stub.Events(); len(events) != 0 {
			t.Fatalf("expected no audit events for infra failure, got %v", stub.Actions())
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
					return 0, repository.ErrSessionNotFound
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
					return 0, repository.ErrSessionNotFound
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

		stub := audittest.New()
		ctx := stub.ContextWith(audit.WithActor(context.Background(), 77))

		if err := service.Logout(ctx, "session-1"); err != nil {
			t.Fatalf("Logout returned error: %v", err)
		}
		if deletedSessionID != "session-1" {
			t.Fatalf("expected deleted session ID %q, got %q", "session-1", deletedSessionID)
		}

		event := stub.FindByAction(audit.ActionAuthLogout)
		if event == nil {
			t.Fatalf("expected %s audit event, got actions=%v", audit.ActionAuthLogout, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeUser {
			t.Fatalf("expected target type %q, got %q", audit.TargetTypeUser, event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 77 {
			t.Fatalf("expected target ID 77, got %v", event.TargetID)
		}
	})

	t.Run("success without actor records only the action", func(t *testing.T) {
		t.Parallel()

		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{
				deleteFunc: func(ctx context.Context, sessionID string) error {
					return nil
				},
			},
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		if err := service.Logout(ctx, "session-3"); err != nil {
			t.Fatalf("Logout returned error: %v", err)
		}

		event := stub.FindByAction(audit.ActionAuthLogout)
		if event == nil {
			t.Fatalf("expected %s audit event, got actions=%v", audit.ActionAuthLogout, stub.Actions())
		}
		if event.TargetType != "" {
			t.Fatalf("expected empty target type, got %q", event.TargetType)
		}
		if event.TargetID != nil {
			t.Fatalf("expected nil target ID, got %v", *event.TargetID)
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

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		err := service.Logout(ctx, "session-2")
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected delete error, got %v", err)
		}
		if events := stub.Events(); len(events) != 0 {
			t.Fatalf("expected no audit events on delete failure, got %v", stub.Actions())
		}
	})
}

func TestAuthServiceSetupPasswordPropagatesTokenUsed(t *testing.T) {
	t.Parallel()

	rawToken := validSetupToken(10)
	now := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	txUserRepo := &setupUserRepositoryMock{
		setPasswordAndStatusFunc: func(ctx context.Context, params repository.SetUserPasswordParams) (*model.User, error) {
			t.Fatalf("SetPasswordAndStatus should not be called after ErrTokenUsed")
			return nil, nil
		},
	}
	txTokenRepo := &setupTokenRepositoryMock{
		getByTokenHashFunc: func(ctx context.Context, tokenHash string) (*model.SetupToken, error) {
			return &model.SetupToken{
				ID:        77,
				UserID:    51,
				Purpose:   model.SetupTokenPurposeInvitation,
				ExpiresAt: now.Add(time.Hour),
			}, nil
		},
		markUsedFunc: func(ctx context.Context, id int64, usedAt time.Time) error {
			if id != 77 {
				t.Fatalf("expected token ID 77, got %d", id)
			}
			return model.ErrTokenUsed
		},
		invalidateAllUnusedFunc: func(ctx context.Context, userID int64, usedAt time.Time) error {
			t.Fatalf("InvalidateAllUnusedTokens should not be called after ErrTokenUsed")
			return nil
		},
	}

	service := NewAuthService(
		&authUserRepositoryMock{},
		&authSessionStoreMock{},
		WithAuthSetupFlows(SetupFlowConfig{
			TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, txTokenRepo)},
			Clock:     func() time.Time { return now },
		}),
	)
	stub := audittest.New()
	ctx := stub.ContextWith(context.Background())

	err := service.SetupPassword(ctx, SetupPasswordInput{
		Token:    rawToken,
		Password: "pa55word",
	})
	if !errors.Is(err, model.ErrTokenUsed) {
		t.Fatalf("expected ErrTokenUsed, got %v", err)
	}
	if len(stub.Events()) != 0 {
		t.Fatalf("expected no audit events after token-used failure, got %v", stub.Actions())
	}
}

func TestAuthServiceSetupPasswordResetClearsSessions(t *testing.T) {
	t.Parallel()

	rawToken := validSetupToken(20)
	now := time.Date(2026, 4, 25, 11, 0, 0, 0, time.UTC)
	const userID = int64(61)
	txUserRepo := &setupUserRepositoryMock{
		setPasswordAndStatusFunc: func(ctx context.Context, params repository.SetUserPasswordParams) (*model.User, error) {
			return &model.User{ID: params.ID, Status: params.Status}, nil
		},
	}
	txTokenRepo := &setupTokenRepositoryMock{
		getByTokenHashFunc: func(ctx context.Context, tokenHash string) (*model.SetupToken, error) {
			return &model.SetupToken{
				ID:        88,
				UserID:    userID,
				Purpose:   model.SetupTokenPurposePasswordReset,
				ExpiresAt: now.Add(time.Hour),
			}, nil
		},
		markUsedFunc: func(ctx context.Context, id int64, usedAt time.Time) error {
			return nil
		},
		invalidateAllUnusedFunc: func(ctx context.Context, _ int64, _ time.Time) error {
			return nil
		},
	}

	deleteCalls := 0
	service := NewAuthService(
		&authUserRepositoryMock{},
		&authSessionStoreMock{
			deleteUserSessionsFunc: func(ctx context.Context, gotUserID int64) error {
				if gotUserID != userID {
					t.Fatalf("expected DeleteUserSessions for user %d, got %d", userID, gotUserID)
				}
				deleteCalls++
				return nil
			},
		},
		WithAuthSetupFlows(SetupFlowConfig{
			TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, txTokenRepo)},
			Clock:     func() time.Time { return now },
		}),
	)
	stub := audittest.New()
	ctx := stub.ContextWith(context.Background())

	if err := service.SetupPassword(ctx, SetupPasswordInput{
		Token:    rawToken,
		Password: "pa55word!",
	}); err != nil {
		t.Fatalf("SetupPassword returned error: %v", err)
	}

	if deleteCalls != 1 {
		t.Fatalf("expected DeleteUserSessions to be called once for password_reset, got %d", deleteCalls)
	}

	event := stub.FindByAction(audit.ActionAuthPasswordSet)
	if event == nil {
		t.Fatalf("expected %s audit event, got actions=%v", audit.ActionAuthPasswordSet, stub.Actions())
	}
	if event.Metadata["purpose"] != "password_reset" {
		t.Fatalf("expected purpose=password_reset, got %v", event.Metadata["purpose"])
	}
}

func TestAuthServiceSetupPasswordInvitationDoesNotClearSessions(t *testing.T) {
	t.Parallel()

	rawToken := validSetupToken(21)
	now := time.Date(2026, 4, 25, 11, 30, 0, 0, time.UTC)
	const userID = int64(62)
	txUserRepo := &setupUserRepositoryMock{
		setPasswordAndStatusFunc: func(ctx context.Context, params repository.SetUserPasswordParams) (*model.User, error) {
			return &model.User{ID: params.ID, Status: params.Status}, nil
		},
	}
	txTokenRepo := &setupTokenRepositoryMock{
		getByTokenHashFunc: func(ctx context.Context, tokenHash string) (*model.SetupToken, error) {
			return &model.SetupToken{
				ID:        89,
				UserID:    userID,
				Purpose:   model.SetupTokenPurposeInvitation,
				ExpiresAt: now.Add(time.Hour),
			}, nil
		},
		markUsedFunc: func(ctx context.Context, id int64, usedAt time.Time) error {
			return nil
		},
		invalidateAllUnusedFunc: func(ctx context.Context, _ int64, _ time.Time) error {
			return nil
		},
	}

	service := NewAuthService(
		&authUserRepositoryMock{},
		&authSessionStoreMock{
			deleteUserSessionsFunc: func(ctx context.Context, gotUserID int64) error {
				t.Fatalf("DeleteUserSessions must not be called for invitation purpose, got userID=%d", gotUserID)
				return nil
			},
		},
		WithAuthSetupFlows(SetupFlowConfig{
			TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, txTokenRepo)},
			Clock:     func() time.Time { return now },
		}),
	)

	if err := service.SetupPassword(context.Background(), SetupPasswordInput{
		Token:    rawToken,
		Password: "pa55word!",
	}); err != nil {
		t.Fatalf("SetupPassword returned error: %v", err)
	}
}

func TestAuthServiceSetupPasswordLength(t *testing.T) {
	t.Run("rejects empty password", func(t *testing.T) {
		t.Parallel()
		assertSetupPasswordLengthError(t, "")
	})

	t.Run("rejects seven code points", func(t *testing.T) {
		t.Parallel()
		assertSetupPasswordLengthError(t, "1234567")
	})

	t.Run("accepts eight code points", func(t *testing.T) {
		t.Parallel()

		rawToken := validSetupToken(11)
		now := time.Date(2026, 4, 25, 10, 30, 0, 0, time.UTC)
		txUserRepo := &setupUserRepositoryMock{
			setPasswordAndStatusFunc: func(ctx context.Context, params repository.SetUserPasswordParams) (*model.User, error) {
				if params.PasswordHash == "" {
					t.Fatalf("expected hashed password")
				}
				if params.Status != model.UserStatusActive {
					t.Fatalf("expected active status, got %q", params.Status)
				}
				return &model.User{ID: params.ID, Status: params.Status}, nil
			},
		}
		txTokenRepo := &setupTokenRepositoryMock{
			getByTokenHashFunc: func(ctx context.Context, tokenHash string) (*model.SetupToken, error) {
				return &model.SetupToken{
					ID:        78,
					UserID:    52,
					Purpose:   model.SetupTokenPurposeInvitation,
					ExpiresAt: now.Add(time.Hour),
				}, nil
			},
			markUsedFunc: func(ctx context.Context, id int64, usedAt time.Time) error {
				if id != 78 {
					t.Fatalf("expected token ID 78, got %d", id)
				}
				return nil
			},
			invalidateAllUnusedFunc: func(ctx context.Context, userID int64, usedAt time.Time) error {
				if userID != 52 {
					t.Fatalf("expected user ID 52, got %d", userID)
				}
				return nil
			},
		}
		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{},
			WithAuthSetupFlows(SetupFlowConfig{
				TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, txTokenRepo)},
				Clock:     func() time.Time { return now },
			}),
		)

		if err := service.SetupPassword(context.Background(), SetupPasswordInput{
			Token:    rawToken,
			Password: "12345678",
		}); err != nil {
			t.Fatalf("SetupPassword returned error: %v", err)
		}
	})

	t.Run("rejects six multibyte code points", func(t *testing.T) {
		t.Parallel()
		assertSetupPasswordLengthError(t, "你好世界你好")
	})
}

func TestAuthServiceChangeOwnPassword(t *testing.T) {
	t.Run("wrong current password is rejected", func(t *testing.T) {
		t.Parallel()

		deleteCalled := false
		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{},
			WithAuthPasswordTxRunner(&authPasswordTxRunnerMock{
				userRepo: &authPasswordUserRepositoryMock{
					getByIDForUpdateFunc: func(ctx context.Context, id int64) (*model.User, error) {
						return &model.User{
							ID:           id,
							PasswordHash: mustHashPassword(t, "correct-password"),
						}, nil
					},
					updatePasswordFunc: func(ctx context.Context, id int64, passwordHash string) (*model.User, error) {
						t.Fatalf("password should not be updated")
						return nil, nil
					},
				},
				sessionRepo: &authPasswordSessionRepositoryMock{
					deleteOtherSessionsFunc: func(ctx context.Context, userID int64, currentSessionID string) (int, error) {
						deleteCalled = true
						return 0, nil
					},
				},
			}),
		)

		_, err := service.ChangeOwnPassword(context.Background(), 7, "session-a", "wrong-password", "new-password")
		if !errors.Is(err, ErrInvalidCurrentPassword) {
			t.Fatalf("expected ErrInvalidCurrentPassword, got %v", err)
		}
		if deleteCalled {
			t.Fatalf("other sessions should not be deleted on wrong current password")
		}
	})

	t.Run("too short new password is rejected", func(t *testing.T) {
		t.Parallel()

		updateCalled := false
		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{},
			WithAuthPasswordTxRunner(&authPasswordTxRunnerMock{
				userRepo: &authPasswordUserRepositoryMock{
					getByIDForUpdateFunc: func(ctx context.Context, id int64) (*model.User, error) {
						return &model.User{
							ID:           id,
							PasswordHash: mustHashPassword(t, "current-password"),
						}, nil
					},
					updatePasswordFunc: func(ctx context.Context, id int64, passwordHash string) (*model.User, error) {
						updateCalled = true
						return nil, nil
					},
				},
				sessionRepo: &authPasswordSessionRepositoryMock{
					deleteOtherSessionsFunc: func(ctx context.Context, userID int64, currentSessionID string) (int, error) {
						t.Fatalf("other sessions should not be deleted")
						return 0, nil
					},
				},
			}),
		)

		_, err := service.ChangeOwnPassword(context.Background(), 7, "session-a", "current-password", "1234567")
		if !errors.Is(err, model.ErrPasswordTooShort) {
			t.Fatalf("expected ErrPasswordTooShort, got %v", err)
		}
		if updateCalled {
			t.Fatalf("password should not be updated for short new password")
		}
	})

	t.Run("success updates password, revokes other sessions, and emits audit", func(t *testing.T) {
		t.Parallel()

		var updatedHash string
		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{},
			WithAuthPasswordTxRunner(&authPasswordTxRunnerMock{
				userRepo: &authPasswordUserRepositoryMock{
					getByIDForUpdateFunc: func(ctx context.Context, id int64) (*model.User, error) {
						return &model.User{
							ID:           id,
							PasswordHash: mustHashPassword(t, "current-password"),
						}, nil
					},
					updatePasswordFunc: func(ctx context.Context, id int64, passwordHash string) (*model.User, error) {
						if id != 7 {
							t.Fatalf("expected user ID 7, got %d", id)
						}
						updatedHash = passwordHash
						return &model.User{ID: id, PasswordHash: passwordHash}, nil
					},
				},
				sessionRepo: &authPasswordSessionRepositoryMock{
					deleteOtherSessionsFunc: func(ctx context.Context, userID int64, currentSessionID string) (int, error) {
						if userID != 7 || currentSessionID != "session-a" {
							t.Fatalf("unexpected delete other sessions input: user=%d session=%q", userID, currentSessionID)
						}
						return 2, nil
					},
				},
			}),
		)
		stub := audittest.New()
		ctx := audit.WithActor(stub.ContextWith(context.Background()), 7)

		revoked, err := service.ChangeOwnPassword(ctx, 7, "session-a", "current-password", "new-password")
		if err != nil {
			t.Fatalf("ChangeOwnPassword returned error: %v", err)
		}
		if revoked != 2 {
			t.Fatalf("expected revoked count 2, got %d", revoked)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(updatedHash), []byte("new-password")); err != nil {
			t.Fatalf("updated hash does not match new password: %v", err)
		}

		event := stub.FindByAction(audit.ActionAuthPasswordChange)
		if event == nil {
			t.Fatalf("expected password change audit event, got %v", stub.Actions())
		}
		if event.Metadata["revoked_session_count"] != 2 {
			t.Fatalf("unexpected audit metadata: %+v", event.Metadata)
		}
	})
}

func TestAuthServiceConfirmEmailChange(t *testing.T) {
	t.Run("success updates email, revokes sessions, invalidates siblings, and emits audit", func(t *testing.T) {
		t.Parallel()

		rawToken := validSetupToken(30)
		now := time.Date(2026, 4, 26, 9, 0, 0, 0, time.UTC)
		newEmail := "alice2@example.com"
		var markedTokenID int64
		var updatedEmail string
		var invalidatedExceptID int64
		txUserRepo := &setupUserRepositoryMock{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				if email != newEmail {
					t.Fatalf("unexpected email lookup: %q", email)
				}
				return nil, repository.ErrUserNotFound
			},
			updateEmailFunc: func(ctx context.Context, id int64, email string) (*model.User, error) {
				if id != 7 {
					t.Fatalf("expected user ID 7, got %d", id)
				}
				updatedEmail = email
				return &model.User{ID: id, Email: email}, nil
			},
		}
		txTokenRepo := &setupTokenRepositoryMock{
			getByTokenHashAndPurposeFunc: func(ctx context.Context, tokenHash string, purpose model.SetupTokenPurpose) (*model.SetupToken, error) {
				if purpose != model.SetupTokenPurposeEmailChange {
					t.Fatalf("expected email_change purpose, got %q", purpose)
				}
				return &model.SetupToken{
					ID:        55,
					UserID:    7,
					Purpose:   model.SetupTokenPurposeEmailChange,
					NewEmail:  &newEmail,
					ExpiresAt: now.Add(time.Hour),
				}, nil
			},
			markUsedFunc: func(ctx context.Context, id int64, usedAt time.Time) error {
				markedTokenID = id
				if !usedAt.Equal(now) {
					t.Fatalf("expected used_at %s, got %s", now, usedAt)
				}
				return nil
			},
			invalidateAllUnusedTokensExceptFunc: func(ctx context.Context, userID int64, exceptTokenID int64, usedAt time.Time) error {
				if userID != 7 {
					t.Fatalf("expected user ID 7, got %d", userID)
				}
				invalidatedExceptID = exceptTokenID
				return nil
			},
		}

		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{},
			WithAuthSetupFlows(SetupFlowConfig{
				TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, txTokenRepo)},
				Clock:     func() time.Time { return now },
			}),
		)
		service.setupFlows.sessionRepoFactory = func(repository.DBTX) emailChangeSessionRepository {
			return &emailChangeSessionRepositoryMock{
				deleteAllSessionsFunc: func(ctx context.Context, userID int64) (int, error) {
					if userID != 7 {
						t.Fatalf("expected sessions deleted for user 7, got %d", userID)
					}
					return 2, nil
				},
			}
		}
		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		err := service.ConfirmEmailChange(ctx, rawToken)
		if err != nil {
			t.Fatalf("ConfirmEmailChange returned error: %v", err)
		}
		if markedTokenID != 55 || invalidatedExceptID != 55 {
			t.Fatalf("expected token 55 marked/excepted, got mark=%d except=%d", markedTokenID, invalidatedExceptID)
		}
		if updatedEmail != newEmail {
			t.Fatalf("expected email update to %q, got %q", newEmail, updatedEmail)
		}

		event := stub.FindByAction(audit.ActionUserEmailChangeConfirm)
		if event == nil {
			t.Fatalf("expected email change confirm audit event, got %v", stub.Actions())
		}
		if event.Metadata["new_email_normalized"] != newEmail {
			t.Fatalf("unexpected audit metadata: %+v", event.Metadata)
		}
		if event.Metadata["revoked_session_count"] != 2 {
			t.Fatalf("unexpected revoked count metadata: %+v", event.Metadata)
		}
	})

	t.Run("token rejection vocabulary", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
		usedAt := now.Add(-time.Minute)
		newEmail := "alice2@example.com"
		tests := []struct {
			name      string
			rawToken  string
			tokenFunc func(ctx context.Context, tokenHash string, purpose model.SetupTokenPurpose) (*model.SetupToken, error)
			want      error
		}{
			{
				name:     "invalid token shape",
				rawToken: "not-a-token",
				want:     model.ErrInvalidToken,
			},
			{
				name:     "unknown token",
				rawToken: validSetupToken(31),
				tokenFunc: func(ctx context.Context, tokenHash string, purpose model.SetupTokenPurpose) (*model.SetupToken, error) {
					return nil, model.ErrTokenNotFound
				},
				want: model.ErrTokenNotFound,
			},
			{
				name:     "used token",
				rawToken: validSetupToken(32),
				tokenFunc: func(ctx context.Context, tokenHash string, purpose model.SetupTokenPurpose) (*model.SetupToken, error) {
					return &model.SetupToken{ID: 1, UserID: 7, Purpose: purpose, NewEmail: &newEmail, ExpiresAt: now.Add(time.Hour), UsedAt: &usedAt}, nil
				},
				want: model.ErrTokenUsed,
			},
			{
				name:     "expired token",
				rawToken: validSetupToken(33),
				tokenFunc: func(ctx context.Context, tokenHash string, purpose model.SetupTokenPurpose) (*model.SetupToken, error) {
					return &model.SetupToken{ID: 1, UserID: 7, Purpose: purpose, NewEmail: &newEmail, ExpiresAt: now.Add(-time.Second)}, nil
				},
				want: model.ErrTokenExpired,
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				service := NewAuthService(
					&authUserRepositoryMock{},
					&authSessionStoreMock{},
					WithAuthSetupFlows(SetupFlowConfig{
						TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(&setupUserRepositoryMock{}, &setupTokenRepositoryMock{
							getByTokenHashAndPurposeFunc: func(ctx context.Context, tokenHash string, purpose model.SetupTokenPurpose) (*model.SetupToken, error) {
								if tt.tokenFunc == nil {
									t.Fatalf("token repository should not be called")
								}
								return tt.tokenFunc(ctx, tokenHash, purpose)
							},
						})},
						Clock: func() time.Time { return now },
					}),
				)

				err := service.ConfirmEmailChange(context.Background(), tt.rawToken)
				if !errors.Is(err, tt.want) {
					t.Fatalf("expected %v, got %v", tt.want, err)
				}
			})
		}
	})

	t.Run("email collision after CAS consumes token and returns ErrEmailAlreadyExists", func(t *testing.T) {
		t.Parallel()

		rawToken := validSetupToken(34)
		now := time.Date(2026, 4, 26, 11, 0, 0, 0, time.UTC)
		newEmail := "alice2@example.com"
		markUsedCalled := false
		updateCalled := false
		txUserRepo := &setupUserRepositoryMock{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return &model.User{ID: 99, Email: email}, nil
			},
			updateEmailFunc: func(ctx context.Context, id int64, email string) (*model.User, error) {
				updateCalled = true
				return nil, nil
			},
		}
		txTokenRepo := &setupTokenRepositoryMock{
			getByTokenHashAndPurposeFunc: func(ctx context.Context, tokenHash string, purpose model.SetupTokenPurpose) (*model.SetupToken, error) {
				return &model.SetupToken{ID: 66, UserID: 7, Purpose: purpose, NewEmail: &newEmail, ExpiresAt: now.Add(time.Hour)}, nil
			},
			markUsedFunc: func(ctx context.Context, id int64, usedAt time.Time) error {
				markUsedCalled = true
				return nil
			},
		}
		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{},
			WithAuthSetupFlows(SetupFlowConfig{
				TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, txTokenRepo)},
				Clock:     func() time.Time { return now },
			}),
		)

		stub := audittest.New()
		err := service.ConfirmEmailChange(stub.ContextWith(context.Background()), rawToken)
		if !errors.Is(err, ErrEmailAlreadyExists) {
			t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
		}
		if !markUsedCalled {
			t.Fatalf("expected token to be marked used before collision return")
		}
		if updateCalled {
			t.Fatalf("user email should not be updated after collision")
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit event after collision, got %v", stub.Actions())
		}
	})

	t.Run("concurrent CAS loser returns ErrTokenUsed", func(t *testing.T) {
		t.Parallel()

		rawToken := validSetupToken(35)
		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		newEmail := "alice2@example.com"
		updateCalled := false
		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{},
			WithAuthSetupFlows(SetupFlowConfig{
				TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(&setupUserRepositoryMock{
					updateEmailFunc: func(ctx context.Context, id int64, email string) (*model.User, error) {
						updateCalled = true
						return nil, nil
					},
				}, &setupTokenRepositoryMock{
					getByTokenHashAndPurposeFunc: func(ctx context.Context, tokenHash string, purpose model.SetupTokenPurpose) (*model.SetupToken, error) {
						return &model.SetupToken{ID: 77, UserID: 7, Purpose: purpose, NewEmail: &newEmail, ExpiresAt: now.Add(time.Hour)}, nil
					},
					markUsedFunc: func(ctx context.Context, id int64, usedAt time.Time) error {
						return model.ErrTokenUsed
					},
				})},
				Clock: func() time.Time { return now },
			}),
		)

		err := service.ConfirmEmailChange(context.Background(), rawToken)
		if !errors.Is(err, model.ErrTokenUsed) {
			t.Fatalf("expected ErrTokenUsed, got %v", err)
		}
		if updateCalled {
			t.Fatalf("email should not be updated when CAS loses")
		}
	})
}

func TestAuthServiceSetupPasswordRejectsEmailChangeToken(t *testing.T) {
	t.Parallel()

	rawToken := validSetupToken(36)
	now := time.Date(2026, 4, 26, 13, 0, 0, 0, time.UTC)
	newEmail := "alice2@example.com"
	txTokenRepo := &setupTokenRepositoryMock{
		getByTokenHashFunc: func(ctx context.Context, tokenHash string) (*model.SetupToken, error) {
			return &model.SetupToken{
				ID:        88,
				UserID:    7,
				Purpose:   model.SetupTokenPurposeEmailChange,
				NewEmail:  &newEmail,
				ExpiresAt: now.Add(time.Hour),
			}, nil
		},
		markUsedFunc: func(ctx context.Context, id int64, usedAt time.Time) error {
			t.Fatalf("email_change token should not be marked used by setup-password")
			return nil
		},
	}
	service := NewAuthService(
		&authUserRepositoryMock{},
		&authSessionStoreMock{},
		WithAuthSetupFlows(SetupFlowConfig{
			TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(&setupUserRepositoryMock{}, txTokenRepo)},
			Clock:     func() time.Time { return now },
		}),
	)

	err := service.SetupPassword(context.Background(), SetupPasswordInput{
		Token:    rawToken,
		Password: "pa55word",
	})
	if !errors.Is(err, model.ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func assertSetupPasswordLengthError(t *testing.T, password string) {
	t.Helper()

	rawToken := validSetupToken(12)
	service := NewAuthService(
		&authUserRepositoryMock{},
		&authSessionStoreMock{},
		WithAuthSetupFlows(SetupFlowConfig{
			TxManager: &setupTxManagerMock{
				withinTxFunc: withSetupRepos(&setupUserRepositoryMock{}, &setupTokenRepositoryMock{
					getByTokenHashFunc: func(ctx context.Context, tokenHash string) (*model.SetupToken, error) {
						t.Fatalf("token lookup should not run for short password")
						return nil, nil
					},
				}),
			},
		}),
	)

	err := service.SetupPassword(context.Background(), SetupPasswordInput{
		Token:    rawToken,
		Password: password,
	})
	if !errors.Is(err, model.ErrPasswordTooShort) {
		t.Fatalf("expected ErrPasswordTooShort, got %v", err)
	}
}

// assertLoginFailure asserts the stub captured a single login-failure event
// with the expected email and reason metadata, and no other events.
func assertLoginFailure(t *testing.T, stub *audittest.Stub, email, reason string) {
	t.Helper()
	event := stub.FindByAction(audit.ActionAuthLoginFailure)
	if event == nil {
		t.Fatalf("expected %s audit event, got actions=%v", audit.ActionAuthLoginFailure, stub.Actions())
	}
	if event.ActorID != nil {
		t.Fatalf("expected nil actor for login failure, got %v", *event.ActorID)
	}
	if event.Metadata["email"] != email {
		t.Fatalf("expected email metadata %q, got %v", email, event.Metadata["email"])
	}
	if event.Metadata["reason"] != reason {
		t.Fatalf("expected reason metadata %q, got %v", reason, event.Metadata["reason"])
	}
}
