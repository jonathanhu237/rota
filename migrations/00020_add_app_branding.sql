-- +goose Up
CREATE TABLE app_branding (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    product_name TEXT NOT NULL CHECK (product_name = btrim(product_name) AND char_length(product_name) BETWEEN 1 AND 60),
    organization_name TEXT NOT NULL DEFAULT '' CHECK (organization_name = btrim(organization_name) AND char_length(organization_name) <= 100),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO app_branding (id, product_name, organization_name)
VALUES (1, 'Rota', '')
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS app_branding;
