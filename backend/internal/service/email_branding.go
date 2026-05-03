package service

import (
	"context"

	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/model"
)

type emailBrandingProvider interface {
	GetBranding(ctx context.Context) (*model.Branding, error)
}

func resolveEmailBranding(ctx context.Context, provider emailBrandingProvider) (email.Branding, error) {
	if provider == nil {
		return email.DefaultBranding(), nil
	}

	branding, err := provider.GetBranding(ctx)
	if err != nil {
		return email.Branding{}, err
	}
	if branding == nil {
		return email.DefaultBranding(), nil
	}
	return email.NormalizeBranding(email.Branding{
		ProductName:      branding.ProductName,
		OrganizationName: branding.OrganizationName,
	}), nil
}
