package admin

import (
	"context"
	"errors"
	"strings"

	"github.com/dwiriyant/paywatch/internal/domain"
	"github.com/dwiriyant/paywatch/internal/provider/gobiz"
	"github.com/dwiriyant/paywatch/internal/tenant"
)

type TenantStore interface {
	Create(ctx context.Context, t *domain.Tenant) error
	GetByID(ctx context.Context, id string) (*domain.Tenant, error)
	List(ctx context.Context) ([]*domain.Tenant, error)
	Update(ctx context.Context, t *domain.Tenant) error
	Delete(ctx context.Context, id string) error
}

// AuthSession is the token pair obtained from a live GoBiz login (password is not stored).
type AuthSession struct {
	AccessToken  string
	RefreshToken string
	MerchantID   string
}

// PasswordVerifier exchanges email/password for tokens against the live API.
type PasswordVerifier func(ctx context.Context, email, password string) (AuthSession, error)

type Service struct {
	tenants        TenantStore
	gate           *tenant.AuthGate
	verifyPassword PasswordVerifier
	pending        *pendingSessions
}

func NewService(tenants TenantStore) *Service {
	return &Service{
		tenants:        tenants,
		verifyPassword: liveVerifyPassword,
		pending:        newPendingSessions(),
	}
}

func (s *Service) WithAuthGate(gate *tenant.AuthGate) *Service {
	s.gate = gate
	return s
}

func (s *Service) WithPasswordVerifier(fn PasswordVerifier) *Service {
	if fn != nil {
		s.verifyPassword = fn
	}
	return s
}

