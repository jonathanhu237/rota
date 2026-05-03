package service

import (
	"context"
	"strings"

	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/model"
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

func resolveInvitationEmailLanguage(ctx context.Context, invitee *model.User) string {
	return email.ResolveLanguage(
		userEmailLanguagePreference(invitee),
		actorEmailLanguage(ctx),
		requestEmailLanguage(ctx),
	)
}

func resolveRequestEmailLanguage(ctx context.Context, recipient *model.User) string {
	return email.ResolveLanguage(userEmailLanguagePreference(recipient), requestEmailLanguage(ctx))
}

func resolveSystemEmailLanguage(recipient *model.User) string {
	return email.ResolveLanguage(userEmailLanguagePreference(recipient))
}
