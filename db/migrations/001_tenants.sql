-- +goose Up
CREATE TABLE tenants (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    login_method TEXT NOT NULL DEFAULT 'password',
    email TEXT NOT NULL DEFAULT '',
    password TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    access_token TEXT NOT NULL DEFAULT '',
    merchant_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tenants_enabled ON tenants (enabled) WHERE enabled = TRUE;

-- +goose Down
DROP INDEX IF EXISTS idx_tenants_enabled;
DROP TABLE IF EXISTS tenants;
