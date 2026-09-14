package repository

import (
	"context"
	"database/sql"

	"github.com/jonathanhu237/rota/api/internal/rota/model"
)

// BrandingRepository is a read-only compatibility adapter over Temvia's
// authoritative system identity. Rota notifications use the same configured
// product name and never maintain a parallel branding row.
type BrandingRepository struct {
	db DBTX
}

func NewBrandingRepository(db DBTX) *BrandingRepository {
	return &BrandingRepository{db: db}
}

func (r *BrandingRepository) GetBranding(ctx context.Context) (*model.Branding, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	branding := &model.Branding{}
	err := r.db.QueryRowContext(ctx, `
		SELECT system_name, organization_name, revision
		FROM auth_system_identity
		WHERE singleton = true`).Scan(&branding.ProductName, &branding.OrganizationName, &branding.Version)
	if err != nil {
		if err == sql.ErrNoRows {
			return &model.Branding{ProductName: model.DefaultProductName, OrganizationName: model.DefaultOrganizationName}, nil
		}
		return nil, err
	}
	return branding, nil
}
