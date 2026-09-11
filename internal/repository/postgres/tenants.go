package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/dwiriyant/paywatch/internal/domain"
)

type TenantRepository struct {
	pool *pgxpool.Pool
}

func NewTenantRepository(pool *pgxpool.Pool) *TenantRepository {
	return &TenantRepository{pool: pool}
}

const tenantCols = `id, app_id, name, provider, enabled, login_method, email, password, phone, access_token, merchant_id, created_at, updated_at`

func (r *TenantRepository) Create(ctx context.Context, t *domain.Tenant) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tenants (
			id, app_id, name, provider, enabled, login_method, email, password, phone, access_token, merchant_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		t.ID, t.AppID, t.Name, t.Provider, t.Enabled, t.LoginMethod, t.Email, t.Password, t.Phone, t.AccessToken, t.MerchantID, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrConflict
		}
		return err
	}
	return nil
}

func (r *TenantRepository) GetByID(ctx context.Context, id string) (*domain.Tenant, error) {
	return scanTenant(r.pool.QueryRow(ctx, `SELECT `+tenantCols+` FROM tenants WHERE id = $1`, id))
}

func (r *TenantRepository) GetByAppID(ctx context.Context, appID string) (*domain.Tenant, error) {
	return scanTenant(r.pool.QueryRow(ctx, `SELECT `+tenantCols+` FROM tenants WHERE app_id = $1`, appID))
}

func (r *TenantRepository) List(ctx context.Context) ([]*domain.Tenant, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+tenantCols+` FROM tenants ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Tenant
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *TenantRepository) ListEnabled(ctx context.Context) ([]*domain.Tenant, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+tenantCols+` FROM tenants WHERE enabled = TRUE ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Tenant
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *TenantRepository) Update(ctx context.Context, t *domain.Tenant) error {
	t.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(ctx, `
		UPDATE tenants SET
			name = $2, provider = $3, enabled = $4, login_method = $5,
			email = $6, password = $7, phone = $8, access_token = $9, merchant_id = $10, updated_at = $11
		WHERE id = $1`,
		t.ID, t.Name, t.Provider, t.Enabled, t.LoginMethod,
		t.Email, t.Password, t.Phone, t.AccessToken, t.MerchantID, t.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *TenantRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTenant(row scanner) (*domain.Tenant, error) {
	var t domain.Tenant
	err := row.Scan(
		&t.ID, &t.AppID, &t.Name, &t.Provider, &t.Enabled, &t.LoginMethod,
		&t.Email, &t.Password, &t.Phone, &t.AccessToken, &t.MerchantID, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func isUniqueViolation(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate key"))
}
