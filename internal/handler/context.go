package handler

import "net/http"

type contextKey string

const (
	contextKeyUserID  contextKey = "userID"
	contextKeyIsAdmin contextKey = "isAdmin"
)

func (h *Handler) getUserID(r *http.Request) string {
	userID, ok := r.Context().Value(contextKeyUserID).(string)
	if !ok {
		panic("getUserID called without auth middleware")
	}
	return userID
}

func (h *Handler) isAdmin(r *http.Request) bool {
	isAdmin, ok := r.Context().Value(contextKeyIsAdmin).(bool)
	if !ok {
		panic("isAdmin called without auth middleware")
	}
	return isAdmin
}
