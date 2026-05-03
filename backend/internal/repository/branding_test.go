package repository

import (
	"testing"
	"time"
)

func TestScanBranding(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC)
	branding := defaultBranding()
	err := scanBranding(brandingScannerStub{
		productName:      "排班系统",
		organizationName: "Acme",
		version:          3,
		createdAt:        now,
		updatedAt:        now.Add(time.Minute),
	}, branding)
	if err != nil {
		t.Fatalf("scanBranding returned error: %v", err)
	}

	if branding.ProductName != "排班系统" ||
		branding.OrganizationName != "Acme" ||
		branding.Version != 3 ||
		!branding.CreatedAt.Equal(now) ||
		!branding.UpdatedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("unexpected branding: %+v", branding)
	}
}

func TestDefaultBranding(t *testing.T) {
	t.Parallel()

	branding := defaultBranding()
	if branding.ProductName != "Rota" ||
		branding.OrganizationName != "" ||
		branding.Version != 1 {
		t.Fatalf("unexpected default branding: %+v", branding)
	}
}

type brandingScannerStub struct {
	productName      string
	organizationName string
	version          int
	createdAt        time.Time
	updatedAt        time.Time
}

func (s brandingScannerStub) Scan(dest ...any) error {
	*(dest[0].(*string)) = s.productName
	*(dest[1].(*string)) = s.organizationName
	*(dest[2].(*int)) = s.version
	*(dest[3].(*time.Time)) = s.createdAt
	*(dest[4].(*time.Time)) = s.updatedAt
	return nil
}
