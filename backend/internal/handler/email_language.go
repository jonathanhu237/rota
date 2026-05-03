package handler

import (
	"context"
	"net/http"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

func emailLanguageContext(r *http.Request, actor *model.User) context.Context {
	ctx := service.WithEmailRequestLanguage(r.Context(), r.Header.Get("Accept-Language"))
	if actor != nil && actor.LanguagePreference != nil {
		ctx = service.WithEmailActorLanguage(ctx, string(*actor.LanguagePreference))
	}
	return ctx
}
