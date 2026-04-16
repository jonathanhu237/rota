package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

type publicationService interface {
	ListPublications(ctx context.Context, input service.ListPublicationsInput) (*service.ListPublicationsResult, error)
	CreatePublication(ctx context.Context, input service.CreatePublicationInput) (*model.Publication, error)
	GetPublicationByID(ctx context.Context, id int64) (*model.Publication, error)
	DeletePublication(ctx context.Context, id int64) error
	GetCurrentPublication(ctx context.Context) (*model.Publication, error)
	ListAvailabilitySubmissionShiftIDs(ctx context.Context, publicationID, userID int64) ([]int64, error)
	CreateAvailabilitySubmission(ctx context.Context, input service.CreateAvailabilitySubmissionInput) (*model.AvailabilitySubmission, error)
	DeleteAvailabilitySubmission(ctx context.Context, input service.DeleteAvailabilitySubmissionInput) error
	ListQualifiedPublicationShifts(ctx context.Context, publicationID, userID int64) ([]*model.TemplateShift, error)
	GetAssignmentBoard(ctx context.Context, publicationID int64) (*service.AssignmentBoardResult, error)
	AutoAssignPublication(ctx context.Context, publicationID int64) (*service.AssignmentBoardResult, error)
	CreateAssignment(ctx context.Context, input service.CreateAssignmentInput) (*model.Assignment, error)
	DeleteAssignment(ctx context.Context, input service.DeleteAssignmentInput) error
	ActivatePublication(ctx context.Context, publicationID int64) (*model.Publication, error)
	EndPublication(ctx context.Context, publicationID int64) (*model.Publication, error)
	GetPublicationRoster(ctx context.Context, publicationID int64) (*service.RosterResult, error)
	GetCurrentRoster(ctx context.Context) (*service.RosterResult, error)
}

type PublicationHandler struct {
	publicationService publicationService
}

type publicationsResponse struct {
	Publications []publicationResponse `json:"publications"`
	Pagination   paginationResponse    `json:"pagination"`
}

type publicationDetailResponse struct {
	Publication *publicationResponse `json:"publication"`
}

type currentPublicationResponse struct {
	Publication *publicationResponse `json:"publication"`
}

type submissionsMeResponse struct {
	ShiftIDs []int64 `json:"shift_ids"`
}

type shiftsMeResponse struct {
	Shifts []templateShiftResponse `json:"shifts"`
}

type createPublicationRequest struct {
	TemplateID        int64     `json:"template_id"`
	Name              string    `json:"name"`
	SubmissionStartAt time.Time `json:"submission_start_at"`
	SubmissionEndAt   time.Time `json:"submission_end_at"`
	PlannedActiveFrom time.Time `json:"planned_active_from"`
}

type createSubmissionRequest struct {
	TemplateShiftID int64 `json:"template_shift_id"`
}

type createAssignmentRequest struct {
	UserID          int64 `json:"user_id"`
	TemplateShiftID int64 `json:"template_shift_id"`
}

func NewPublicationHandler(publicationService publicationService) *PublicationHandler {
	return &PublicationHandler{publicationService: publicationService}
}

func (h *PublicationHandler) List(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.publicationService.ListPublications(r.Context(), service.ListPublicationsInput{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	publications := make([]publicationResponse, 0, len(result.Publications))
	for _, publication := range result.Publications {
		publications = append(publications, *newPublicationResponse(publication))
	}

	writeData(w, http.StatusOK, publicationsResponse{
		Publications: publications,
		Pagination: paginationResponse{
			Page:       result.Page,
			PageSize:   result.PageSize,
			Total:      result.Total,
			TotalPages: result.TotalPages,
		},
	})
}

func (h *PublicationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createPublicationRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	if req.TemplateID <= 0 || req.Name == "" || req.SubmissionStartAt.IsZero() || req.SubmissionEndAt.IsZero() || req.PlannedActiveFrom.IsZero() {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	publication, err := h.publicationService.CreatePublication(r.Context(), service.CreatePublicationInput{
		TemplateID:        req.TemplateID,
		Name:              req.Name,
		SubmissionStartAt: req.SubmissionStartAt,
		SubmissionEndAt:   req.SubmissionEndAt,
		PlannedActiveFrom: req.PlannedActiveFrom,
	})
	if err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	writeData(w, http.StatusCreated, publicationDetailResponse{
		Publication: newPublicationResponse(publication),
	})
}

func (h *PublicationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	publication, err := h.publicationService.GetPublicationByID(r.Context(), id)
	if err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, publicationDetailResponse{
		Publication: newPublicationResponse(publication),
	})
}

func (h *PublicationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	if err := h.publicationService.DeletePublication(r.Context(), id); err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PublicationHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	publication, err := h.publicationService.GetCurrentPublication(r.Context())
	if err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, currentPublicationResponse{
		Publication: newPublicationResponse(publication),
	})
}

func (h *PublicationHandler) ListMySubmissionShiftIDs(w http.ResponseWriter, r *http.Request) {
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

	shiftIDs, err := h.publicationService.ListAvailabilitySubmissionShiftIDs(r.Context(), publicationID, user.ID)
	if err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, submissionsMeResponse{
		ShiftIDs: append([]int64{}, shiftIDs...),
	})
}

