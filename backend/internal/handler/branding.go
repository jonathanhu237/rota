package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

type brandingService interface {
	GetBranding(ctx context.Context) (*model.Branding, error)
	UpdateBranding(ctx context.Context, input service.UpdateBrandingInput) (*model.Branding, error)
}

type BrandingHandler struct {
	brandingService brandingService
}

type brandingResponse struct {
	ProductName      string    `json:"product_name"`
	OrganizationName string    `json:"organization_name"`
	Version          int       `json:"version"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type updateBrandingRequest struct {
	ProductName      string `json:"product_name"`
	OrganizationName string `json:"organization_name"`
	Version          int    `json:"version"`
}

func NewBrandingHandler(brandingService brandingService) *BrandingHandler {
	return &BrandingHandler{brandingService: brandingService}
}

func (h *BrandingHandler) Get(w http.ResponseWriter, r *http.Request) {
	branding, err := h.brandingService.GetBranding(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}

	writeData(w, http.StatusOK, newBrandingResponse(branding))
}

func (h *BrandingHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req updateBrandingRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	branding, err := h.brandingService.UpdateBranding(r.Context(), service.UpdateBrandingInput{
		ProductName:      req.ProductName,
		OrganizationName: req.OrganizationName,
		Version:          req.Version,
	})
	if err != nil {
		h.writeBrandingServiceError(w, err)
		return
	}

	writeData(w, http.StatusOK, newBrandingResponse(branding))
}

func (h *BrandingHandler) writeBrandingServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
	case errors.Is(err, service.ErrVersionConflict):
		writeError(w, http.StatusConflict, "VERSION_CONFLICT", "Branding has been updated by another request")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}

func newBrandingResponse(branding *model.Branding) brandingResponse {
	return brandingResponse{
		ProductName:      branding.ProductName,
		OrganizationName: branding.OrganizationName,
		Version:          branding.Version,
		CreatedAt:        branding.CreatedAt,
		UpdatedAt:        branding.UpdatedAt,
	}
}
