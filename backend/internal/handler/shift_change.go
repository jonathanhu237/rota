package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

// ShiftChangeService defines the service contract consumed by ShiftChangeHandler.
type shiftChangeService interface {
	CreateShiftChangeRequest(ctx context.Context, input service.CreateShiftChangeInput) (*model.ShiftChangeRequest, error)
	GetShiftChangeRequest(ctx context.Context, requestID, viewerUserID int64, viewerIsAdmin bool) (*model.ShiftChangeRequest, error)
	ListShiftChangeRequests(ctx context.Context, publicationID, viewerUserID int64, viewerIsAdmin bool) ([]*model.ShiftChangeRequest, error)
	CountPendingForViewer(ctx context.Context, viewerUserID int64) (int, error)
	CancelShiftChangeRequest(ctx context.Context, requestID, viewerUserID int64) error
	RejectShiftChangeRequest(ctx context.Context, requestID, viewerUserID int64) error
	ApproveShiftChangeRequest(ctx context.Context, requestID, viewerUserID int64) error
	ListPublicationMembers(ctx context.Context, publicationID int64) ([]service.PublicationMember, error)
}

// ShiftChangeHandler exposes the shift-change endpoints.
type ShiftChangeHandler struct {
	shiftChangeService shiftChangeService
}

// NewShiftChangeHandler constructs a new handler.
func NewShiftChangeHandler(shiftChangeService shiftChangeService) *ShiftChangeHandler {
	return &ShiftChangeHandler{shiftChangeService: shiftChangeService}
}

type createShiftChangeRequestBody struct {
	Type                      string  `json:"type"`
	RequesterAssignmentID     int64   `json:"requester_assignment_id"`
	OccurrenceDate            string  `json:"occurrence_date"`
	CounterpartUserID         *int64  `json:"counterpart_user_id,omitempty"`
	CounterpartAssignmentID   *int64  `json:"counterpart_assignment_id,omitempty"`
	CounterpartOccurrenceDate *string `json:"counterpart_occurrence_date,omitempty"`
}

type shiftChangeRequestResponse struct {
	ID                        int64   `json:"id"`
	PublicationID             int64   `json:"publication_id"`
	Type                      string  `json:"type"`
	RequesterUserID           int64   `json:"requester_user_id"`
	RequesterAssignmentID     int64   `json:"requester_assignment_id"`
	OccurrenceDate            string  `json:"occurrence_date"`
	CounterpartUserID         *int64  `json:"counterpart_user_id"`
	CounterpartAssignmentID   *int64  `json:"counterpart_assignment_id"`
	CounterpartOccurrenceDate *string `json:"counterpart_occurrence_date"`
	State                     string  `json:"state"`
	DecidedByUserID           *int64  `json:"decided_by_user_id"`
	CreatedAt                 string  `json:"created_at"`
	DecidedAt                 *string `json:"decided_at"`
	ExpiresAt                 string  `json:"expires_at"`
}

type shiftChangeRequestDetailResponse struct {
	Request shiftChangeRequestResponse `json:"request"`
}

type shiftChangeRequestListResponse struct {
	Requests []shiftChangeRequestResponse `json:"requests"`
}

type unreadCountResponse struct {
	Count int `json:"count"`
}

type publicationMembersResponse struct {
	Members []publicationMemberResponse `json:"members"`
}

type publicationMemberResponse struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}

// Create handles POST /publications/{id}/shift-changes.
func (h *ShiftChangeHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	var body createShiftChangeRequestBody
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	occurrenceDate, err := parseOccurrenceDate(body.OccurrenceDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid occurrence date")
		return
	}
	var counterpartOccurrenceDate *time.Time
	if body.CounterpartOccurrenceDate != nil {
		parsed, err := parseOccurrenceDate(*body.CounterpartOccurrenceDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid counterpart occurrence date")
			return
		}
		counterpartOccurrenceDate = &parsed
	}

	request, err := h.shiftChangeService.CreateShiftChangeRequest(r.Context(), service.CreateShiftChangeInput{
		PublicationID:             publicationID,
		RequesterUserID:           user.ID,
		Type:                      model.ShiftChangeType(body.Type),
		RequesterAssignmentID:     body.RequesterAssignmentID,
		OccurrenceDate:            occurrenceDate,
		CounterpartUserID:         body.CounterpartUserID,
		CounterpartAssignmentID:   body.CounterpartAssignmentID,
		CounterpartOccurrenceDate: counterpartOccurrenceDate,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}

	writeData(w, http.StatusCreated, shiftChangeRequestDetailResponse{Request: newShiftChangeRequestResponse(request)})
}

// GetByID handles GET /publications/{id}/shift-changes/{request_id}.
func (h *ShiftChangeHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	requestID, err := parsePathID(r, "request_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request id")
		return
	}

	req, err := h.shiftChangeService.GetShiftChangeRequest(r.Context(), requestID, user.ID, user.IsAdmin)
	if err != nil {
		h.writeError(w, err)
		return
	}

	writeData(w, http.StatusOK, shiftChangeRequestDetailResponse{Request: newShiftChangeRequestResponse(req)})
}

// List handles GET /publications/{id}/shift-changes.
func (h *ShiftChangeHandler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	rows, err := h.shiftChangeService.ListShiftChangeRequests(r.Context(), publicationID, user.ID, user.IsAdmin)
	if err != nil {
		h.writeError(w, err)
		return
	}

	responses := make([]shiftChangeRequestResponse, 0, len(rows))
	for _, req := range rows {
		responses = append(responses, newShiftChangeRequestResponse(req))
	}
	writeData(w, http.StatusOK, shiftChangeRequestListResponse{Requests: responses})
}

