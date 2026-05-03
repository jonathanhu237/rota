package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

func TestBrandingServiceGetNormalizesDefaults(t *testing.T) {
	t.Parallel()

	service := NewBrandingService(&brandingRepositoryMock{
		getFunc: func(ctx context.Context) (*model.Branding, error) {
			return &model.Branding{ProductName: "  ", OrganizationName: "  ", Version: 0}, nil
		},
	})

	branding, err := service.GetBranding(context.Background())
	if err != nil {
		t.Fatalf("GetBranding returned error: %v", err)
	}
	if branding.ProductName != "Rota" || branding.OrganizationName != "" || branding.Version != 1 {
		t.Fatalf("unexpected branding: %+v", branding)
	}
}

func TestBrandingServiceUpdateValidatesAndTrims(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC)
	var got repository.UpdateBrandingParams
	service := NewBrandingService(&brandingRepositoryMock{
		updateFunc: func(ctx context.Context, params repository.UpdateBrandingParams) (*model.Branding, error) {
			got = params
			return &model.Branding{
				ProductName:      params.ProductName,
				OrganizationName: params.OrganizationName,
				Version:          params.Version + 1,
				CreatedAt:        now,
				UpdatedAt:        now,
			}, nil
		},
	})

	branding, err := service.UpdateBranding(context.Background(), UpdateBrandingInput{
		ProductName:      "  排班系统  ",
		OrganizationName: "  Acme  ",
		Version:          2,
	})
	if err != nil {
		t.Fatalf("UpdateBranding returned error: %v", err)
	}
	if got.ProductName != "排班系统" || got.OrganizationName != "Acme" || got.Version != 2 {
		t.Fatalf("unexpected repository params: %+v", got)
	}
	if branding.ProductName != "排班系统" || branding.OrganizationName != "Acme" || branding.Version != 3 {
		t.Fatalf("unexpected branding: %+v", branding)
	}
}

func TestBrandingServiceUpdateRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	service := NewBrandingService(&brandingRepositoryMock{})
	longProduct := makeString("a", 61)
	longOrganization := makeString("企", 101)

	tests := []struct {
		name  string
		input UpdateBrandingInput
	}{
		{name: "blank product", input: UpdateBrandingInput{ProductName: " ", Version: 1}},
		{name: "long product", input: UpdateBrandingInput{ProductName: longProduct, Version: 1}},
		{name: "long organization", input: UpdateBrandingInput{ProductName: "Rota", OrganizationName: longOrganization, Version: 1}},
		{name: "missing version", input: UpdateBrandingInput{ProductName: "Rota"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := service.UpdateBranding(context.Background(), tt.input)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestBrandingServiceUpdateMapsVersionConflict(t *testing.T) {
	t.Parallel()

	service := NewBrandingService(&brandingRepositoryMock{
		updateFunc: func(ctx context.Context, params repository.UpdateBrandingParams) (*model.Branding, error) {
			return nil, repository.ErrVersionConflict
		},
	})

	_, err := service.UpdateBranding(context.Background(), UpdateBrandingInput{
		ProductName: "Rota",
		Version:     1,
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
}

type brandingRepositoryMock struct {
	getFunc    func(ctx context.Context) (*model.Branding, error)
	updateFunc func(ctx context.Context, params repository.UpdateBrandingParams) (*model.Branding, error)
}

func (m *brandingRepositoryMock) Get(ctx context.Context) (*model.Branding, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx)
	}
	return &model.Branding{ProductName: "Rota", Version: 1}, nil
}

func (m *brandingRepositoryMock) Update(ctx context.Context, params repository.UpdateBrandingParams) (*model.Branding, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, params)
	}
	return nil, nil
}

func makeString(value string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += value
	}
	return result
}
