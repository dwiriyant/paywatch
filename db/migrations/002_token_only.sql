-- +goose Up
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS refresh_token TEXT NOT NULL DEFAULT '';

-- Token-only: wipe stored passwords; tenants without an access token stay disabled.
UPDATE tenants SET enabled = FALSE WHERE COALESCE(access_token, '') = '';
UPDATE tenants
SET password = '',
    login_method = CASE WHEN COALESCE(access_token, '') <> '' THEN 'token' ELSE login_method END;

-- +goose Down
ALTER TABLE tenants DROP COLUMN IF EXISTS refresh_token;
