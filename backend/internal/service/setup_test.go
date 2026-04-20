package service

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type setupTokenRepositoryMock struct {
	createFunc                 func(ctx context.Context, params repository.CreateSetupTokenParams) (*model.SetupToken, error)
	getByTokenHashFunc         func(ctx context.Context, tokenHash string) (*model.SetupToken, error)
	invalidateUnusedTokensFunc func(ctx context.Context, userID int64, purpose model.SetupTokenPurpose, usedAt time.Time) error
	invalidateAllUnusedFunc    func(ctx context.Context, userID int64, usedAt time.Time) error
	markUsedFunc               func(ctx context.Context, id int64, usedAt time.Time) error
}

func (m *setupTokenRepositoryMock) Create(ctx context.Context, params repository.CreateSetupTokenParams) (*model.SetupToken, error) {
	return m.createFunc(ctx, params)
}

func (m *setupTokenRepositoryMock) GetByTokenHash(ctx context.Context, tokenHash string) (*model.SetupToken, error) {
	return m.getByTokenHashFunc(ctx, tokenHash)
}

func (m *setupTokenRepositoryMock) InvalidateUnusedTokens(ctx context.Context, userID int64, purpose model.SetupTokenPurpose, usedAt time.Time) error {
	return m.invalidateUnusedTokensFunc(ctx, userID, purpose, usedAt)
}

func (m *setupTokenRepositoryMock) InvalidateAllUnusedTokens(ctx context.Context, userID int64, usedAt time.Time) error {
	return m.invalidateAllUnusedFunc(ctx, userID, usedAt)
}

func (m *setupTokenRepositoryMock) MarkUsed(ctx context.Context, id int64, usedAt time.Time) error {
	return m.markUsedFunc(ctx, id, usedAt)
}

type setupTxManagerMock struct {
	withinTxFunc func(
		ctx context.Context,
		fn func(ctx context.Context, userRepo setupUserRepository, tokenRepo setupTokenRepository) error,
	) error
}

func (m *setupTxManagerMock) WithinTx(
	ctx context.Context,
	fn func(ctx context.Context, userRepo setupUserRepository, tokenRepo setupTokenRepository) error,
) error {
	return m.withinTxFunc(ctx, fn)
}

type emailerMock struct {
	sendFunc func(ctx context.Context, msg email.Message) error
}

func (m *emailerMock) Send(ctx context.Context, msg email.Message) error {
	return m.sendFunc(ctx, msg)
}

