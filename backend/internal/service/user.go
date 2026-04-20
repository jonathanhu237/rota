package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

const (
	defaultUserListPage     = 1
	defaultUserListPageSize = 10
	maxUserListPageSize     = 100
)

var (
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidInput       = errors.New("invalid input")
	ErrUserNotFound       = errors.New("user not found")
	ErrVersionConflict    = errors.New("version conflict")
)

type userRepository interface {
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	ListPaginated(ctx context.Context, params repository.ListUsersParams) ([]*model.User, int, error)
	Create(ctx context.Context, params repository.CreateUserParams) (*model.User, error)
	Update(ctx context.Context, params repository.UpdateUserParams) (*model.User, error)
	UpdateStatus(ctx context.Context, params repository.UpdateUserStatusParams) (*model.User, error)
}

type userSessionStore interface {
	DeleteUserSessions(ctx context.Context, userID int64) error
}

type UserService struct {
	userRepo     userRepository
	sessionStore userSessionStore
	setupFlows   *setupFlowHelper
}

type UserServiceOption func(*UserService)

type ListUsersInput struct {
	Page     int
	PageSize int
}

type ListUsersResult struct {
	Users      []*model.User
	Page       int
	PageSize   int
	Total      int
	TotalPages int
}

type CreateUserInput struct {
	Email   string
	Name    string
	IsAdmin bool
}

type UpdateUserInput struct {
	ID      int64
	Email   string
	Name    string
	IsAdmin bool
	Version int
}

type UpdateUserStatusInput struct {
	ID      int64
	Status  model.UserStatus
	Version int
}

func WithSetupFlows(config SetupFlowConfig) UserServiceOption {
	return func(service *UserService) {
		service.setupFlows = newSetupFlowHelper(config)
	}
}

func NewUserService(userRepo userRepository, sessionStore userSessionStore, opts ...UserServiceOption) *UserService {
	service := &UserService{
		userRepo:     userRepo,
		sessionStore: sessionStore,
	}

	for _, opt := range opts {
		opt(service)
	}

	return service
}

