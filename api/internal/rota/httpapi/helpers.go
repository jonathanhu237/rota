package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"github.com/jonathanhu237/rota/api/internal/auth/domain"
	"github.com/jonathanhu237/rota/api/internal/rota/model"
	"github.com/jonathanhu237/rota/api/internal/rota/service"
)

type currentUserContextKey struct{}

func WithCurrentUser(ctx context.Context, user *model.User) context.Context {
	return context.WithValue(ctx, currentUserContextKey{}, user)
}

func currentUserFromRequest(r *http.Request) (*model.User, bool) {
	user, ok := r.Context().Value(currentUserContextKey{}).(*model.User)
	return user, ok && user != nil
}

func parsePathUserID(r *http.Request, name string) (string, error) {
	value := r.PathValue(name)
	if !domain.IsCanonicalUUID(value) {
		return "", strconv.ErrSyntax
	}
	return value, nil
}

func parsePathID(r *http.Request, name string) (int64, error) {
	value := r.PathValue(name)
	if value == "" {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseInt(value, 10, 64)
}

func parseOptionalInt(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

func emailLanguageContext(r *http.Request, actor *model.User) context.Context {
	ctx := service.WithEmailRequestLanguage(r.Context(), r.Header.Get("Accept-Language"))
	if actor != nil && actor.LanguagePreference != nil {
		ctx = service.WithEmailActorLanguage(ctx, string(*actor.LanguagePreference))
	}
	return ctx
}
