package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type userRepositoryMock struct {
	getByIDFunc        func(ctx context.Context, id int64) (*model.User, error)
	getByEmailFunc     func(ctx context.Context, email string) (*model.User, error)
	listPaginatedFunc  func(ctx context.Context, params repository.ListUsersParams) ([]*model.User, int, error)
	createFunc         func(ctx context.Context, params repository.CreateUserParams) (*model.User, error)
	updateFunc         func(ctx context.Context, params repository.UpdateUserParams) (*model.User, error)
	updateStatusFunc   func(ctx context.Context, params repository.UpdateUserStatusParams) (*model.User, error)
	updatePasswordFunc func(ctx context.Context, params repository.UpdateUserPasswordParams) (*model.User, error)
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

		user, err := service.UpdateUser(context.Background(), UpdateUserInput{
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
	})

	t.Run("invalid email", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{}, &userSessionStoreMock{})

		_, err := service.UpdateUser(context.Background(), UpdateUserInput{
			ID:      1,
			Email:   "invalid-email",
			Name:    "Worker",
			Version: 1,
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
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

		_, err := service.UpdateUser(context.Background(), UpdateUserInput{
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
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
			updateFunc: func(ctx context.Context, params repository.UpdateUserParams) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
		}, &userSessionStoreMock{})

		_, err := service.UpdateUser(context.Background(), UpdateUserInput{
			ID:      1,
			Email:   "worker@example.com",
			Name:    "Worker",
			Version: 1,
		})
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("version conflict", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{
			getByEmailFunc: func(ctx context.Context, email string) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
			updateFunc: func(ctx context.Context, params repository.UpdateUserParams) (*model.User, error) {
				return nil, repository.ErrVersionConflict
			},
		}, &userSessionStoreMock{})

		_, err := service.UpdateUser(context.Background(), UpdateUserInput{
			ID:      1,
			Email:   "worker@example.com",
			Name:    "Worker",
			Version: 1,
		})
		if !errors.Is(err, ErrVersionConflict) {
			t.Fatalf("expected ErrVersionConflict, got %v", err)
		}
	})
}

func TestUserServiceUpdateUserStatus(t *testing.T) {
	t.Run("disable success clears sessions", func(t *testing.T) {
		t.Parallel()

		var deletedUserID int64
		service := NewUserService(&userRepositoryMock{
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

		user, err := service.UpdateUserStatus(context.Background(), UpdateUserStatusInput{
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
	})

	t.Run("enable success does not clear sessions", func(t *testing.T) {
		t.Parallel()

		deleteCalls := 0
		service := NewUserService(&userRepositoryMock{
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

		user, err := service.UpdateUserStatus(context.Background(), UpdateUserStatusInput{
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
	})

	t.Run("invalid status", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{}, &userSessionStoreMock{})

		_, err := service.UpdateUserStatus(context.Background(), UpdateUserStatusInput{
			ID:      1,
			Status:  model.UserStatus("invalid"),
			Version: 1,
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{
			updateStatusFunc: func(ctx context.Context, params repository.UpdateUserStatusParams) (*model.User, error) {
				return nil, repository.ErrUserNotFound
			},
		}, &userSessionStoreMock{})

		_, err := service.UpdateUserStatus(context.Background(), UpdateUserStatusInput{
			ID:      1,
			Status:  model.UserStatusActive,
			Version: 1,
		})
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("version conflict", func(t *testing.T) {
		t.Parallel()

		service := NewUserService(&userRepositoryMock{
			updateStatusFunc: func(ctx context.Context, params repository.UpdateUserStatusParams) (*model.User, error) {
				return nil, repository.ErrVersionConflict
			},
		}, &userSessionStoreMock{})

		_, err := service.UpdateUserStatus(context.Background(), UpdateUserStatusInput{
			ID:      1,
			Status:  model.UserStatusActive,
			Version: 1,
		})
		if !errors.Is(err, ErrVersionConflict) {
			t.Fatalf("expected ErrVersionConflict, got %v", err)
		}
	})
}
