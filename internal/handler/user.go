package handler

import "net/http"

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	pagination, err := h.parsePagination(r)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	users, total, err := h.userService.List(r.Context(), pagination.Page, pagination.PageSize)
	if err != nil {
		h.internalServerError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"users": users,
		"pagination": map[string]any{
			"page":      pagination.Page,
			"page_size": pagination.PageSize,
			"total":     total,
		},
	})
}
