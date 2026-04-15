package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

type templateService interface {
	ListTemplates(ctx context.Context, input service.ListTemplatesInput) (*service.ListTemplatesResult, error)
	CreateTemplate(ctx context.Context, input service.CreateTemplateInput) (*model.Template, error)
	GetTemplateByID(ctx context.Context, id int64) (*model.Template, error)
	UpdateTemplate(ctx context.Context, input service.UpdateTemplateInput) (*model.Template, error)
	DeleteTemplate(ctx context.Context, id int64) error
	CloneTemplate(ctx context.Context, id int64) (*model.Template, error)
	CreateTemplateShift(ctx context.Context, input service.CreateTemplateShiftInput) (*model.TemplateShift, error)
	UpdateTemplateShift(ctx context.Context, input service.UpdateTemplateShiftInput) (*model.TemplateShift, error)
	DeleteTemplateShift(ctx context.Context, templateID, shiftID int64) error
}

type TemplateHandler struct {
	templateService templateService
}

type templatesResponse struct {
	Templates  []templateListResponse `json:"templates"`
	Pagination paginationResponse     `json:"pagination"`
}

type templateDetailResponse struct {
	Template templateResponse `json:"template"`
}

type templateShiftDetailResponse struct {
	Shift templateShiftResponse `json:"shift"`
}

type createTemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type replaceTemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type templateShiftRequest struct {
	Weekday           int    `json:"weekday"`
	StartTime         string `json:"start_time"`
	EndTime           string `json:"end_time"`
	PositionID        int64  `json:"position_id"`
	RequiredHeadcount int    `json:"required_headcount"`
}

func NewTemplateHandler(templateService templateService) *TemplateHandler {
	return &TemplateHandler{templateService: templateService}
}

func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.templateService.ListTemplates(r.Context(), service.ListTemplatesInput{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	templates := make([]templateListResponse, 0, len(result.Templates))
	for _, template := range result.Templates {
		templates = append(templates, newTemplateListResponse(template))
	}

	writeData(w, http.StatusOK, templatesResponse{
		Templates: templates,
		Pagination: paginationResponse{
			Page:       result.Page,
			PageSize:   result.PageSize,
			Total:      result.Total,
			TotalPages: result.TotalPages,
		},
	})
}

func (h *TemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createTemplateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	template, err := h.templateService.CreateTemplate(r.Context(), service.CreateTemplateInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	writeData(w, http.StatusCreated, templateDetailResponse{
		Template: newTemplateResponse(template),
	})
}

func (h *TemplateHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template id")
		return
	}

	template, err := h.templateService.GetTemplateByID(r.Context(), id)
	if err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, templateDetailResponse{
		Template: newTemplateResponse(template),
	})
}

func (h *TemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template id")
		return
	}

	var req replaceTemplateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	template, err := h.templateService.UpdateTemplate(r.Context(), service.UpdateTemplateInput{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, templateDetailResponse{
		Template: newTemplateResponse(template),
	})
}

func (h *TemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template id")
		return
	}

	if err := h.templateService.DeleteTemplate(r.Context(), id); err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TemplateHandler) Clone(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template id")
		return
	}

	template, err := h.templateService.CloneTemplate(r.Context(), id)
	if err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	writeData(w, http.StatusCreated, templateDetailResponse{
		Template: newTemplateResponse(template),
	})
}

func (h *TemplateHandler) CreateShift(w http.ResponseWriter, r *http.Request) {
	templateID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template id")
		return
	}

	var req templateShiftRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	shift, err := h.templateService.CreateTemplateShift(r.Context(), service.CreateTemplateShiftInput{
		TemplateID:        templateID,
		Weekday:           req.Weekday,
		StartTime:         req.StartTime,
		EndTime:           req.EndTime,
		PositionID:        req.PositionID,
		RequiredHeadcount: req.RequiredHeadcount,
	})
	if err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	writeData(w, http.StatusCreated, templateShiftDetailResponse{
		Shift: newTemplateShiftResponse(shift),
	})
}

func (h *TemplateHandler) UpdateShift(w http.ResponseWriter, r *http.Request) {
	templateID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template id")
		return
	}

	shiftID, err := parsePathID(r, "shift_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template shift id")
		return
	}

	var req templateShiftRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	shift, err := h.templateService.UpdateTemplateShift(r.Context(), service.UpdateTemplateShiftInput{
		TemplateID:        templateID,
		ShiftID:           shiftID,
		Weekday:           req.Weekday,
		StartTime:         req.StartTime,
		EndTime:           req.EndTime,
		PositionID:        req.PositionID,
		RequiredHeadcount: req.RequiredHeadcount,
	})
	if err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, templateShiftDetailResponse{
		Shift: newTemplateShiftResponse(shift),
	})
}

func (h *TemplateHandler) DeleteShift(w http.ResponseWriter, r *http.Request) {
	templateID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template id")
		return
	}

	shiftID, err := parsePathID(r, "shift_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template shift id")
		return
	}

	if err := h.templateService.DeleteTemplateShift(r.Context(), templateID, shiftID); err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TemplateHandler) writeTemplateServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
	case errors.Is(err, service.ErrTemplateNotFound):
		writeError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", "Template not found")
	case errors.Is(err, service.ErrTemplateLocked):
		writeError(w, http.StatusConflict, "TEMPLATE_LOCKED", "Template is locked")
	case errors.Is(err, service.ErrTemplateShiftNotFound):
		writeError(w, http.StatusNotFound, "TEMPLATE_SHIFT_NOT_FOUND", "Template shift not found")
	case errors.Is(err, service.ErrInvalidShiftTime):
		writeError(w, http.StatusBadRequest, "INVALID_SHIFT_TIME", "Shift end time must be after start time")
	case errors.Is(err, service.ErrInvalidWeekday):
		writeError(w, http.StatusBadRequest, "INVALID_WEEKDAY", "Weekday must be between 1 and 7")
	case errors.Is(err, service.ErrInvalidHeadcount):
		writeError(w, http.StatusBadRequest, "INVALID_HEADCOUNT", "Required headcount must be positive")
	case errors.Is(err, service.ErrPositionNotFound):
		writeError(w, http.StatusNotFound, "POSITION_NOT_FOUND", "Position not found")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}