// Approve handles POST /publications/{id}/shift-changes/{request_id}/approve.
// The same endpoint serves both "approve" (swap / give_direct) and
// "claim" (give_pool) semantics.
func (h *ShiftChangeHandler) Approve(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	requestID, err := parsePathID(r, "request_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request id")
		return
	}

	if err := h.shiftChangeService.ApproveShiftChangeRequest(r.Context(), requestID, user.ID); err != nil {
		h.writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Reject handles POST /publications/{id}/shift-changes/{request_id}/reject.
func (h *ShiftChangeHandler) Reject(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	requestID, err := parsePathID(r, "request_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request id")
		return
	}

	if err := h.shiftChangeService.RejectShiftChangeRequest(r.Context(), requestID, user.ID); err != nil {
		h.writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Cancel handles POST /publications/{id}/shift-changes/{request_id}/cancel.
func (h *ShiftChangeHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	requestID, err := parsePathID(r, "request_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request id")
		return
	}

	if err := h.shiftChangeService.CancelShiftChangeRequest(r.Context(), requestID, user.ID); err != nil {
		h.writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListMembers handles GET /publications/{id}/members.
func (h *ShiftChangeHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	members, err := h.shiftChangeService.ListPublicationMembers(r.Context(), publicationID)
	if err != nil {
		h.writeError(w, err)
		return
	}

	out := make([]publicationMemberResponse, 0, len(members))
	for _, m := range members {
		out = append(out, publicationMemberResponse{UserID: m.UserID, Name: m.Name})
	}
	writeData(w, http.StatusOK, publicationMembersResponse{Members: out})
}

// UnreadCount handles GET /users/me/notifications/unread-count.
func (h *ShiftChangeHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	count, err := h.shiftChangeService.CountPendingForViewer(r.Context(), user.ID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, unreadCountResponse{Count: count})
}

func (h *ShiftChangeHandler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
	case errors.Is(err, service.ErrInvalidOccurrenceDate):
		writeError(w, http.StatusBadRequest, "INVALID_OCCURRENCE_DATE", "Invalid occurrence date")
	case errors.Is(err, service.ErrShiftChangeInvalidType):
		writeError(w, http.StatusBadRequest, "SHIFT_CHANGE_INVALID_TYPE", "Invalid shift change type or payload")
	case errors.Is(err, service.ErrShiftChangeSelf):
		writeError(w, http.StatusBadRequest, "SHIFT_CHANGE_SELF", "Cannot target yourself")
	case errors.Is(err, service.ErrShiftChangeNotOwner):
		writeError(w, http.StatusForbidden, "SHIFT_CHANGE_NOT_OWNER", "Not authorized for this request")
	case errors.Is(err, service.ErrShiftChangeNotQualified):
		writeError(w, http.StatusForbidden, "SHIFT_CHANGE_NOT_QUALIFIED", "Not qualified for the involved shift")
	case errors.Is(err, service.ErrUserDisabled):
		writeError(w, http.StatusConflict, "USER_DISABLED", "User is disabled")
	case errors.Is(err, service.ErrShiftChangeNotPending):
		writeError(w, http.StatusConflict, "SHIFT_CHANGE_NOT_PENDING", "Request is no longer pending")
	case errors.Is(err, service.ErrShiftChangeExpired):
		writeError(w, http.StatusConflict, "SHIFT_CHANGE_EXPIRED", "Request has expired")
	case errors.Is(err, service.ErrShiftChangeInvalidated):
		writeError(w, http.StatusConflict, "SHIFT_CHANGE_INVALIDATED", "Request is no longer applicable")
	case errors.Is(err, service.ErrShiftChangeNotFound):
		writeError(w, http.StatusNotFound, "SHIFT_CHANGE_NOT_FOUND", "Request not found")
	case errors.Is(err, service.ErrPublicationNotPublished):
		writeError(w, http.StatusConflict, "PUBLICATION_NOT_PUBLISHED", "Publication is not published")
	case errors.Is(err, service.ErrPublicationNotFound):
		writeError(w, http.StatusNotFound, "PUBLICATION_NOT_FOUND", "Publication not found")
	case errors.Is(err, service.ErrSchedulingRetryable):
		writeError(w, http.StatusServiceUnavailable, "SCHEDULING_RETRYABLE", "Scheduling conflict, please retry")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}

func newShiftChangeRequestResponse(req *model.ShiftChangeRequest) shiftChangeRequestResponse {
	resp := shiftChangeRequestResponse{
		ID:                      req.ID,
		PublicationID:           req.PublicationID,
		Type:                    string(req.Type),
		RequesterUserID:         req.RequesterUserID,
		RequesterAssignmentID:   req.RequesterAssignmentID,
		OccurrenceDate:          req.OccurrenceDate.Format("2006-01-02"),
		CounterpartUserID:       req.CounterpartUserID,
		CounterpartAssignmentID: req.CounterpartAssignmentID,
		State:                   string(req.State),
		DecidedByUserID:         req.DecidedByUserID,
		CreatedAt:               req.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		ExpiresAt:               req.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if req.DecidedAt != nil {
		decided := req.DecidedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.DecidedAt = &decided
	}
	if req.CounterpartOccurrenceDate != nil {
		counterpartOccurrenceDate := req.CounterpartOccurrenceDate.Format("2006-01-02")
		resp.CounterpartOccurrenceDate = &counterpartOccurrenceDate
	}
	return resp
}

func parseOccurrenceDate(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, errors.New("empty occurrence date")
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}
