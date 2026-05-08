package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

type leaveService interface {
	Create(ctx context.Context, input service.CreateLeaveInput) (*service.LeaveDetail, error)
	Cancel(ctx context.Context, leaveID, userID int64) error
	GetByID(ctx context.Context, leaveID int64, viewerUserID int64, viewerIsAdmin bool) (*service.LeaveDetail, error)
	ListPool(ctx context.Context, viewerUserID int64, viewerIsAdmin bool, input service.ListLeavePoolInput) (*service.LeavePoolResult, error)
	ListForUser(ctx context.Context, userID int64, input service.ListLeavesInput) ([]*service.LeaveDetail, error)
	ListForPublication(ctx context.Context, publicationID int64, input service.ListLeavesInput) ([]*service.LeaveDetail, error)
	PreviewOccurrences(ctx context.Context, userID int64, from time.Time, to time.Time) ([]*service.OccurrencePreview, error)
}

type LeaveHandler struct {
	leaveService leaveService
}

func NewLeaveHandler(leaveService leaveService) *LeaveHandler {
	return &LeaveHandler{leaveService: leaveService}
}

type createLeaveRequest struct {
	AssignmentID      int64  `json:"assignment_id"`
	OccurrenceDate    string `json:"occurrence_date"`
	Type              string `json:"type"`
	CounterpartUserID *int64 `json:"counterpart_user_id,omitempty"`
	Category          string `json:"category"`
	Reason            string `json:"reason,omitempty"`
}

type leaveDetailResponse struct {
	Leave leaveResponse `json:"leave"`
}

type leaveListResponse struct {
	Leaves []leaveResponse `json:"leaves"`
}

type leavePoolResponse struct {
	Leaves     []leaveResponse `json:"leaves"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	TotalCount int             `json:"total_count"`
}

type leaveResponse struct {
	ID                   int64                      `json:"id"`
	UserID               int64                      `json:"user_id"`
	PublicationID        int64                      `json:"publication_id"`
	ShiftChangeRequestID int64                      `json:"shift_change_request_id"`
	Category             string                     `json:"category"`
	Reason               string                     `json:"reason"`
	State                string                     `json:"state"`
	ShareURL             string                     `json:"share_url"`
	CreatedAt            string                     `json:"created_at"`
	UpdatedAt            string                     `json:"updated_at"`
	Request              shiftChangeRequestResponse `json:"request"`
	RequesterName        string                     `json:"requester_name,omitempty"`
	CounterpartName      *string                    `json:"counterpart_name,omitempty"`
	SubstituteName       *string                    `json:"substitute_name,omitempty"`
	Shift                *leaveShiftResponse        `json:"shift,omitempty"`
	Urgency              *leaveUrgencyResponse      `json:"urgency,omitempty"`
	Actions              leaveActionsResponse       `json:"actions"`
}

type leaveShiftResponse struct {
	AssignmentID    int64  `json:"assignment_id"`
	SlotID          int64  `json:"slot_id"`
	Weekday         int    `json:"weekday"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	PositionID      int64  `json:"position_id"`
	PositionName    string `json:"position_name"`
	OccurrenceStart string `json:"occurrence_start"`
	OccurrenceEnd   string `json:"occurrence_end"`
}

type leaveUrgencyResponse struct {
	OccurrenceStart     string `json:"occurrence_start"`
	SecondsUntilStart   int64  `json:"seconds_until_start"`
	StartsWithin24Hours bool   `json:"starts_within_24_hours"`
}

type leaveActionsResponse struct {
	CanClaim       bool   `json:"can_claim"`
	CanApprove     bool   `json:"can_approve"`
	CanReject      bool   `json:"can_reject"`
	CanCancel      bool   `json:"can_cancel"`
	DisabledReason string `json:"disabled_reason,omitempty"`
}

type leavePreviewResponse struct {
	Occurrences []leavePreviewOccurrenceResponse `json:"occurrences"`
}

type leavePreviewOccurrenceResponse struct {
	AssignmentID     int64                          `json:"assignment_id"`
	OccurrenceDate   string                         `json:"occurrence_date"`
	Slot             leavePreviewSlotResponse       `json:"slot"`
	Position         leavePreviewPositionResponse   `json:"position"`
	OccurrenceStart  string                         `json:"occurrence_start"`
	OccurrenceEnd    string                         `json:"occurrence_end"`
	DirectCandidates []leaveDirectCandidateResponse `json:"direct_candidates"`
}

