package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

type userService interface {
	ListUsers(ctx context.Context, input service.ListUsersInput) (*service.ListUsersResult, error)
	CreateUser(ctx context.Context, input service.CreateUserInput) (*model.User, error)
	ResendInvitation(ctx context.Context, userID int64) error
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
	UpdateUser(ctx context.Context, input service.UpdateUserInput) (*model.User, error)
	UpdateOwnProfile(ctx context.Context, input service.UpdateOwnProfileInput) (*model.User, error)
	UpdateUserStatus(ctx context.Context, input service.UpdateUserStatusInput) (*model.User, error)
}

type UserHandler struct {
	userService userService
}

type usersResponse struct {
	Users      []userResponse     `json:"users"`
	Pagination paginationResponse `json:"pagination"`
}

type userDetailResponse struct {
	User userResponse `json:"user"`
}

type createUserRequest struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
}

type updateUserRequest struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
	Version int    `json:"version"`
}

type updateUserStatusRequest struct {
	Status  model.UserStatus `json:"status"`
	Version int              `json:"version"`
}

type optionalStringRequestField struct {
	Set   bool
	Value *string
}

type updateOwnProfileRequest struct {
	Name               optionalStringRequestField `json:"name"`
	LanguagePreference optionalStringRequestField `json:"language_preference"`
	ThemePreference    optionalStringRequestField `json:"theme_preference"`
}

func (f *optionalStringRequestField) UnmarshalJSON(data []byte) error {
	f.Set = true
	if string(data) == "null" {
		f.Value = nil
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	f.Value = &value
	return nil
}

func (f optionalStringRequestField) toServiceField() service.OptionalStringField {
	return service.OptionalStringField{
		Set:   f.Set,
		Value: f.Value,
	}
}

func NewUserHandler(userService userService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	page, err := parseOptionalInt(r.URL.Query().Get("page"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid page parameter")
		return
	}

	pageSize, err := parseOptionalInt(r.URL.Query().Get("page_size"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid page size parameter")
		return
	}

	result, err := h.userService.ListUsers(r.Context(), service.ListUsersInput{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		h.writeUserServiceError(w, err)
		return
	}

	users := make([]userResponse, 0, len(result.Users))
	for _, user := range result.Users {
		users = append(users, newUserResponse(user))
	}

	writeData(w, http.StatusOK, usersResponse{
		Users: users,
		Pagination: paginationResponse{
			Page:       result.Page,
			PageSize:   result.PageSize,
			Total:      result.Total,
			TotalPages: result.TotalPages,
		},
	})
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	user, err := h.userService.CreateUser(r.Context(), service.CreateUserInput{
		Email:   req.Email,
		Name:    req.Name,
		IsAdmin: req.IsAdmin,
	})
	if err != nil {
		h.writeUserServiceError(w, err)
		return
	}

	writeData(w, http.StatusCreated, userDetailResponse{
		User: newUserResponse(user),
	})
}

func (h *UserHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid user id")
		return
	}

	user, err := h.userService.GetUserByID(r.Context(), id)
	if err != nil {
		h.writeUserServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, userDetailResponse{
		User: newUserResponse(user),
	})
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid user id")
		return
	}

	var req updateUserRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	user, err := h.userService.UpdateUser(r.Context(), service.UpdateUserInput{
		ID:      id,
		Email:   req.Email,
		Name:    req.Name,
		IsAdmin: req.IsAdmin,
		Version: req.Version,
	})
	if err != nil {
		h.writeUserServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, userDetailResponse{
		User: newUserResponse(user),
	})
}

func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}

	var req updateOwnProfileRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	updatedUser, err := h.userService.UpdateOwnProfile(r.Context(), service.UpdateOwnProfileInput{
		ID:                 user.ID,
		Name:               req.Name.toServiceField(),
		LanguagePreference: req.LanguagePreference.toServiceField(),
		ThemePreference:    req.ThemePreference.toServiceField(),
	})
	if err != nil {
		h.writeUserServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, userDetailResponse{
		User: newUserResponse(updatedUser),
	})
}

func (h *UserHandler) ResendInvitation(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid user id")
		return
	}

	if err := h.userService.ResendInvitation(r.Context(), id); err != nil {
		h.writeUserServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid user id")
		return
	}

	var req updateUserStatusRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	user, err := h.userService.UpdateUserStatus(r.Context(), service.UpdateUserStatusInput{
		ID:      id,
		Status:  req.Status,
		Version: req.Version,
	})
	if err != nil {
		h.writeUserServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, userDetailResponse{
		User: newUserResponse(user),
	})
}

func (h *UserHandler) writeUserServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
	case errors.Is(err, model.ErrPasswordTooShort):
		writeError(w, http.StatusBadRequest, "PASSWORD_TOO_SHORT", "Password must have at least 8 characters")
	case errors.Is(err, service.ErrEmailAlreadyExists):
		writeError(w, http.StatusConflict, "EMAIL_ALREADY_EXISTS", "Email already exists")
	case errors.Is(err, model.ErrUserNotPending):
		writeError(w, http.StatusConflict, "USER_NOT_PENDING", "User is not pending")
	case errors.Is(err, service.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
	case errors.Is(err, service.ErrVersionConflict):
		writeError(w, http.StatusConflict, "VERSION_CONFLICT", "User has been updated by another request")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}

func parseOptionalInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsedValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsedValue, nil
}

func parsePathID(r *http.Request, key string) (int64, error) {
	value := r.PathValue(key)
	if value == "" {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseInt(value, 10, 64)
}