func TestUserServiceCreateUserInvitationFlow(t *testing.T) {
	t.Run("creates a pending user, stores an invitation token, and sends an email", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)
		randomBytes := strings.NewReader(strings.Repeat("a", 32))
		txUserRepo := &setupUserRepositoryMock{
			createFunc: func(ctx context.Context, params repository.CreateUserParams) (*model.User, error) {
				if params.PasswordHash != nil {
					t.Fatalf("expected pending users to be created without a password hash")
				}
				if params.Status != model.UserStatusPending {
					t.Fatalf("expected pending status, got %q", params.Status)
				}
				return &model.User{
					ID:      12,
					Email:   params.Email,
					Name:    params.Name,
					IsAdmin: params.IsAdmin,
					Status:  params.Status,
				}, nil
			},
		}
		txTokenRepo := &setupTokenRepositoryMock{
			invalidateUnusedTokensFunc: func(ctx context.Context, userID int64, purpose model.SetupTokenPurpose, usedAt time.Time) error {
				if userID != 12 {
					t.Fatalf("expected user ID 12, got %d", userID)
				}
				if purpose != model.SetupTokenPurposeInvitation {
					t.Fatalf("expected invitation purpose, got %q", purpose)
				}
				if !usedAt.Equal(now) {
					t.Fatalf("expected usedAt %s, got %s", now, usedAt)
				}
				return nil
			},
			createFunc: func(ctx context.Context, params repository.CreateSetupTokenParams) (*model.SetupToken, error) {
				if params.UserID != 12 {
					t.Fatalf("expected token for user 12, got %d", params.UserID)
				}
				if params.Purpose != model.SetupTokenPurposeInvitation {
					t.Fatalf("expected invitation purpose, got %q", params.Purpose)
				}
				if !params.ExpiresAt.Equal(now.Add(72 * time.Hour)) {
					t.Fatalf("expected invitation expiry %s, got %s", now.Add(72*time.Hour), params.ExpiresAt)
				}
				return &model.SetupToken{ID: 7, UserID: 12, TokenHash: params.TokenHash}, nil
			},
		}

		var sent email.Message
		service := NewUserService(
			&userRepositoryMock{
				getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
					return nil, repository.ErrUserNotFound
				},
			},
			&userSessionStoreMock{},
			WithSetupFlows(SetupFlowConfig{
				TxManager:          &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, txTokenRepo)},
				Emailer:            &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error { sent = msg; return nil }},
				Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
				AppBaseURL:         "http://localhost:5173",
				InvitationTokenTTL: 72 * time.Hour,
				Clock:              func() time.Time { return now },
				RandomReader:       randomBytes,
			}),
		)

		user, err := service.CreateUser(context.Background(), CreateUserInput{
			Email:   " worker@example.com ",
			Name:    " Worker ",
			IsAdmin: true,
		})
		if err != nil {
			t.Fatalf("CreateUser returned error: %v", err)
		}
		if user.Status != model.UserStatusPending {
			t.Fatalf("expected pending user, got %+v", user)
		}
		if sent.To != "worker@example.com" {
			t.Fatalf("expected invitation email recipient, got %q", sent.To)
		}
		if !strings.Contains(sent.Body, "/setup-password?token=") {
			t.Fatalf("expected invitation email to contain setup link, got %q", sent.Body)
		}
	})

	t.Run("logs email delivery failures without surfacing them", func(t *testing.T) {
		t.Parallel()

		txUserRepo := &setupUserRepositoryMock{
			createFunc: func(ctx context.Context, params repository.CreateUserParams) (*model.User, error) {
				return &model.User{
					ID:     13,
					Email:  params.Email,
					Name:   params.Name,
					Status: model.UserStatusPending,
				}, nil
			},
		}
		txTokenRepo := &setupTokenRepositoryMock{
			invalidateUnusedTokensFunc: func(ctx context.Context, userID int64, purpose model.SetupTokenPurpose, usedAt time.Time) error {
				return nil
			},
			createFunc: func(ctx context.Context, params repository.CreateSetupTokenParams) (*model.SetupToken, error) {
				return &model.SetupToken{ID: 8, UserID: 13, TokenHash: params.TokenHash}, nil
			},
		}

		service := NewUserService(
			&userRepositoryMock{
				getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
					return nil, repository.ErrUserNotFound
				},
			},
			&userSessionStoreMock{},
			WithSetupFlows(SetupFlowConfig{
				TxManager:          &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, txTokenRepo)},
				Emailer:            &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error { return errors.New("smtp failed") }},
				Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
				AppBaseURL:         "http://localhost:5173",
				InvitationTokenTTL: 72 * time.Hour,
				Clock:              func() time.Time { return time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC) },
				RandomReader:       strings.NewReader(strings.Repeat("b", 32)),
			}),
		)

		user, err := service.CreateUser(context.Background(), CreateUserInput{
			Email: "worker@example.com",
			Name:  "Worker",
		})
		if err != nil {
			t.Fatalf("CreateUser returned error: %v", err)
		}
		if user.Status != model.UserStatusPending {
			t.Fatalf("expected pending user, got %+v", user)
		}
	})
}