type leavePreviewSlotResponse struct {
	ID        int64  `json:"id"`
	Weekday   int    `json:"weekday"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type leavePreviewPositionResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type leaveDirectCandidateResponse struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}

func (h *LeaveHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}

	var req createLeaveRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	occurrenceDate, err := parseOccurrenceDate(req.OccurrenceDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid occurrence date")
		return
	}

	detail, err := h.leaveService.Create(r.Context(), service.CreateLeaveInput{
		UserID:            user.ID,
		AssignmentID:      req.AssignmentID,
		OccurrenceDate:    occurrenceDate,
		Type:              model.ShiftChangeType(req.Type),
		CounterpartUserID: req.CounterpartUserID,
		Category:          model.LeaveCategory(req.Category),
		Reason:            req.Reason,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}

	writeData(w, http.StatusCreated, leaveDetailResponse{Leave: newLeaveResponse(detail)})
}

func (h *LeaveHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	leaveID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid leave id")
		return
	}

	detail, err := h.leaveService.GetByID(r.Context(), leaveID, user.ID, user.IsAdmin)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, leaveDetailResponse{Leave: newLeaveResponse(detail)})
}

func (h *LeaveHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	leaveID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid leave id")
		return
	}

	if err := h.leaveService.Cancel(r.Context(), leaveID, user.ID); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *LeaveHandler) ListPool(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	page, pageSize, err := parsePageQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid pagination")
		return
	}
	result, err := h.leaveService.ListPool(r.Context(), user.ID, user.IsAdmin, service.ListLeavePoolInput{
		State:    r.URL.Query().Get("state"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, leavePoolResponse{
		Leaves:     newLeaveResponses(result.Leaves),
		Page:       result.Page,
		PageSize:   result.PageSize,
		TotalCount: result.TotalCount,
	})
}

func (h *LeaveHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	page, pageSize, err := parsePageQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid pagination")
		return
	}

	rows, err := h.leaveService.ListForUser(r.Context(), user.ID, service.ListLeavesInput{Page: page, PageSize: pageSize})
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, leaveListResponse{Leaves: newLeaveResponses(rows)})
}

func (h *LeaveHandler) PreviewMine(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	from, err := parseOccurrenceDate(r.URL.Query().Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid from date")
		return
	}
	to, err := parseOccurrenceDate(r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid to date")
		return
	}

	rows, err := h.leaveService.PreviewOccurrences(r.Context(), user.ID, from, to)
	if err != nil {
		h.writeError(w, err)
		return
	}
	out := make([]leavePreviewOccurrenceResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, newLeavePreviewOccurrenceResponse(row))
	}
	writeData(w, http.StatusOK, leavePreviewResponse{Occurrences: out})
}

func (h *LeaveHandler) ListForPublication(w http.ResponseWriter, r *http.Request) {
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}
	page, pageSize, err := parsePageQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid pagination")
		return
	}

	rows, err := h.leaveService.ListForPublication(r.Context(), publicationID, service.ListLeavesInput{Page: page, PageSize: pageSize})
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, leaveListResponse{Leaves: newLeaveResponses(rows)})
}

func (h *LeaveHandler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
	case errors.Is(err, service.ErrInvalidOccurrenceDate):
		writeError(w, http.StatusBadRequest, "INVALID_OCCURRENCE_DATE", "Invalid occurrence date")
	case errors.Is(err, service.ErrShiftChangeInvalidType):
		writeError(w, http.StatusBadRequest, "SHIFT_CHANGE_INVALID_TYPE", "Invalid leave type")
	case errors.Is(err, service.ErrShiftChangeSelf):
		writeError(w, http.StatusBadRequest, "SHIFT_CHANGE_SELF", "Cannot target yourself")
	case errors.Is(err, service.ErrLeaveNotOwner):
		writeError(w, http.StatusForbidden, "LEAVE_NOT_OWNER", "Not authorized for this leave")
	case errors.Is(err, service.ErrShiftChangeNotOwner):
		writeError(w, http.StatusForbidden, "SHIFT_CHANGE_NOT_OWNER", "Not authorized for this request")
	case errors.Is(err, service.ErrShiftChangeNotQualified):
		writeError(w, http.StatusForbidden, "SHIFT_CHANGE_NOT_QUALIFIED", "Not qualified for the involved shift")
	case errors.Is(err, service.ErrNotQualified):
		writeError(w, http.StatusForbidden, "NOT_QUALIFIED", "Not qualified")
	case errors.Is(err, service.ErrUserDisabled):
		writeError(w, http.StatusConflict, "USER_DISABLED", "User is disabled")
	case errors.Is(err, service.ErrPublicationNotActive):
		writeError(w, http.StatusConflict, "PUBLICATION_NOT_ACTIVE", "Publication is not active")
	case errors.Is(err, service.ErrLeaveNotFound):
		writeError(w, http.StatusNotFound, "LEAVE_NOT_FOUND", "Leave not found")
	case errors.Is(err, service.ErrShiftChangeNotFound):
		writeError(w, http.StatusNotFound, "SHIFT_CHANGE_NOT_FOUND", "Request not found")
	case errors.Is(err, service.ErrSchedulingRetryable):
		writeError(w, http.StatusServiceUnavailable, "SCHEDULING_RETRYABLE", "Scheduling conflict, please retry")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}

func newLeaveResponse(detail *service.LeaveDetail) leaveResponse {
	leave := detail.Leave
	resp := leaveResponse{
		ID:                   leave.ID,
		UserID:               leave.UserID,
		PublicationID:        leave.PublicationID,
		ShiftChangeRequestID: leave.ShiftChangeRequestID,
		Category:             string(leave.Category),
		Reason:               leave.Reason,
		State:                string(detail.State),
		ShareURL:             "/leaves/" + strconv.FormatInt(leave.ID, 10),
		CreatedAt:            leave.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:            leave.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Request:              newShiftChangeRequestResponse(detail.Request),
		RequesterName:        detail.RequesterName,
		CounterpartName:      detail.CounterpartName,
		SubstituteName:       detail.SubstituteName,
		Actions: leaveActionsResponse{
			CanClaim:       detail.Actions.CanClaim,
			CanApprove:     detail.Actions.CanApprove,
			CanReject:      detail.Actions.CanReject,
			CanCancel:      detail.Actions.CanCancel,
			DisabledReason: string(detail.Actions.DisabledReason),
		},
	}
	if detail.Shift != nil {
		resp.Shift = &leaveShiftResponse{
			AssignmentID:    detail.Shift.AssignmentID,
			SlotID:          detail.Shift.SlotID,
			Weekday:         detail.Shift.Weekday,
			StartTime:       detail.Shift.StartTime,
			EndTime:         detail.Shift.EndTime,
			PositionID:      detail.Shift.PositionID,
			PositionName:    detail.Shift.PositionName,
			OccurrenceStart: detail.Shift.OccurrenceStart.Format("2006-01-02T15:04:05Z07:00"),
			OccurrenceEnd:   detail.Shift.OccurrenceEnd.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	if detail.Urgency != nil {
		resp.Urgency = &leaveUrgencyResponse{
			OccurrenceStart:     detail.Urgency.OccurrenceStart.Format("2006-01-02T15:04:05Z07:00"),
			SecondsUntilStart:   detail.Urgency.SecondsUntilStart,
			StartsWithin24Hours: detail.Urgency.StartsWithin24Hours,
		}
	}
	return resp
}

func newLeaveResponses(rows []*service.LeaveDetail) []leaveResponse {
	out := make([]leaveResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, newLeaveResponse(row))
	}
	return out
}

func newLeavePreviewOccurrenceResponse(row *service.OccurrencePreview) leavePreviewOccurrenceResponse {
	resp := leavePreviewOccurrenceResponse{
		AssignmentID:   row.AssignmentID,
		OccurrenceDate: row.OccurrenceDate.Format("2006-01-02"),
		Slot: leavePreviewSlotResponse{
			ID:        row.Slot.ID,
			Weekday:   publicationSlotWeekday(row.Slot),
			StartTime: row.Slot.StartTime,
			EndTime:   row.Slot.EndTime,
		},
		Position: leavePreviewPositionResponse{
			ID:   row.Position.ID,
			Name: row.Position.Name,
		},
		OccurrenceStart: row.OccurrenceStart.Format("2006-01-02T15:04:05Z07:00"),
		OccurrenceEnd:   row.OccurrenceEnd.Format("2006-01-02T15:04:05Z07:00"),
	}
	resp.DirectCandidates = make([]leaveDirectCandidateResponse, 0, len(row.DirectCandidates))
	for _, candidate := range row.DirectCandidates {
		resp.DirectCandidates = append(resp.DirectCandidates, leaveDirectCandidateResponse{
			UserID: candidate.UserID,
			Name:   candidate.Name,
		})
	}
	return resp
}

func parsePageQuery(r *http.Request) (int, int, error) {
	page, err := parseOptionalInt(r.URL.Query().Get("page"))
	if err != nil {
		return 0, 0, err
	}
	pageSize, err := parseOptionalInt(r.URL.Query().Get("page_size"))
	if err != nil {
		return 0, 0, err
	}
	return page, pageSize, nil
}
