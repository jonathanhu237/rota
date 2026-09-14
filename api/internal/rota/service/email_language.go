package service

import (
	"context"
	"strings"

	"github.com/jonathanhu237/rota/api/internal/rota/email"
	"github.com/jonathanhu237/rota/api/internal/rota/model"
)

type emailRequestLanguageContextKey struct{}
type emailActorLanguageContextKey struct{}

func WithEmailRequestLanguage(ctx context.Context, acceptLanguage string) context.Context {
	return context.WithValue(ctx, emailRequestLanguageContextKey{}, email.ParseAcceptLanguage(acceptLanguage))
}

func WithEmailActorLanguage(ctx context.Context, language string) context.Context {
	if strings.TrimSpace(language) == "" {
		return ctx
	}
	return context.WithValue(ctx, emailActorLanguageContextKey{}, email.NormalizeLanguage(language))
}

func requestEmailLanguage(ctx context.Context) string {
	if value, ok := ctx.Value(emailRequestLanguageContextKey{}).(string); ok {
		return value
	}
	return ""
}

func actorEmailLanguage(ctx context.Context) string {
	if value, ok := ctx.Value(emailActorLanguageContextKey{}).(string); ok {
		return value
	}
	return ""
}

func userEmailLanguagePreference(user *model.User) string {
	if user == nil || user.LanguagePreference == nil {
		return ""
	}
	return string(*user.LanguagePreference)
}

func resolveRequestEmailLanguage(ctx context.Context, recipient *model.User) string {
	return email.ResolveLanguage(userEmailLanguagePreference(recipient), requestEmailLanguage(ctx))
}

func resolveSystemEmailLanguage(recipient *model.User) string {
	return email.ResolveLanguage(userEmailLanguagePreference(recipient))
}