func (h *PublicationHandler) CreateSubmission(w http.ResponseWriter, r *http.Request) {
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

	var req createSubmissionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if _, err := h.publicationService.CreateAvailabilitySubmission(r.Context(), service.CreateAvailabilitySubmissionInput{
		PublicationID:   publicationID,
		UserID:          user.ID,
		TemplateShiftID: req.TemplateShiftID,
	}); err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *PublicationHandler) DeleteSubmission(w http.ResponseWriter, r *http.Request) {
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

	shiftID, err := parsePathID(r, "shift_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template shift id")
		return
	}

	if err := h.publicationService.DeleteAvailabilitySubmission(r.Context(), service.DeleteAvailabilitySubmissionInput{
		PublicationID:   publicationID,
		UserID:          user.ID,
		TemplateShiftID: shiftID,
	}); err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PublicationHandler) ListMyQualifiedShifts(w http.ResponseWriter, r *http.Request) {
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

	shifts, err := h.publicationService.ListQualifiedPublicationShifts(r.Context(), publicationID, user.ID)
	if err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	responseShifts := make([]templateShiftResponse, 0, len(shifts))
	for _, shift := range shifts {
		responseShifts = append(responseShifts, newTemplateShiftResponse(shift))
	}

	writeData(w, http.StatusOK, shiftsMeResponse{
		Shifts: responseShifts,
	})
}

func (h *PublicationHandler) GetAssignmentBoard(w http.ResponseWriter, r *http.Request) {
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	result, err := h.publicationService.GetAssignmentBoard(r.Context(), publicationID)
	if err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, newAssignmentBoardResponse(result))
}

func (h *PublicationHandler) AutoAssign(w http.ResponseWriter, r *http.Request) {
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	result, err := h.publicationService.AutoAssignPublication(r.Context(), publicationID)
	if err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, newAssignmentBoardResponse(result))
}

func (h *PublicationHandler) CreateAssignment(w http.ResponseWriter, r *http.Request) {
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	var req createAssignmentRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	if req.UserID <= 0 || req.TemplateShiftID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if _, err := h.publicationService.CreateAssignment(r.Context(), service.CreateAssignmentInput{
		PublicationID:   publicationID,
		UserID:          req.UserID,
		TemplateShiftID: req.TemplateShiftID,
	}); err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *PublicationHandler) DeleteAssignment(w http.ResponseWriter, r *http.Request) {
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	assignmentID, err := parsePathID(r, "assignment_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid assignment id")
		return
	}

	if err := h.publicationService.DeleteAssignment(r.Context(), service.DeleteAssignmentInput{
		PublicationID: publicationID,
		AssignmentID:  assignmentID,
	}); err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PublicationHandler) Activate(w http.ResponseWriter, r *http.Request) {
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	publication, err := h.publicationService.ActivatePublication(r.Context(), publicationID)
	if err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, publicationDetailResponse{
		Publication: newPublicationResponse(publication),
	})
}

func (h *PublicationHandler) End(w http.ResponseWriter, r *http.Request) {
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	publication, err := h.publicationService.EndPublication(r.Context(), publicationID)
	if err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, publicationDetailResponse{
		Publication: newPublicationResponse(publication),
	})
}

func (h *PublicationHandler) GetRoster(w http.ResponseWriter, r *http.Request) {
	publicationID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid publication id")
		return
	}

	result, err := h.publicationService.GetPublicationRoster(r.Context(), publicationID)
	if err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, newRosterResponse(result))
}

func (h *PublicationHandler) GetCurrentRoster(w http.ResponseWriter, r *http.Request) {
	result, err := h.publicationService.GetCurrentRoster(r.Context())
	if err != nil {
		h.writePublicationServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, newRosterResponse(result))
}