func TestUserServiceResendInvitation(t *testing.T) {
	t.Run("reissues an invitation for a pending user", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
		txUserRepo := &setupUserRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return &model.User{
					ID:     id,
					Email:  "pending@example.com",
					Name:   "Pending User",
					Status: model.UserStatusPending,
				}, nil
			},
		}
		txTokenRepo := &setupTokenRepositoryMock{
			invalidateUnusedTokensFunc: func(ctx context.Context, userID int64, purpose model.SetupTokenPurpose, usedAt time.Time) error {
				if userID != 21 || purpose != model.SetupTokenPurposeInvitation {
					t.Fatalf("unexpected invalidate call: userID=%d purpose=%q", userID, purpose)
				}
				return nil
			},
			createFunc: func(ctx context.Context, params repository.CreateSetupTokenParams) (*model.SetupToken, error) {
				return &model.SetupToken{ID: 3, UserID: params.UserID, TokenHash: params.TokenHash}, nil
			},
		}

		sendCount := 0
		service := NewUserService(
			&userRepositoryMock{},
			&userSessionStoreMock{},
			WithSetupFlows(SetupFlowConfig{
				TxManager:          &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, txTokenRepo)},
				Emailer:            &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error { sendCount++; return nil }},
				Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
				AppBaseURL:         "http://localhost:5173",
				InvitationTokenTTL: 72 * time.Hour,
				Clock:              func() time.Time { return now },
				RandomReader:       strings.NewReader(strings.Repeat("c", 32)),
			}),
		)

		if err := service.ResendInvitation(context.Background(), 21); err != nil {
			t.Fatalf("ResendInvitation returned error: %v", err)
		}
		if sendCount != 1 {
			t.Fatalf("expected one invitation email, got %d", sendCount)
		}
	})

	t.Run("rejects non-pending users", func(t *testing.T) {
		t.Parallel()

		txUserRepo := &setupUserRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return &model.User{
					ID:     id,
					Email:  "active@example.com",
					Name:   "Active User",
					Status: model.UserStatusActive,
				}, nil
			},
		}

		service := NewUserService(
			&userRepositoryMock{},
			&userSessionStoreMock{},
			WithSetupFlows(SetupFlowConfig{
				TxManager:    &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, &setupTokenRepositoryMock{})},
				Emailer:      &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error { return nil }},
				Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
				AppBaseURL:   "http://localhost:5173",
				Clock:        time.Now,
				RandomReader: strings.NewReader(strings.Repeat("d", 32)),
			}),
		)

		err := service.ResendInvitation(context.Background(), 21)
		if !errors.Is(err, model.ErrUserNotPending) {
			t.Fatalf("expected ErrUserNotPending, got %v", err)
		}
	})
}