func (s *UserService) ListUsers(ctx context.Context, input ListUsersInput) (*ListUsersResult, error) {
	page, pageSize, err := normalizePagination(input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	users, total, err := s.userRepo.ListPaginated(ctx, repository.ListUsersParams{
		Offset: offset,
		Limit:  pageSize,
	})
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	return &ListUsersResult{
		Users:      users,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func (s *UserService) CreateUser(ctx context.Context, input CreateUserInput) (*model.User, error) {
	if s.setupFlows == nil || s.setupFlows.txManager == nil {
		return nil, ErrInvalidInput
	}

	email, name, err := normalizeUserProfileInput(input.Email, input.Name)
	if err != nil {
		return nil, err
	}
	if err := s.ensureEmailAvailable(ctx, email, 0); err != nil {
		return nil, err
	}

	var createdUser *model.User
	var rawToken string
	err = s.setupFlows.txManager.WithinTx(ctx, func(
		ctx context.Context,
		txUserRepo repository.SetupUserRepository,
		txTokenRepo repository.SetupTokenRepositoryWriter,
	) error {
		createdUser, err = txUserRepo.Create(ctx, repository.CreateUserParams{
			Email:        email,
			PasswordHash: nil,
			Name:         name,
			IsAdmin:      input.IsAdmin,
			Status:       model.UserStatusPending,
		})
		if err != nil {
			return mapRepositoryError(err)
		}

		rawToken, err = s.setupFlows.issueToken(
			ctx,
			txTokenRepo,
			createdUser.ID,
			model.SetupTokenPurposeInvitation,
			s.setupFlows.invitationTokenTTL,
		)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	s.setupFlows.sendInvitation(ctx, createdUser, rawToken)
	return createdUser, nil
}

func (s *UserService) ResendInvitation(ctx context.Context, userID int64) error {
	if s.setupFlows == nil || s.setupFlows.txManager == nil || userID <= 0 {
		return ErrInvalidInput
	}

	var user *model.User
	var rawToken string
	err := s.setupFlows.txManager.WithinTx(ctx, func(
		ctx context.Context,
		txUserRepo repository.SetupUserRepository,
		txTokenRepo repository.SetupTokenRepositoryWriter,
	) error {
		var err error
		user, err = txUserRepo.GetByID(ctx, userID)
		if err != nil {
			return mapRepositoryError(err)
		}
		if user.Status != model.UserStatusPending {
			return model.ErrUserNotPending
		}

		rawToken, err = s.setupFlows.issueToken(
			ctx,
			txTokenRepo,
			user.ID,
			model.SetupTokenPurposeInvitation,
			s.setupFlows.invitationTokenTTL,
		)
		return err
	})
	if err != nil {
		return err
	}

	s.setupFlows.sendInvitation(ctx, user, rawToken)
	return nil
}

func (s *UserService) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	if id <= 0 {
		return nil, ErrInvalidInput
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return user, nil
}

func (s *UserService) UpdateUser(ctx context.Context, input UpdateUserInput) (*model.User, error) {
	email, name, err := normalizeUserProfileInput(input.Email, input.Name)
	if err != nil {
		return nil, err
	}
	if input.ID <= 0 || input.Version <= 0 {
		return nil, ErrInvalidInput
	}
	if err := s.ensureEmailAvailable(ctx, email, input.ID); err != nil {
		return nil, err
	}

	user, err := s.userRepo.Update(ctx, repository.UpdateUserParams{
		ID:      input.ID,
		Email:   email,
		Name:    name,
		IsAdmin: input.IsAdmin,
		Version: input.Version,
	})
	if err != nil {
		return nil, mapRepositoryError(err)
	}

	return user, nil
}

func (s *UserService) UpdateUserStatus(ctx context.Context, input UpdateUserStatusInput) (*model.User, error) {
	if input.ID <= 0 || input.Version <= 0 {
		return nil, ErrInvalidInput
	}
	if input.Status != model.UserStatusActive && input.Status != model.UserStatusDisabled {
		return nil, ErrInvalidInput
	}

	user, err := s.userRepo.UpdateStatus(ctx, repository.UpdateUserStatusParams{
		ID:      input.ID,
		Status:  input.Status,
		Version: input.Version,
	})
	if err != nil {
		return nil, mapRepositoryError(err)
	}

	if user.Status == model.UserStatusDisabled && s.sessionStore != nil {
		if err := s.sessionStore.DeleteUserSessions(ctx, user.ID); err != nil {
			return nil, err
		}
	}

	return user, nil
}

func (s *UserService) ensureEmailAvailable(ctx context.Context, email string, excludeID int64) error {
	user, err := s.userRepo.GetByEmail(ctx, email)
	switch {
	case errors.Is(err, repository.ErrUserNotFound):
		return nil
	case err != nil:
		return err
	case user.ID != excludeID:
		return ErrEmailAlreadyExists
	default:
		return nil
	}
}

func normalizePagination(page, pageSize int) (int, int, error) {
	if page == 0 {
		page = defaultUserListPage
	}
	if pageSize == 0 {
		pageSize = defaultUserListPageSize
	}
	if page < 1 || pageSize < 1 || pageSize > maxUserListPageSize {
		return 0, 0, ErrInvalidInput
	}
	return page, pageSize, nil
}

func normalizeUserProfileInput(email, name string) (string, string, error) {
	normalizedEmail := strings.TrimSpace(email)
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" || !isValidEmail(normalizedEmail) {
		return "", "", ErrInvalidInput
	}
	return normalizedEmail, normalizedName, nil
}

func isValidEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}

func mapRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrEmailAlreadyExists):
		return ErrEmailAlreadyExists
	case errors.Is(err, repository.ErrUserNotFound):
		return ErrUserNotFound
	case errors.Is(err, repository.ErrVersionConflict):
		return ErrVersionConflict
	default:
		return err
	}
}