type GobizInput struct {
	LoginMethod  string `json:"login_method"`
	Email        string `json:"email"`
	Password     string `json:"password"` // request-only; never persisted
	Phone        string `json:"phone"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	MerchantID   string `json:"merchant_id"`
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

type VerifyInput struct {
	Provider string      `json:"provider"`
	Gobiz    *GobizInput `json:"gobiz"`
}

type VerifyResult struct {
	OK               bool   `json:"ok"`
	MerchantID       string `json:"merchant_id,omitempty"`
	HasToken         bool   `json:"has_access_token,omitempty"`
	HasRefresh       bool   `json:"has_refresh_token,omitempty"`
	SaveWindowSec    int    `json:"save_window_sec,omitempty"` // create/update with same password within this many seconds skips re-login
	Message          string `json:"message,omitempty"`
}

type TenantView struct {
	ID           string `json:"id"`
	AppID        string `json:"app_id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Enabled      bool   `json:"enabled"`
	LoginMethod  string `json:"login_method"`
	Email        string `json:"email,omitempty"`
	Phone        string `json:"phone,omitempty"`
	MerchantID   string `json:"merchant_id,omitempty"`
	HasPassword  bool   `json:"has_password"` // always false after token-only; kept for API compat
	HasToken     bool   `json:"has_access_token"`
	HasRefresh   bool   `json:"has_refresh_token"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func toView(t *domain.Tenant) TenantView {
	return TenantView{
		ID: t.ID, AppID: t.AppID, Name: t.Name, Provider: t.Provider, Enabled: t.Enabled,
		LoginMethod: t.LoginMethod, Email: t.Email, Phone: t.Phone, MerchantID: t.MerchantID,
		HasPassword: false, HasToken: t.AccessToken != "", HasRefresh: t.RefreshToken != "",
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
	enabled := false
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	t := &domain.Tenant{
		AppID: in.AppID, Name: in.Name, Provider: provider, Enabled: enabled,
		LoginMethod: "token",
	}
	if in.Gobiz != nil {
		if err := s.applyGobizSession(ctx, t, in.Gobiz, true); err != nil {
			return nil, err
		}
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
	if in.Provider != nil {
		p := strings.ToLower(*in.Provider)
		if p != "gobiz" {
			return nil, domain.ErrInvalidInput
		}
		t.Provider = p
	}
	if in.Gobiz != nil {
		if err := s.applyGobizSession(ctx, t, in.Gobiz, false); err != nil {
			return nil, err
		}
		if in.Enabled == nil {
			t.Enabled = false
		}
	}
	if in.Enabled != nil {
		t.Enabled = *in.Enabled
	}
	if err := validateGobiz(t); err != nil {
		return nil, err
	}
	if err := s.tenants.Update(ctx, t); err != nil {
		return nil, err
	}
	if s.gate != nil && t.Enabled && (in.Enabled != nil || in.Gobiz != nil) {
		s.gate.Unblock(t.AppID)
	}
	v := toView(t)
	return &v, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.tenants.Delete(ctx, id)
}

// Verify checks gobiz credentials against the live API (does not enable polling).
func (s *Service) Verify(ctx context.Context, in VerifyInput) (*VerifyResult, error) {
	provider := strings.ToLower(in.Provider)
	if provider == "" {
		provider = "gobiz"
	}
	if provider != "gobiz" || in.Gobiz == nil {
		return nil, domain.ErrInvalidInput
	}
	return s.verifyGobiz(ctx, in.Gobiz)
}

// VerifyTenant checks stored tokens (refresh if needed) and persists rotated tokens.
func (s *Service) VerifyTenant(ctx context.Context, id string) (*VerifyResult, error) {
	t, err := s.tenants.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	g := &GobizInput{
		LoginMethod:  "token",
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		MerchantID:   t.MerchantID,
		Email:        t.Email,
	}
	sess, out, err := s.verifyGobizSession(ctx, g)
	if err != nil {
		return nil, err
	}
	if sess.AccessToken != "" && sess.AccessToken != t.AccessToken {
		t.AccessToken = sess.AccessToken
		if sess.RefreshToken != "" {
			t.RefreshToken = sess.RefreshToken
		}
		t.Password = ""
		t.LoginMethod = "token"
		if sess.MerchantID != "" {
			t.MerchantID = sess.MerchantID
		}
		_ = s.tenants.Update(ctx, t)
	}
	return out, nil
}

func (s *Service) verifyGobiz(ctx context.Context, g *GobizInput) (*VerifyResult, error) {
	_, out, err := s.verifyGobizSession(ctx, g)
	return out, err
}

func (s *Service) verifyGobizSession(ctx context.Context, g *GobizInput) (AuthSession, *VerifyResult, error) {
	method := strings.ToLower(g.LoginMethod)
	if method == "" {
		if g.Password != "" {
			method = "password"
		} else {
			method = "token"
		}
	}
	switch method {
	case "password":
		if g.Email == "" || g.Password == "" {
			return AuthSession{}, nil, domain.ErrInvalidInput
		}
		sess, err := s.verifyPassword(ctx, g.Email, g.Password)
		if err != nil {
			return AuthSession{}, nil, err
		}
		s.pending.Put(g.Email, g.Password, sess)
		return sess, &VerifyResult{
			OK: true, MerchantID: sess.MerchantID, Message: "credentials ok; save tenant within 60s to skip re-login",
			HasToken: sess.AccessToken != "", HasRefresh: sess.RefreshToken != "",
			SaveWindowSec: int(pendingSessionTTL.Seconds()),
		}, nil
	case "token":
		if g.AccessToken == "" && g.RefreshToken == "" {
			return AuthSession{}, nil, domain.ErrInvalidInput
		}
		sess, err := liveVerifyOrRefresh(ctx, g.AccessToken, g.RefreshToken)
		if err != nil {
			return AuthSession{}, nil, err
		}
		return sess, &VerifyResult{
			OK: true, MerchantID: sess.MerchantID, Message: "credentials ok",
			HasToken: true, HasRefresh: sess.RefreshToken != "",
		}, nil
	default:
		return AuthSession{}, nil, domain.ErrInvalidInput
	}
}

// applyGobizSession exchanges a password (if present) for tokens and never keeps the password.
func (s *Service) applyGobizSession(ctx context.Context, t *domain.Tenant, g *GobizInput, create bool) error {
	if g.Email != "" || create {
		t.Email = g.Email
	}
	if g.Phone != "" || create {
		t.Phone = g.Phone
	}
	if g.MerchantID != "" {
		t.MerchantID = g.MerchantID
	}

	switch {
	case g.Password != "":
		if g.Email == "" {
			return domain.ErrInvalidInput
		}
		sess, ok := s.pending.Get(g.Email, g.Password)
		if !ok {
			var err error
			sess, err = s.verifyPassword(ctx, g.Email, g.Password)
			if err != nil {
				return err
			}
		} else {
			s.pending.Clear(g.Email, g.Password)
		}
		t.AccessToken = sess.AccessToken
		t.RefreshToken = sess.RefreshToken
		if sess.MerchantID != "" {
			t.MerchantID = sess.MerchantID
		}
	case g.AccessToken != "":
		t.AccessToken = g.AccessToken
		if g.RefreshToken != "" {
			t.RefreshToken = g.RefreshToken
		}
	case create:
		return domain.ErrInvalidInput
	}

	t.Password = ""
	t.LoginMethod = "token"
	return nil
}

func validateGobiz(t *domain.Tenant) error {
	if t.AccessToken == "" {
		return domain.ErrInvalidInput
	}
	t.Password = ""
	t.LoginMethod = "token"
	return nil
}

func liveVerifyPassword(ctx context.Context, email, password string) (AuthSession, error) {
	c := gobiz.NewClient(nil)
	if err := c.LoginPassword(ctx, email, password); err != nil {
		return AuthSession{}, err
	}
	mid, err := c.ResolveMerchantID(ctx)
	if err != nil {
		return AuthSession{}, err
	}
	return AuthSession{
		AccessToken:  c.Token(),
		RefreshToken: c.RefreshToken(),
		MerchantID:   mid,
	}, nil
}

func liveVerifyOrRefresh(ctx context.Context, access, refresh string) (AuthSession, error) {
	c := gobiz.NewClient(nil)
	c.SetToken(access)
	c.SetRefreshToken(refresh)
	if access != "" && c.TokenValid(ctx) {
		mid, err := c.ResolveMerchantID(ctx)
		if err != nil {
			return AuthSession{}, err
		}
		return AuthSession{AccessToken: c.Token(), RefreshToken: c.RefreshToken(), MerchantID: mid}, nil
	}
	if refresh == "" {
		return AuthSession{}, errors.Join(domain.ErrAuthFatal, errors.New("gobiz access token invalid"))
	}
	if err := c.RefreshAccessToken(ctx, refresh); err != nil {
		return AuthSession{}, err
	}
	mid, err := c.ResolveMerchantID(ctx)
	if err != nil {
		return AuthSession{}, err
	}
	return AuthSession{AccessToken: c.Token(), RefreshToken: c.RefreshToken(), MerchantID: mid}, nil
}