func TestAuthServiceSetupFlows(t *testing.T) {
	t.Run("RequestPasswordReset creates a reset token only for active users", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		txUserRepo := &setupUserRepositoryMock{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return &model.User{
					ID:     30,
					Email:  email,
					Name:   "Worker",
					Status: model.UserStatusActive,
				}, nil
			},
		}
		txTokenRepo := &setupTokenRepositoryMock{
			invalidateUnusedTokensFunc: func(ctx context.Context, userID int64, purpose model.SetupTokenPurpose, usedAt time.Time) error {
				if purpose != model.SetupTokenPurposePasswordReset {
					t.Fatalf("expected password reset purpose, got %q", purpose)
				}
				return nil
			},
			createFunc: func(ctx context.Context, params repository.CreateSetupTokenParams) (*model.SetupToken, error) {
				if params.Purpose != model.SetupTokenPurposePasswordReset {
					t.Fatalf("expected password reset purpose, got %q", params.Purpose)
				}
				return &model.SetupToken{ID: 4, UserID: params.UserID, TokenHash: params.TokenHash}, nil
			},
		}

		sendCount := 0
		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{},
			WithAuthSetupFlows(SetupFlowConfig{
				TxManager:             &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, txTokenRepo)},
				Emailer:               &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error { sendCount++; return nil }},
				Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
				AppBaseURL:            "http://localhost:5173",
				PasswordResetTokenTTL: time.Hour,
				Clock:                 func() time.Time { return now },
				RandomReader:          strings.NewReader(strings.Repeat("e", 32)),
			}),
		)

		if err := service.RequestPasswordReset(context.Background(), "worker@example.com"); err != nil {
			t.Fatalf("RequestPasswordReset returned error: %v", err)
		}
		if sendCount != 1 {
			t.Fatalf("expected one password reset email, got %d", sendCount)
		}
	})

	t.Run("RequestPasswordReset is a no-op for unknown, pending, or disabled users", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name string
			user *model.User
			err  error
		}{
			{name: "unknown", err: repository.ErrUserNotFound},
			{name: "pending", user: &model.User{ID: 31, Email: "pending@example.com", Status: model.UserStatusPending}},
			{name: "disabled", user: &model.User{ID: 32, Email: "disabled@example.com", Status: model.UserStatusDisabled}},
		} {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				txUserRepo := &setupUserRepositoryMock{
					getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
						if tc.err != nil {
							return nil, tc.err
						}
						return tc.user, nil
					},
				}

				createCalled := false
				service := NewAuthService(
					&authUserRepositoryMock{},
					&authSessionStoreMock{},
					WithAuthSetupFlows(SetupFlowConfig{
						TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, &setupTokenRepositoryMock{
							invalidateUnusedTokensFunc: func(ctx context.Context, userID int64, purpose model.SetupTokenPurpose, usedAt time.Time) error {
								createCalled = true
								return nil
							},
							createFunc: func(ctx context.Context, params repository.CreateSetupTokenParams) (*model.SetupToken, error) {
								createCalled = true
								return &model.SetupToken{}, nil
							},
						})},
						Emailer:      &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error { createCalled = true; return nil }},
						Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
						AppBaseURL:   "http://localhost:5173",
						Clock:        time.Now,
						RandomReader: strings.NewReader(strings.Repeat("f", 32)),
					}),
				)

				if err := service.RequestPasswordReset(context.Background(), "worker@example.com"); err != nil {
					t.Fatalf("RequestPasswordReset returned error: %v", err)
				}
				if createCalled {
					t.Fatalf("expected %s request to be a no-op", tc.name)
				}
			})
		}
	})

	t.Run("PreviewSetupToken returns the token preview for a valid token", func(t *testing.T) {
		t.Parallel()

		rawToken := validSetupToken(7)
		tokenHash := hashSetupToken(rawToken)
		service := NewAuthService(
			&authUserRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
					return &model.User{
						ID:    id,
						Email: "worker@example.com",
						Name:  "Worker",
					}, nil
				},
				getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
					return nil, repository.ErrUserNotFound
				},
			},
			&authSessionStoreMock{},
			WithAuthSetupFlows(SetupFlowConfig{
				SetupTokenRepo: &setupTokenRepositoryMock{
					getByTokenHashFunc: func(ctx context.Context, actualHash string) (*model.SetupToken, error) {
						if actualHash != tokenHash {
							t.Fatalf("expected token hash %q, got %q", tokenHash, actualHash)
						}
						return &model.SetupToken{
							ID:        5,
							UserID:    41,
							Purpose:   model.SetupTokenPurposeInvitation,
							ExpiresAt: time.Now().Add(time.Hour),
						}, nil
					},
				},
				Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
				Clock:  time.Now,
			}),
		)

		preview, err := service.PreviewSetupToken(context.Background(), rawToken)
		if err != nil {
			t.Fatalf("PreviewSetupToken returned error: %v", err)
		}
		if preview.Email != "worker@example.com" || preview.Name != "Worker" {
			t.Fatalf("unexpected preview: %+v", preview)
		}
		if preview.Purpose != model.SetupTokenPurposeInvitation {
			t.Fatalf("unexpected purpose: %q", preview.Purpose)
		}
	})

	t.Run("PreviewSetupToken maps invalid token states", func(t *testing.T) {
		t.Parallel()

		expiredToken := validSetupToken(8)
		now := time.Date(2026, 4, 20, 11, 0, 0, 0, time.UTC)
		service := NewAuthService(
			&authUserRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
					return &model.User{ID: id, Email: "worker@example.com", Name: "Worker"}, nil
				},
				getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
					return nil, repository.ErrUserNotFound
				},
			},
			&authSessionStoreMock{},
			WithAuthSetupFlows(SetupFlowConfig{
				SetupTokenRepo: &setupTokenRepositoryMock{
					getByTokenHashFunc: func(ctx context.Context, tokenHash string) (*model.SetupToken, error) {
						return &model.SetupToken{
							ID:        6,
							UserID:    41,
							Purpose:   model.SetupTokenPurposeInvitation,
							ExpiresAt: now.Add(-time.Minute),
						}, nil
					},
				},
				Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
				Clock:  func() time.Time { return now },
			}),
		)

		if _, err := service.PreviewSetupToken(context.Background(), "bad token"); !errors.Is(err, model.ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken, got %v", err)
		}
		if _, err := service.PreviewSetupToken(context.Background(), expiredToken); !errors.Is(err, model.ErrTokenExpired) {
			t.Fatalf("expected ErrTokenExpired, got %v", err)
		}
	})

	t.Run("SetupPassword updates the password, activates the user, and consumes tokens", func(t *testing.T) {
		t.Parallel()

		rawToken := validSetupToken(9)
		now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
		txUserRepo := &setupUserRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return &model.User{
					ID:      id,
					Email:   "worker@example.com",
					Name:    "Worker",
					Status:  model.UserStatusPending,
					Version: 1,
				}, nil
			},
			setPasswordAndStatusFunc: func(ctx context.Context, params repository.SetUserPasswordParams) (*model.User, error) {
				if params.Status != model.UserStatusActive {
					t.Fatalf("expected active status, got %q", params.Status)
				}
				if params.PasswordHash == "" {
					t.Fatalf("expected hashed password to be stored")
				}
				return &model.User{
					ID:      params.ID,
					Email:   "worker@example.com",
					Name:    "Worker",
					Status:  params.Status,
					Version: 2,
				}, nil
			},
		}
		txTokenRepo := &setupTokenRepositoryMock{
			getByTokenHashFunc: func(ctx context.Context, tokenHash string) (*model.SetupToken, error) {
				return &model.SetupToken{
					ID:        7,
					UserID:    51,
					Purpose:   model.SetupTokenPurposeInvitation,
					ExpiresAt: now.Add(time.Hour),
				}, nil
			},
			markUsedFunc: func(ctx context.Context, id int64, usedAt time.Time) error {
				if id != 7 {
					t.Fatalf("expected token ID 7, got %d", id)
				}
				return nil
			},
			invalidateAllUnusedFunc: func(ctx context.Context, userID int64, usedAt time.Time) error {
				if userID != 51 {
					t.Fatalf("expected user ID 51, got %d", userID)
				}
				return nil
			},
		}

		service := NewAuthService(
			&authUserRepositoryMock{},
			&authSessionStoreMock{},
			WithAuthSetupFlows(SetupFlowConfig{
				TxManager:    &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, txTokenRepo)},
				Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
				Clock:        func() time.Time { return now },
				RandomReader: strings.NewReader(strings.Repeat("g", 32)),
			}),
		)

		if err := service.SetupPassword(context.Background(), SetupPasswordInput{
			Token:    rawToken,
			Password: "pa55word",
		}); err != nil {
			t.Fatalf("SetupPassword returned error: %v", err)
		}
	})
}

