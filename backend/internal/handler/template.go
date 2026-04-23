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
	CreateTemplateSlot(ctx context.Context, input service.CreateTemplateSlotInput) (*model.TemplateSlot, error)
	UpdateTemplateSlot(ctx context.Context, input service.UpdateTemplateSlotInput) (*model.TemplateSlot, error)
	DeleteTemplateSlot(ctx context.Context, templateID, slotID int64) error
	CreateTemplateSlotPosition(ctx context.Context, input service.CreateTemplateSlotPositionInput) (*model.TemplateSlotPosition, error)
	UpdateTemplateSlotPosition(ctx context.Context, input service.UpdateTemplateSlotPositionInput) (*model.TemplateSlotPosition, error)
	DeleteTemplateSlotPosition(ctx context.Context, templateID, slotID, slotPositionID int64) error
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

type templateSlotDetailResponse struct {
	Slot templateSlotResponse `json:"slot"`
}

type templateSlotPositionDetailResponse struct {
	Position templateSlotPositionResponse `json:"position"`
}

type createTemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type replaceTemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type templateSlotRequest struct {
	Weekday   int    `json:"weekday"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type templateSlotPositionRequest struct {
	PositionID        int64 `json:"position_id"`
	RequiredHeadcount int   `json:"required_headcount"`
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
	if err := readJSON(w, r, &req); err != nil {
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
	if err := readJSON(w, r, &req); err != nil {
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

func (h *TemplateHandler) CreateSlot(w http.ResponseWriter, r *http.Request) {
	templateID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template id")
		return
	}

	var req templateSlotRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	slot, err := h.templateService.CreateTemplateSlot(r.Context(), service.CreateTemplateSlotInput{
		TemplateID: templateID,
		Weekday:    req.Weekday,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
	})
	if err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	writeData(w, http.StatusCreated, templateSlotDetailResponse{
		Slot: newTemplateSlotResponse(slot),
	})
}

func (h *TemplateHandler) UpdateSlot(w http.ResponseWriter, r *http.Request) {
	templateID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template id")
		return
	}

	slotID, err := parsePathID(r, "slot_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template slot id")
		return
	}

	var req templateSlotRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	slot, err := h.templateService.UpdateTemplateSlot(r.Context(), service.UpdateTemplateSlotInput{
		TemplateID: templateID,
		SlotID:     slotID,
		Weekday:    req.Weekday,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
	})
	if err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, templateSlotDetailResponse{
		Slot: newTemplateSlotResponse(slot),
	})
}

func (h *TemplateHandler) DeleteSlot(w http.ResponseWriter, r *http.Request) {
	templateID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template id")
		return
	}

	slotID, err := parsePathID(r, "slot_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template slot id")
		return
	}

	if err := h.templateService.DeleteTemplateSlot(r.Context(), templateID, slotID); err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TemplateHandler) CreateSlotPosition(w http.ResponseWriter, r *http.Request) {
	templateID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template id")
		return
	}

	slotID, err := parsePathID(r, "slot_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template slot id")
		return
	}

	var req templateSlotPositionRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	slotPosition, err := h.templateService.CreateTemplateSlotPosition(r.Context(), service.CreateTemplateSlotPositionInput{
		TemplateID:        templateID,
		SlotID:            slotID,
		PositionID:        req.PositionID,
		RequiredHeadcount: req.RequiredHeadcount,
	})
	if err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	writeData(w, http.StatusCreated, templateSlotPositionDetailResponse{
		Position: newTemplateSlotPositionResponse(slotPosition),
	})
}

func (h *TemplateHandler) UpdateSlotPosition(w http.ResponseWriter, r *http.Request) {
	templateID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template id")
		return
	}

	slotID, err := parsePathID(r, "slot_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template slot id")
		return
	}

	slotPositionID, err := parsePathID(r, "position_entry_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template slot position id")
		return
	}

	var req templateSlotPositionRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	slotPosition, err := h.templateService.UpdateTemplateSlotPosition(r.Context(), service.UpdateTemplateSlotPositionInput{
		TemplateID:        templateID,
		SlotID:            slotID,
		SlotPositionID:    slotPositionID,
		PositionID:        req.PositionID,
		RequiredHeadcount: req.RequiredHeadcount,
	})
	if err != nil {
		h.writeTemplateServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, templateSlotPositionDetailResponse{
		Position: newTemplateSlotPositionResponse(slotPosition),
	})
}

func (h *TemplateHandler) DeleteSlotPosition(w http.ResponseWriter, r *http.Request) {
	templateID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template id")
		return
	}

	slotID, err := parsePathID(r, "slot_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template slot id")
		return
	}

	slotPositionID, err := parsePathID(r, "position_entry_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid template slot position id")
		return
	}

	if err := h.templateService.DeleteTemplateSlotPosition(r.Context(), templateID, slotID, slotPositionID); err != nil {
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
	case errors.Is(err, service.ErrTemplateSlotOverlap):
		writeError(w, http.StatusConflict, "TEMPLATE_SLOT_OVERLAP", "Template slot overlaps with existing slot")
	case errors.Is(err, service.ErrTemplateSlotNotFound):
		writeError(w, http.StatusNotFound, "TEMPLATE_SLOT_NOT_FOUND", "Template slot not found")
	case errors.Is(err, service.ErrTemplateSlotPositionNotFound):
		writeError(w, http.StatusNotFound, "TEMPLATE_SLOT_POSITION_NOT_FOUND", "Template slot position not found")
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