func newAssignmentBoardResponse(result *service.AssignmentBoardResult) assignmentBoardResponse {
	responseShifts := make([]assignmentBoardShiftResponse, 0, len(result.Shifts))
	for _, shiftResult := range result.Shifts {
		candidates := make([]assignmentCandidateResponse, 0, len(shiftResult.Candidates))
		for _, candidate := range shiftResult.Candidates {
			candidates = append(candidates, assignmentCandidateResponse{
				UserID: candidate.UserID,
				Name:   candidate.Name,
				Email:  candidate.Email,
			})
		}

		assignments := make([]assignmentResponse, 0, len(shiftResult.Assignments))
		for _, assignment := range shiftResult.Assignments {
			assignments = append(assignments, assignmentResponse{
				AssignmentID: assignment.AssignmentID,
				UserID:       assignment.UserID,
				Name:         assignment.Name,
				Email:        assignment.Email,
			})
		}

		responseShifts = append(responseShifts, assignmentBoardShiftResponse{
			Shift:       newPublicationShiftResponse(shiftResult.Shift),
			Candidates:  candidates,
			Assignments: assignments,
		})
	}

	return assignmentBoardResponse{
		Publication: newPublicationResponse(result.Publication),
		Shifts:      responseShifts,
	}
}

func newRosterResponse(result *service.RosterResult) rosterResponse {
	if result == nil {
		return rosterResponse{
			Publication: nil,
			Weekdays:    make([]rosterWeekdayResponse, 0),
		}
	}

	weekdays := make([]rosterWeekdayResponse, 0, len(result.Weekdays))
	for _, weekday := range result.Weekdays {
		shifts := make([]rosterShiftResponse, 0, len(weekday.Shifts))
		for _, shiftResult := range weekday.Shifts {
			assignments := make([]rosterAssignmentResponse, 0, len(shiftResult.Assignments))
			for _, assignment := range shiftResult.Assignments {
				assignments = append(assignments, rosterAssignmentResponse{
					UserID: assignment.UserID,
					Name:   assignment.Name,
				})
			}

			shifts = append(shifts, rosterShiftResponse{
				Shift:       newPublicationShiftResponse(shiftResult.Shift),
				Assignments: assignments,
			})
		}

		weekdays = append(weekdays, rosterWeekdayResponse{
			Weekday: weekday.Weekday,
			Shifts:  shifts,
		})
	}

	return rosterResponse{
		Publication: newPublicationResponse(result.Publication),
		Weekdays:    weekdays,
	}
}

func (h *PublicationHandler) writePublicationServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
	case errors.Is(err, service.ErrInvalidPublicationWindow):
		writeError(w, http.StatusBadRequest, "INVALID_PUBLICATION_WINDOW", "Invalid publication window")
	case errors.Is(err, service.ErrPublicationAlreadyExists):
		writeError(w, http.StatusConflict, "PUBLICATION_ALREADY_EXISTS", "Publication already exists")
	case errors.Is(err, service.ErrPublicationNotFound):
		writeError(w, http.StatusNotFound, "PUBLICATION_NOT_FOUND", "Publication not found")
	case errors.Is(err, service.ErrPublicationNotDeletable):
		writeError(w, http.StatusConflict, "PUBLICATION_NOT_DELETABLE", "Publication is not deletable")
	case errors.Is(err, service.ErrPublicationNotCollecting):
		writeError(w, http.StatusConflict, "PUBLICATION_NOT_COLLECTING", "Publication is not collecting submissions")
	case errors.Is(err, service.ErrPublicationNotAssigning):
		writeError(w, http.StatusConflict, "PUBLICATION_NOT_ASSIGNING", "Publication is not assigning")
	case errors.Is(err, service.ErrPublicationNotActive):
		writeError(w, http.StatusConflict, "PUBLICATION_NOT_ACTIVE", "Publication is not active")
	case errors.Is(err, service.ErrTemplateNotFound):
		writeError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", "Template not found")
	case errors.Is(err, service.ErrTemplateShiftNotFound):
		writeError(w, http.StatusNotFound, "TEMPLATE_SHIFT_NOT_FOUND", "Template shift not found")
	case errors.Is(err, service.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
	case errors.Is(err, service.ErrUserDisabled):
		writeError(w, http.StatusConflict, "USER_DISABLED", "User is disabled")
	case errors.Is(err, service.ErrNotQualified):
		writeError(w, http.StatusForbidden, "NOT_QUALIFIED", "User is not qualified for this shift")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}
