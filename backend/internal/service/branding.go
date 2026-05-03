package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

const (
	maxBrandingProductNameLength      = 60
	maxBrandingOrganizationNameLength = 100
)

type brandingRepository interface {
	Get(ctx context.Context) (*model.Branding, error)
	Update(ctx context.Context, params repository.UpdateBrandingParams) (*model.Branding, error)
}

type BrandingService struct {
	brandingRepo brandingRepository
}

type UpdateBrandingInput struct {
	ProductName      string
	OrganizationName string
	Version          int
}

func NewBrandingService(brandingRepo brandingRepository) *BrandingService {
	return &BrandingService{brandingRepo: brandingRepo}
}

func (s *BrandingService) GetBranding(ctx context.Context) (*model.Branding, error) {
	branding, err := s.brandingRepo.Get(ctx)
	if err != nil {
		return nil, err
	}
	return normalizeBrandingModel(branding), nil
}

func (s *BrandingService) UpdateBranding(ctx context.Context, input UpdateBrandingInput) (*model.Branding, error) {
	productName, organizationName, err := normalizeBrandingInput(input.ProductName, input.OrganizationName)
	if err != nil {
		return nil, err
	}
	if input.Version <= 0 {
		return nil, ErrInvalidInput
	}

	branding, err := s.brandingRepo.Update(ctx, repository.UpdateBrandingParams{
		ProductName:      productName,
		OrganizationName: organizationName,
		Version:          input.Version,
	})
	if err != nil {
		return nil, mapBrandingRepositoryError(err)
	}
	return normalizeBrandingModel(branding), nil
}

func normalizeBrandingInput(productName, organizationName string) (string, string, error) {
	normalizedProductName := strings.TrimSpace(productName)
	if normalizedProductName == "" ||
		utf8.RuneCountInString(normalizedProductName) > maxBrandingProductNameLength {
		return "", "", ErrInvalidInput
	}

	normalizedOrganizationName := strings.TrimSpace(organizationName)
	if utf8.RuneCountInString(normalizedOrganizationName) > maxBrandingOrganizationNameLength {
		return "", "", ErrInvalidInput
	}

	return normalizedProductName, normalizedOrganizationName, nil
}

func normalizeBrandingModel(branding *model.Branding) *model.Branding {
	if branding == nil {
		return &model.Branding{
			ProductName:      model.DefaultProductName,
			OrganizationName: model.DefaultOrganizationName,
			Version:          1,
		}
	}
	normalized := *branding
	normalized.ProductName = strings.TrimSpace(normalized.ProductName)
	if normalized.ProductName == "" {
		normalized.ProductName = model.DefaultProductName
	}
	normalized.OrganizationName = strings.TrimSpace(normalized.OrganizationName)
	if normalized.Version <= 0 {
		normalized.Version = 1
	}
	return &normalized
}

func mapBrandingRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrVersionConflict):
		return ErrVersionConflict
	default:
		return err
	}
}
