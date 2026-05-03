package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

type UpdateBrandingParams struct {
	ProductName      string
	OrganizationName string
	Version          int
}

type BrandingRepository struct {
	db DBTX
}

func NewBrandingRepository(db DBTX) *BrandingRepository {
	return &BrandingRepository{db: db}
}

func (r *BrandingRepository) Get(ctx context.Context) (*model.Branding, error) {
	const query = `
		SELECT product_name, organization_name, version, created_at, updated_at
		FROM app_branding
		WHERE id = 1;
	`

	branding := &model.Branding{}
	err := scanBranding(r.db.QueryRowContext(ctx, query), branding)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultBranding(), nil
	}
	if err != nil {
		return nil, err
	}
	return branding, nil
}

func (r *BrandingRepository) UpsertDefault(ctx context.Context) (*model.Branding, error) {
	const query = `
		INSERT INTO app_branding (id, product_name, organization_name)
		VALUES (1, $1, $2)
		ON CONFLICT (id) DO UPDATE
		SET id = app_branding.id
		RETURNING product_name, organization_name, version, created_at, updated_at;
	`

	branding := &model.Branding{}
	if err := scanBranding(
		r.db.QueryRowContext(
			ctx,
			query,
			model.DefaultProductName,
			model.DefaultOrganizationName,
		),
		branding,
	); err != nil {
		return nil, err
	}
	return branding, nil
}

func (r *BrandingRepository) Update(ctx context.Context, params UpdateBrandingParams) (*model.Branding, error) {
	const query = `
		UPDATE app_branding
		SET product_name = $1,
		    organization_name = $2,
		    version = version + 1,
		    updated_at = NOW()
		WHERE id = 1 AND version = $3
		RETURNING product_name, organization_name, version, created_at, updated_at;
	`

	branding := &model.Branding{}
	err := scanBranding(
		r.db.QueryRowContext(
			ctx,
			query,
			params.ProductName,
			params.OrganizationName,
			params.Version,
		),
		branding,
	)
	if errors.Is(err, sql.ErrNoRows) {
		exists, existsErr := r.exists(ctx)
		if existsErr != nil {
			return nil, existsErr
		}
		if exists || params.Version != 1 {
			return nil, ErrVersionConflict
		}
		return r.insertUpdated(ctx, params)
	}
	if err != nil {
		return nil, err
	}
	return branding, nil
}

func (r *BrandingRepository) exists(ctx context.Context) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM app_branding WHERE id = 1);`

	var exists bool
	if err := r.db.QueryRowContext(ctx, query).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *BrandingRepository) insertUpdated(ctx context.Context, params UpdateBrandingParams) (*model.Branding, error) {
	const query = `
		INSERT INTO app_branding (id, product_name, organization_name, version)
		VALUES (1, $1, $2, 2)
		RETURNING product_name, organization_name, version, created_at, updated_at;
	`

	branding := &model.Branding{}
	err := scanBranding(
		r.db.QueryRowContext(
			ctx,
			query,
			params.ProductName,
			params.OrganizationName,
		),
		branding,
	)
	if err != nil {
		return nil, err
	}
	return branding, nil
}

type brandingScanner interface {
	Scan(dest ...any) error
}

func scanBranding(scanner brandingScanner, branding *model.Branding) error {
	return scanner.Scan(
		&branding.ProductName,
		&branding.OrganizationName,
		&branding.Version,
		&branding.CreatedAt,
		&branding.UpdatedAt,
	)
}

func defaultBranding() *model.Branding {
	return &model.Branding{
		ProductName:      model.DefaultProductName,
		OrganizationName: model.DefaultOrganizationName,
		Version:          1,
	}
}
