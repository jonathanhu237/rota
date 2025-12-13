package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// --------------------------------------------------
// JSON
// --------------------------------------------------
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error(err.Error())
	}
}

func (h *Handler) readJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

func (h *Handler) errorResponse(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": message,
		},
	})
}

func (h *Handler) internalServerError(w http.ResponseWriter, err error) {
	h.logger.Error(err.Error())
	h.errorResponse(w, http.StatusInternalServerError, "internal server error")
}

func (h *Handler) invalidRequestBody(w http.ResponseWriter) {
	h.errorResponse(w, http.StatusBadRequest, "invalid request body")
}

func (h *Handler) validationError(w http.ResponseWriter, errors map[string]string) {
	h.writeJSON(w, http.StatusBadRequest, map[string]any{
		"error": map[string]any{
			"message": "validation failed",
			"details": errors,
		},
	})
}

// --------------------------------------------------
// Pagination
// --------------------------------------------------
type Pagination struct {
	Page     int
	PageSize int
}

func (h *Handler) parsePagination(r *http.Request) (Pagination, error) {
	var p Pagination

	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")

	if pageStr == "" {
		p.Page = 1
	} else {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			return Pagination{}, errors.New("invalid page parameter")
		}
		p.Page = page
	}

	if pageSizeStr == "" {
		p.PageSize = 10
	} else {
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 1 {
			return Pagination{}, errors.New("invalid page_size parameter")
		}
		p.PageSize = pageSize
	}

	return p, nil
}
