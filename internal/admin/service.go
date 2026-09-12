package admin

import (
	"context"
	"strings"

	"github.com/dwiriyant/paywatch/internal/domain"
	"github.com/dwiriyant/paywatch/internal/tenant"
)

type TenantStore interface {
	Create(ctx context.Context, t *domain.Tenant) error
	GetByID(ctx context.Context, id string) (*domain.Tenant, error)
	List(ctx context.Context) ([]*domain.Tenant, error)
	Update(ctx context.Context, t *domain.Tenant) error
	Delete(ctx context.Context, id string) error
}

type Service struct {
	tenants TenantStore
	gate    *tenant.AuthGate
}

func NewService(tenants TenantStore) *Service {
	return &Service{tenants: tenants}
}

func (s *Service) WithAuthGate(gate *tenant.AuthGate) *Service {
	s.gate = gate
	return s
}

type GobizInput struct {
	LoginMethod string `json:"login_method"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	Phone       string `json:"phone"`
	AccessToken string `json:"access_token"`
	MerchantID  string `json:"merchant_id"`
}

type CreateTenantInput struct {
	AppID    string      `json:"app_id"`
	Name     string      `json:"name"`
	Provider string      `json:"provider"`
	Enabled  *bool       `json:"enabled"`
	Gobiz    *GobizInput `json:"gobiz"`
}

type UpdateTenantInput struct {
	Name     *string     `json:"name"`
	Enabled  *bool       `json:"enabled"`
	Provider *string     `json:"provider"`
	Gobiz    *GobizInput `json:"gobiz"`
}

type TenantView struct {
	ID          string `json:"id"`
	AppID       string `json:"app_id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Enabled     bool   `json:"enabled"`
	LoginMethod string `json:"login_method"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	MerchantID  string `json:"merchant_id,omitempty"`
	HasPassword bool   `json:"has_password"`
	HasToken    bool   `json:"has_access_token"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toView(t *domain.Tenant) TenantView {
	return TenantView{
		ID: t.ID, AppID: t.AppID, Name: t.Name, Provider: t.Provider, Enabled: t.Enabled,
		LoginMethod: t.LoginMethod, Email: t.Email, Phone: t.Phone, MerchantID: t.MerchantID,
		HasPassword: t.Password != "", HasToken: t.AccessToken != "",
		CreatedAt: t.CreatedAt.UTC().Format(timeRFC3339), UpdatedAt: t.UpdatedAt.UTC().Format(timeRFC3339),
	}
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"

func (s *Service) Create(ctx context.Context, in CreateTenantInput) (*TenantView, error) {
	if in.AppID == "" {
		return nil, domain.ErrInvalidInput
	}
	provider := strings.ToLower(in.Provider)
	if provider == "" {
		provider = "gobiz"
	}
	if provider != "gobiz" {
		return nil, domain.ErrInvalidInput
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	t := &domain.Tenant{
		AppID: in.AppID, Name: in.Name, Provider: provider, Enabled: enabled,
		LoginMethod: "password",
	}
	if in.Gobiz != nil {
		applyGobiz(t, in.Gobiz, true)
	}
	if err := validateGobiz(t); err != nil {
		return nil, err
	}
	if err := s.tenants.Create(ctx, t); err != nil {
		return nil, err
	}
	v := toView(t)
	return &v, nil
}

func (s *Service) List(ctx context.Context) ([]TenantView, error) {
	list, err := s.tenants.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]TenantView, 0, len(list))
	for _, t := range list {
		out = append(out, toView(t))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id string) (*TenantView, error) {
	t, err := s.tenants.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	v := toView(t)
	return &v, nil
}

func (s *Service) Update(ctx context.Context, id string, in UpdateTenantInput) (*TenantView, error) {
	t, err := s.tenants.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		t.Name = *in.Name
	}
	if in.Enabled != nil {
		t.Enabled = *in.Enabled
	}
	if in.Provider != nil {
		p := strings.ToLower(*in.Provider)
		if p != "gobiz" {
			return nil, domain.ErrInvalidInput
		}
		t.Provider = p
	}
	if in.Gobiz != nil {
		applyGobiz(t, in.Gobiz, false)
	}
	if err := validateGobiz(t); err != nil {
		return nil, err
	}
	if err := s.tenants.Update(ctx, t); err != nil {
		return nil, err
	}
	// Re-enable or credential change → allow watcher to poll again.
	if s.gate != nil && t.Enabled && (in.Enabled != nil || in.Gobiz != nil) {
		s.gate.Unblock(t.AppID)
	}
	v := toView(t)
	return &v, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.tenants.Delete(ctx, id)
}

func applyGobiz(t *domain.Tenant, g *GobizInput, create bool) {
	if g.LoginMethod != "" {
		t.LoginMethod = strings.ToLower(g.LoginMethod)
	} else if create {
		t.LoginMethod = "password"
	}
	if g.Email != "" || create {
		t.Email = g.Email
	}
	if g.Password != "" {
		t.Password = g.Password
	}
	if g.Phone != "" || create {
		t.Phone = g.Phone
	}
	if g.AccessToken != "" {
		t.AccessToken = g.AccessToken
	}
	if g.MerchantID != "" || create {
		t.MerchantID = g.MerchantID
	}
}

func validateGobiz(t *domain.Tenant) error {
	switch t.LoginMethod {
	case "password":
		if t.Email == "" || t.Password == "" {
			return domain.ErrInvalidInput
		}
	case "otp":
		if t.Phone == "" {
			return domain.ErrInvalidInput
		}
	case "token":
		if t.AccessToken == "" {
			return domain.ErrInvalidInput
		}
	default:
		return domain.ErrInvalidInput
	}
	return nil
}
