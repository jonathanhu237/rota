package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type userRepositoryMock struct {
	getByIDFunc          func(ctx context.Context, id int64) (*model.User, error)
	getByEmailFunc       func(ctx context.Context, email string) (*model.User, error)
	listPaginatedFunc    func(ctx context.Context, params repository.ListUsersParams) ([]*model.User, int, error)
	createFunc           func(ctx context.Context, params repository.CreateUserParams) (*model.User, error)
	updateFunc           func(ctx context.Context, params repository.UpdateUserParams) (*model.User, error)
	updateOwnProfileFunc func(ctx context.Context, params repository.UpdateOwnProfileParams) (*model.User, error)
	updateStatusFunc     func(ctx context.Context, params repository.UpdateUserStatusParams) (*model.User, error)
	updatePasswordFunc   func(ctx context.Context, params repository.UpdateUserPasswordParams) (*model.User, error)
}

func (m *userRepositoryMock) GetByID(ctx context.Context, id int64) (*model.User, error) {
	return m.getByIDFunc(ctx, id)
}

func (m *userRepositoryMock) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return m.getByEmailFunc(ctx, email)
}

func (m *userRepositoryMock) ListPaginated(ctx context.Context, params repository.ListUsersParams) ([]*model.User, int, error) {
	return m.listPaginatedFunc(ctx, params)
}

func (m *userRepositoryMock) Create(ctx context.Context, params repository.CreateUserParams) (*model.User, error) {
	return m.createFunc(ctx, params)
}

func (m *userRepositoryMock) Update(ctx context.Context, params repository.UpdateUserParams) (*model.User, error) {
	return m.updateFunc(ctx, params)
}

func (m *userRepositoryMock) UpdatePreferencesAndName(ctx context.Context, params repository.UpdateOwnProfileParams) (*model.User, error) {
	return m.updateOwnProfileFunc(ctx, params)
}

func (m *userRepositoryMock) UpdateStatus(ctx context.Context, params repository.UpdateUserStatusParams) (*model.User, error) {
	return m.updateStatusFunc(ctx, params)
}

func (m *userRepositoryMock) UpdatePassword(ctx context.Context, params repository.UpdateUserPasswordParams) (*model.User, error) {
	return m.updatePasswordFunc(ctx, params)
}

type userSessionStoreMock struct {
	deleteUserSessionsFunc func(ctx context.Context, userID int64) error
}

func (m *userSessionStoreMock) DeleteUserSessions(ctx context.Context, userID int64) error {
	return m.deleteUserSessionsFunc(ctx, userID)
}

func TestUserServiceListUsers(t *testing.T) {
	t.Run("success with pagination", func(t *testing.T) {
		t.Parallel()

		var receivedParams repository.ListUsersParams
		service := NewUserService(&userRepositoryMock{
			listPaginatedFunc: func(ctx context.Context, params repository.ListUsersParams) ([]*model.User, int, error) {
				receivedParams = params
				return []*model.User{{ID: 1}, {ID: 2}}, 25, nil
			},
		}, &userSessionStoreMock{})

		result, err := service.ListUsers(context.Background(), ListUsersInput{
			Page:     2,
			PageSize: 5,
		})
		if err != nil {
			t.Fatalf("ListUsers returned error: %v", err)
		}
		if receivedParams.Offset != 5 || receivedParams.Limit != 5 {
			t.Fatalf("expected offset=5 limit=5, got offset=%d limit=%d", receivedParams.Offset, receivedParams.Limit)
		}
		if result.Page != 2 || result.PageSize != 5 || result.Total != 25 || result.TotalPages != 5 {
			t.Fatalf("unexpected pagination result: %+v", result)
		}
	})

	t.Run("default pagination values", func(t *testing.T) {
		t.Parallel()

		var receivedParams repository.ListUsersParams
		service := NewUserService(&userRepositoryMock{
			listPaginatedFunc: func(ctx context.Context, params repository.ListUsersParams) ([]*model.User, int, error) {
				receivedParams = params
				return nil, 0, nil
			},
		}, &userSessionStoreMock{})

		result, err := service.ListUsers(context.Background(), ListUsersInput{})
		if err != nil {
			t.Fatalf("ListUsers returned error: %v", err)
		}
		if receivedParams.Offset != 0 || receivedParams.Limit != defaultUserListPageSize {
			t.Fatalf("expected default offset=0 limit=%d, got offset=%d limit=%d", defaultUserListPageSize, receivedParams.Offset, receivedParams.Limit)
		}
		if result.Page != defaultUserListPage || result.PageSize != defaultUserListPageSize {
			t.Fatalf("unexpected defaults: %+v", result)
		}
	})

	t.Run("invalid pagination", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{}, &userSessionStoreMock{})

		_, err := service.ListUsers(context.Background(), ListUsersInput{
			Page:     -1,
			PageSize: 10,
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})
}

func TestUserServiceGetUserByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return &model.User{ID: id, Email: "worker@example.com"}, nil
			},
		}, &userSessionStoreMock{})

		user, err := service.GetUserByID(context.Background(), 12)
		if err != nil {
			t.Fatalf("GetUserByID returned error: %v", err)
		}
		if user.ID != 12 {
			t.Fatalf("expected user ID 12, got %d", user.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
		}, &userSessionStoreMock{})

		_, err := service.GetUserByID(context.Background(), 12)
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{}, &userSessionStoreMock{})

		_, err := service.GetUserByID(context.Background(), 0)
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})
}

func TestUserServiceUpdateUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var updateParams repository.UpdateUserParams
		service := NewUserService(&userRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return &model.User{
					ID:      id,
					Email:   "old@example.com",
					Name:    "Old Name",
					IsAdmin: false,
					Version: 2,
				}, nil
			},
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
			updateFunc: func(ctx context.Context, params repository.UpdateUserParams) (*model.User, error) {
				updateParams = params
				return &model.User{
					ID:      params.ID,
					Email:   params.Email,
					Name:    params.Name,
					IsAdmin: params.IsAdmin,
					Version: params.Version + 1,
				}, nil
			},
		}, &userSessionStoreMock{})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		user, err := service.UpdateUser(ctx, UpdateUserInput{
			ID:      5,
			Email:   " worker@example.com ",
			Name:    " Worker ",
			IsAdmin: true,
			Version: 2,
		})
		if err != nil {
			t.Fatalf("UpdateUser returned error: %v", err)
		}
		if updateParams.ID != 5 || updateParams.Email != "worker@example.com" || updateParams.Name != "Worker" || !updateParams.IsAdmin || updateParams.Version != 2 {
			t.Fatalf("unexpected update params: %+v", updateParams)
		}
		if user.Version != 3 {
			t.Fatalf("expected returned version 3, got %d", user.Version)
		}

		event := stub.FindByAction(audit.ActionUserUpdate)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionUserUpdate, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeUser {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 5 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if _, ok := event.Metadata["email"]; !ok {
			t.Fatalf("expected email change in metadata, got %+v", event.Metadata)
		}
		if _, ok := event.Metadata["name"]; !ok {
			t.Fatalf("expected name change in metadata, got %+v", event.Metadata)
		}
		if _, ok := event.Metadata["is_admin"]; !ok {
			t.Fatalf("expected is_admin change in metadata, got %+v", event.Metadata)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{}, &userSessionStoreMock{})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		_, err := service.UpdateUser(ctx, UpdateUserInput{
			ID:      1,
			Email:   "invalid-email",
			Name:    "Worker",
			Version: 1,
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		t.Parallel()

		updateCalled := false
		service := NewUserService(&userRepositoryMock{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return &model.User{ID: 99, Email: email}, nil
			},
			updateFunc: func(ctx context.Context, params repository.UpdateUserParams) (*model.User, error) {
				updateCalled = true
				return nil, nil
			},
		}, &userSessionStoreMock{})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		_, err := service.UpdateUser(ctx, UpdateUserInput{
			ID:      1,
			Email:   "duplicate@example.com",
			Name:    "Worker",
			Version: 1,
		})
		if !errors.Is(err, ErrEmailAlreadyExists) {
			t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
		}
		if updateCalled {
			t.Fatalf("Update should not be called when duplicate email is detected early")
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
			updateFunc: func(ctx context.Context, params repository.UpdateUserParams) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
		}, &userSessionStoreMock{})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		_, err := service.UpdateUser(ctx, UpdateUserInput{
			ID:      1,
			Email:   "worker@example.com",
			Name:    "Worker",
			Version: 1,
		})
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("version conflict", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return &model.User{
					ID:      id,
					Email:   "worker@example.com",
					Name:    "Worker",
					Version: 1,
				}, nil
			},
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
			updateFunc: func(ctx context.Context, params repository.UpdateUserParams) (*model.User, error) {
				return nil, repository.ErrVersionConflict
			},
		}, &userSessionStoreMock{})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		_, err := service.UpdateUser(ctx, UpdateUserInput{
			ID:      1,
			Email:   "worker@example.com",
			Name:    "Worker",
			Version: 1,
		})
		if !errors.Is(err, ErrVersionConflict) {
			t.Fatalf("expected ErrVersionConflict, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})
}

func TestUserServiceUpdateUserStatus(t *testing.T) {
	t.Run("disable success clears sessions", func(t *testing.T) {
		t.Parallel()

		var deletedUserID int64
		service := NewUserService(&userRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return &model.User{
					ID:      id,
					Status:  model.UserStatusActive,
					Version: 2,
				}, nil
			},
			updateStatusFunc: func(ctx context.Context, params repository.UpdateUserStatusParams) (*model.User, error) {
				return &model.User{
					ID:      params.ID,
					Status:  model.UserStatusDisabled,
					Version: params.Version + 1,
				}, nil
			},
		}, &userSessionStoreMock{
			deleteUserSessionsFunc: func(ctx context.Context, userID int64) error {
				deletedUserID = userID
				return nil
			},
		})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		user, err := service.UpdateUserStatus(ctx, UpdateUserStatusInput{
			ID:      1,
			Status:  model.UserStatusDisabled,
			Version: 2,
		})
		if err != nil {
			t.Fatalf("UpdateUserStatus returned error: %v", err)
		}
		if user.Status != model.UserStatusDisabled {
			t.Fatalf("expected disabled status, got %q", user.Status)
		}
		if deletedUserID != 1 {
			t.Fatalf("expected DeleteUserSessions to be called with user ID 1, got %d", deletedUserID)
		}

		event := stub.FindByAction(audit.ActionUserStatusDisable)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionUserStatusDisable, stub.Actions())
		}
		if event.TargetID == nil || *event.TargetID != 1 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["previous_status"] != string(model.UserStatusActive) {
			t.Fatalf("expected previous_status=active, got %v", event.Metadata["previous_status"])
		}
		if event.Metadata["new_status"] != string(model.UserStatusDisabled) {
			t.Fatalf("expected new_status=disabled, got %v", event.Metadata["new_status"])
		}
	})

	t.Run("enable success does not clear sessions", func(t *testing.T) {
		t.Parallel()

		deleteCalls := 0
		service := NewUserService(&userRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return &model.User{
					ID:      id,
					Status:  model.UserStatusDisabled,
					Version: 2,
				}, nil
			},
			updateStatusFunc: func(ctx context.Context, params repository.UpdateUserStatusParams) (*model.User, error) {
				return &model.User{
					ID:      params.ID,
					Status:  model.UserStatusActive,
					Version: params.Version + 1,
				}, nil
			},
		}, &userSessionStoreMock{
			deleteUserSessionsFunc: func(ctx context.Context, userID int64) error {
				deleteCalls++
				return nil
			},
		})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		user, err := service.UpdateUserStatus(ctx, UpdateUserStatusInput{
			ID:      1,
			Status:  model.UserStatusActive,
			Version: 2,
		})
		if err != nil {
			t.Fatalf("UpdateUserStatus returned error: %v", err)
		}
		if user.Status != model.UserStatusActive {
			t.Fatalf("expected active status, got %q", user.Status)
		}
		if deleteCalls != 0 {
			t.Fatalf("DeleteUserSessions should not be called when enabling a user")
		}

		event := stub.FindByAction(audit.ActionUserStatusActivate)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionUserStatusActivate, stub.Actions())
		}
		if event.Metadata["previous_status"] != string(model.UserStatusDisabled) {
			t.Fatalf("expected previous_status=disabled, got %v", event.Metadata["previous_status"])
		}
		if event.Metadata["new_status"] != string(model.UserStatusActive) {
			t.Fatalf("expected new_status=active, got %v", event.Metadata["new_status"])
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{}, &userSessionStoreMock{})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		_, err := service.UpdateUserStatus(ctx, UpdateUserStatusInput{
			ID:      1,
			Status:  model.UserStatus("invalid"),
			Version: 1,
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
			updateStatusFunc: func(ctx context.Context, params repository.UpdateUserStatusParams) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
		}, &userSessionStoreMock{})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		_, err := service.UpdateUserStatus(ctx, UpdateUserStatusInput{
			ID:      1,
			Status:  model.UserStatusActive,
			Version: 1,
		})
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("version conflict", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return &model.User{
					ID:      id,
					Status:  model.UserStatusActive,
					Version: 1,
				}, nil
			},
			updateStatusFunc: func(ctx context.Context, params repository.UpdateUserStatusParams) (*model.User, error) {
				return nil, repository.ErrVersionConflict
			},
		}, &userSessionStoreMock{})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		_, err := service.UpdateUserStatus(ctx, UpdateUserStatusInput{
			ID:      1,
			Status:  model.UserStatusActive,
			Version: 1,
		})
		if !errors.Is(err, ErrVersionConflict) {
			t.Fatalf("expected ErrVersionConflict, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})
}

func TestUserServiceUpdateOwnProfile(t *testing.T) {
	t.Run("rejects empty trimmed name", func(t *testing.T) {
		t.Parallel()

		updateCalled := false
		service := NewUserService(&userRepositoryMock{
			updateOwnProfileFunc: func(ctx context.Context, params repository.UpdateOwnProfileParams) (*model.User, error) {
				updateCalled = true
				return nil, nil
			},
		}, &userSessionStoreMock{})

		name := "   "
		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		_, err := service.UpdateOwnProfile(ctx, UpdateOwnProfileInput{
			ID:   7,
			Name: OptionalStringField{Set: true, Value: &name},
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
		if updateCalled {
			t.Fatalf("repository update should not run for invalid name")
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("rejects out of enum language", func(t *testing.T) {
		t.Parallel()

		language := "fr"
		service := NewUserService(&userRepositoryMock{}, &userSessionStoreMock{})
		_, err := service.UpdateOwnProfile(context.Background(), UpdateOwnProfileInput{
			ID:                 7,
			LanguagePreference: OptionalStringField{Set: true, Value: &language},
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("rejects out of enum theme", func(t *testing.T) {
		t.Parallel()

		theme := "sepia"
		service := NewUserService(&userRepositoryMock{}, &userSessionStoreMock{})
		_, err := service.UpdateOwnProfile(context.Background(), UpdateOwnProfileInput{
			ID:              7,
			ThemePreference: OptionalStringField{Set: true, Value: &theme},
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("partial update only touches supplied fields and emits audit", func(t *testing.T) {
		t.Parallel()

		name := "  Alice  "
		theme := "dark"
		var received repository.UpdateOwnProfileParams
		service := NewUserService(&userRepositoryMock{
			updateOwnProfileFunc: func(ctx context.Context, params repository.UpdateOwnProfileParams) (*model.User, error) {
				received = params
				themePreference := model.ThemePreferenceDark
				return &model.User{
					ID:              params.ID,
					Name:            "Alice",
					ThemePreference: &themePreference,
					Version:         2,
				}, nil
			},
		}, &userSessionStoreMock{})
		stub := audittest.New()
		ctx := audit.WithActor(stub.ContextWith(context.Background()), 7)

		user, err := service.UpdateOwnProfile(ctx, UpdateOwnProfileInput{
			ID:              7,
			Name:            OptionalStringField{Set: true, Value: &name},
			ThemePreference: OptionalStringField{Set: true, Value: &theme},
		})
		if err != nil {
			t.Fatalf("UpdateOwnProfile returned error: %v", err)
		}
		if user.Name != "Alice" || user.ThemePreference == nil || *user.ThemePreference != model.ThemePreferenceDark {
			t.Fatalf("unexpected user: %+v", user)
		}
		if !received.Name.Set || received.Name.Value == nil || *received.Name.Value != "Alice" {
			t.Fatalf("unexpected name update field: %+v", received.Name)
		}
		if received.LanguagePreference.Set {
			t.Fatalf("language should not be touched: %+v", received.LanguagePreference)
		}
		if !received.ThemePreference.Set || received.ThemePreference.Value == nil || *received.ThemePreference.Value != "dark" {
			t.Fatalf("unexpected theme update field: %+v", received.ThemePreference)
		}

		event := stub.FindByAction(audit.ActionUserUpdate)
		if event == nil {
			t.Fatalf("expected user.update audit event, got %v", stub.Actions())
		}
		fields, ok := event.Metadata["fields"].([]string)
		if !ok {
			t.Fatalf("expected fields metadata as []string, got %+v", event.Metadata["fields"])
		}
		if !reflect.DeepEqual(fields, []string{"name", "theme_preference"}) {
			t.Fatalf("unexpected fields metadata: %+v", fields)
		}
	})
}

func TestUserServiceRequestEmailChange(t *testing.T) {
	t.Run("wrong current password is rejected", func(t *testing.T) {
		t.Parallel()

		tokenCreated := false
		emailQueued := false
		txUserRepo := &setupUserRepositoryMock{
			getByIDForUpdateFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return &model.User{ID: id, Email: "alice@example.com", PasswordHash: mustHashPassword(t, "right-password")}, nil
			},
		}
		txTokenRepo := &setupTokenRepositoryMock{
			createFunc: func(ctx context.Context, params repository.CreateSetupTokenParams) (*model.SetupToken, error) {
				tokenCreated = true
				return nil, nil
			},
		}
		service := NewUserService(
			&userRepositoryMock{},
			&userSessionStoreMock{},
			WithSetupFlows(SetupFlowConfig{
				TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, txTokenRepo)},
				OutboxRepo: &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error {
					emailQueued = true
					return nil
				}},
			}),
		)
		stub := audittest.New()

		err := service.RequestEmailChange(stub.ContextWith(context.Background()), RequestEmailChangeInput{
			UserID:          7,
			NewEmail:        "alice2@example.com",
			CurrentPassword: "wrong-password",
		})
		if !errors.Is(err, ErrInvalidCurrentPassword) {
			t.Fatalf("expected ErrInvalidCurrentPassword, got %v", err)
		}
		if tokenCreated || emailQueued {
			t.Fatalf("token/email side effects should not run on wrong password")
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events, got %v", stub.Actions())
		}
	})

	t.Run("same as current email is rejected", func(t *testing.T) {
		t.Parallel()

		txUserRepo := &setupUserRepositoryMock{
			getByIDForUpdateFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return &model.User{ID: id, Email: "alice@example.com", PasswordHash: mustHashPassword(t, "pa55word")}, nil
			},
		}
		service := NewUserService(
			&userRepositoryMock{},
			&userSessionStoreMock{},
			WithSetupFlows(SetupFlowConfig{
				TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, &setupTokenRepositoryMock{})},
			}),
		)

		err := service.RequestEmailChange(context.Background(), RequestEmailChangeInput{
			UserID:          7,
			NewEmail:        "ALICE@example.com",
			CurrentPassword: "pa55word",
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("invalid email shape is rejected before transaction", func(t *testing.T) {
		t.Parallel()

		txCalled := false
		service := NewUserService(
			&userRepositoryMock{},
			&userSessionStoreMock{},
			WithSetupFlows(SetupFlowConfig{
				TxManager: &setupTxManagerMock{withinTxFunc: func(ctx context.Context, fn func(context.Context, *sql.Tx, setupUserRepository, setupTokenRepository) error) error {
					txCalled = true
					return nil
				}},
			}),
		)

		err := service.RequestEmailChange(context.Background(), RequestEmailChangeInput{
			UserID:          7,
			NewEmail:        "not-an-email",
			CurrentPassword: "pa55word",
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
		if txCalled {
			t.Fatalf("transaction should not run for invalid email shape")
		}
	})

	t.Run("new email collision is rejected", func(t *testing.T) {
		t.Parallel()

		txUserRepo := &setupUserRepositoryMock{
			getByIDForUpdateFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return &model.User{ID: id, Email: "alice@example.com", PasswordHash: mustHashPassword(t, "pa55word")}, nil
			},
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return &model.User{ID: 99, Email: email}, nil
			},
		}
		tokenCreated := false
		service := NewUserService(
			&userRepositoryMock{},
			&userSessionStoreMock{},
			WithSetupFlows(SetupFlowConfig{
				TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, &setupTokenRepositoryMock{
					createFunc: func(ctx context.Context, params repository.CreateSetupTokenParams) (*model.SetupToken, error) {
						tokenCreated = true
						return nil, nil
					},
				})},
			}),
		)

		err := service.RequestEmailChange(context.Background(), RequestEmailChangeInput{
			UserID:          7,
			NewEmail:        "bob@example.com",
			CurrentPassword: "pa55word",
		})
		if !errors.Is(err, ErrEmailAlreadyExists) {
			t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
		}
		if tokenCreated {
			t.Fatalf("token should not be created on collision")
		}
	})

	t.Run("success invalidates prior token, creates new token, queues two emails, and emits audit", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 26, 14, 0, 0, 0, time.UTC)
		newEmail := "alice2@example.com"
		var invalidatedPurpose model.SetupTokenPurpose
		var createdParams repository.CreateSetupTokenParams
		var messages []email.Message
		txUserRepo := &setupUserRepositoryMock{
			getByIDForUpdateFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return &model.User{
					ID:           id,
					Email:        "alice@example.com",
					Name:         "Alice",
					PasswordHash: mustHashPassword(t, "pa55word"),
				}, nil
			},
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
		}
		txTokenRepo := &setupTokenRepositoryMock{
			invalidateUnusedTokensFunc: func(ctx context.Context, userID int64, purpose model.SetupTokenPurpose, usedAt time.Time) error {
				if userID != 7 || !usedAt.Equal(now) {
					t.Fatalf("unexpected invalidation input: user=%d at=%s", userID, usedAt)
				}
				invalidatedPurpose = purpose
				return nil
			},
			createFunc: func(ctx context.Context, params repository.CreateSetupTokenParams) (*model.SetupToken, error) {
				createdParams = params
				return &model.SetupToken{ID: 123, UserID: params.UserID, NewEmail: params.NewEmail}, nil
			},
		}
		service := NewUserService(
			&userRepositoryMock{},
			&userSessionStoreMock{},
			WithSetupFlows(SetupFlowConfig{
				TxManager: &setupTxManagerMock{withinTxFunc: withSetupRepos(txUserRepo, txTokenRepo)},
				OutboxRepo: &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error {
					messages = append(messages, msg)
					return nil
				}},
				AppBaseURL:   "https://app.example.com",
				Clock:        func() time.Time { return now },
				RandomReader: strings.NewReader(strings.Repeat("e", 32)),
			}),
		)
		stub := audittest.New()
		ctx := audit.WithActor(stub.ContextWith(context.Background()), 7)

		err := service.RequestEmailChange(ctx, RequestEmailChangeInput{
			UserID:          7,
			NewEmail:        newEmail,
			CurrentPassword: "pa55word",
		})
		if err != nil {
			t.Fatalf("RequestEmailChange returned error: %v", err)
		}
		if invalidatedPurpose != model.SetupTokenPurposeEmailChange {
			t.Fatalf("expected email_change invalidation, got %q", invalidatedPurpose)
		}
		if createdParams.Purpose != model.SetupTokenPurposeEmailChange {
			t.Fatalf("expected email_change token, got %+v", createdParams)
		}
		if createdParams.NewEmail == nil || *createdParams.NewEmail != newEmail {
			t.Fatalf("expected token new_email %q, got %+v", newEmail, createdParams.NewEmail)
		}
		if !createdParams.ExpiresAt.Equal(now.Add(24 * time.Hour)) {
			t.Fatalf("expected expiry %s, got %s", now.Add(24*time.Hour), createdParams.ExpiresAt)
		}
		if len(messages) != 2 {
			t.Fatalf("expected 2 outbox messages, got %d", len(messages))
		}
		if messages[0].To != newEmail || !strings.Contains(messages[0].Body, "/auth/confirm-email-change?token=") {
			t.Fatalf("unexpected confirm message: %+v", messages[0])
		}
		if messages[1].To != "alice@example.com" || strings.Contains(messages[1].Body, "?token=") {
			t.Fatalf("unexpected notice message: %+v", messages[1])
		}

		event := stub.FindByAction(audit.ActionUserEmailChangeRequest)
		if event == nil {
			t.Fatalf("expected email change request audit event, got %v", stub.Actions())
		}
		if event.Metadata["new_email_normalized"] != newEmail {
			t.Fatalf("unexpected audit metadata: %+v", event.Metadata)
		}
	})
}

// TestUserServiceAuditMetadataHasNoSecrets is a belt-and-suspenders guard
// against future regressions that might accidentally leak secrets into the
// audit event metadata.
func TestUserServiceAuditMetadataHasNoSecrets(t *testing.T) {
	t.Parallel()

	// Use the CreateUser invitation flow so every side effect along the
	// happy path is exercised and captured by the stub recorder.
	stub := audittest.New()
	ctx := stub.ContextWith(context.Background())

	txUserRepo := &setupUserRepositoryMock{
		createFunc: func(ctx context.Context, params repository.CreateUserParams) (*model.User, error) {
			return &model.User{
				ID:      42,
				Email:   params.Email,
				Name:    params.Name,
				IsAdmin: params.IsAdmin,
				Status:  params.Status,
			}, nil
		},
	}
	txTokenRepo := &setupTokenRepositoryMock{
		invalidateUnusedTokensFunc: func(ctx context.Context, userID int64, purpose model.SetupTokenPurpose, usedAt time.Time) error {
			return nil
		},
		createFunc: func(ctx context.Context, params repository.CreateSetupTokenParams) (*model.SetupToken, error) {
			return &model.SetupToken{ID: 1, UserID: params.UserID, TokenHash: params.TokenHash}, nil
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
			OutboxRepo:         &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error { return nil }},
			Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
			AppBaseURL:         "http://localhost:5173",
			InvitationTokenTTL: 72 * time.Hour,
			Clock:              func() time.Time { return time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC) },
			RandomReader:       strings.NewReader(strings.Repeat("z", 32)),
		}),
	)

	user, err := service.CreateUser(ctx, CreateUserInput{
		Email:   "worker@example.com",
		Name:    "Worker",
		IsAdmin: true,
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if user.ID != 42 {
		t.Fatalf("expected user ID 42, got %d", user.ID)
	}

	events := stub.Events()
	if len(events) == 0 {
		t.Fatalf("expected at least one audit event")
	}

	forbidden := []string{"password", "password_hash", "token", "session"}
	for _, event := range events {
		encoded, err := json.Marshal(event.Metadata)
		if err != nil {
			t.Fatalf("marshal metadata for action %q: %v", event.Action, err)
		}
		lower := strings.ToLower(string(encoded))
		for _, needle := range forbidden {
			if strings.Contains(lower, needle) {
				t.Fatalf(
					"audit metadata for action %q contains forbidden substring %q: %s",
					event.Action, needle, string(encoded),
				)
			}
		}
	}
}