type setupUserRepositoryMock struct {
	getByIDFunc              func(ctx context.Context, id int64) (*model.User, error)
	getByEmailFunc           func(ctx context.Context, email string) (*model.User, error)
	createFunc               func(ctx context.Context, params repository.CreateUserParams) (*model.User, error)
	setPasswordAndStatusFunc func(ctx context.Context, params repository.SetUserPasswordParams) (*model.User, error)
}

func (m *setupUserRepositoryMock) GetByID(ctx context.Context, id int64) (*model.User, error) {
	return m.getByIDFunc(ctx, id)
}

func (m *setupUserRepositoryMock) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return m.getByEmailFunc(ctx, email)
}

func (m *setupUserRepositoryMock) Create(ctx context.Context, params repository.CreateUserParams) (*model.User, error) {
	return m.createFunc(ctx, params)
}

func (m *setupUserRepositoryMock) SetPasswordAndStatus(ctx context.Context, params repository.SetUserPasswordParams) (*model.User, error) {
	return m.setPasswordAndStatusFunc(ctx, params)
}

func withSetupRepos(
	userRepo setupUserRepository,
	tokenRepo setupTokenRepository,
) func(
	ctx context.Context,
	fn func(ctx context.Context, userRepo setupUserRepository, tokenRepo setupTokenRepository) error,
) error {
	return func(
		ctx context.Context,
		fn func(ctx context.Context, userRepo setupUserRepository, tokenRepo setupTokenRepository) error,
	) error {
		return fn(ctx, userRepo, tokenRepo)
	}
}

func validSetupToken(seed byte) string {
	data := make([]byte, 32)
	for i := range data {
		data[i] = seed
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
