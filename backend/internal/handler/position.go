package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

type positionService interface {
	ListPositions(ctx context.Context, input service.ListPositionsInput) (*service.ListPositionsResult, error)
	GetPositionByID(ctx context.Context, id int64) (*model.Position, error)
	CreatePosition(ctx context.Context, input service.CreatePositionInput) (*model.Position, error)
	UpdatePosition(ctx context.Context, input service.UpdatePositionInput) (*model.Position, error)
	DeletePosition(ctx context.Context, id int64) error
}

type PositionHandler struct {
	positionService positionService
}

type positionsResponse struct {
	Positions  []positionResponse `json:"positions"`
	Pagination paginationResponse `json:"pagination"`
}

type positionDetailResponse struct {
	Position positionResponse `json:"position"`
}

type createPositionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updatePositionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func NewPositionHandler(positionService positionService) *PositionHandler {
	return &PositionHandler{positionService: positionService}
}

func (h *PositionHandler) List(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.positionService.ListPositions(r.Context(), service.ListPositionsInput{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		h.writePositionServiceError(w, err)
		return
	}

	positions := make([]positionResponse, 0, len(result.Positions))
	for _, position := range result.Positions {
		positions = append(positions, newPositionResponse(position))
	}

	writeData(w, http.StatusOK, positionsResponse{
		Positions: positions,
		Pagination: paginationResponse{
			Page:       result.Page,
			PageSize:   result.PageSize,
			Total:      result.Total,
			TotalPages: result.TotalPages,
		},
	})
}

func (h *PositionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createPositionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	position, err := h.positionService.CreatePosition(r.Context(), service.CreatePositionInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		h.writePositionServiceError(w, err)
		return
	}

	writeData(w, http.StatusCreated, positionDetailResponse{
		Position: newPositionResponse(position),
	})
}

func (h *PositionHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid position id")
		return
	}

	position, err := h.positionService.GetPositionByID(r.Context(), id)
	if err != nil {
		h.writePositionServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, positionDetailResponse{
		Position: newPositionResponse(position),
	})
}

func (h *PositionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid position id")
		return
	}

	var req updatePositionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	position, err := h.positionService.UpdatePosition(r.Context(), service.UpdatePositionInput{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		h.writePositionServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, positionDetailResponse{
		Position: newPositionResponse(position),
	})
}

func (h *PositionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid position id")
		return
	}

	if err := h.positionService.DeletePosition(r.Context(), id); err != nil {
		h.writePositionServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PositionHandler) writePositionServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
	case errors.Is(err, service.ErrPositionInUse):
		writeError(w, http.StatusConflict, "POSITION_IN_USE", "Position is in use")
	case errors.Is(err, service.ErrPositionNotFound):
		writeError(w, http.StatusNotFound, "POSITION_NOT_FOUND", "Position not found")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}
