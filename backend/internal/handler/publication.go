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
	case errors.Is(err, service.ErrTemplateNotFound):
		writeError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", "Template not found")
	case errors.Is(err, service.ErrTemplateShiftNotFound):
		writeError(w, http.StatusNotFound, "TEMPLATE_SHIFT_NOT_FOUND", "Template shift not found")
	case errors.Is(err, service.ErrNotQualified):
		writeError(w, http.StatusForbidden, "NOT_QUALIFIED", "User is not qualified for this shift")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}
