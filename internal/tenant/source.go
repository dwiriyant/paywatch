package tenant

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/dwiriyant/paywatch/internal/domain"
	"github.com/dwiriyant/paywatch/internal/provider"
	"github.com/dwiriyant/paywatch/internal/provider/gobiz"
)

type Lister interface {
	ListEnabled(ctx context.Context) ([]*domain.Tenant, error)
}

type Disabler interface {
	GetByAppID(ctx context.Context, appID string) (*domain.Tenant, error)
	Update(ctx context.Context, t *domain.Tenant) error
}

type Options struct {
	HistoryDays int
	HistorySize int
	CacheDir    string
	Log         *slog.Logger
	Gate        *AuthGate
	Disabler    Disabler
}

// Source builds live providers from DB tenants each poll cycle.
type Source struct {
	lister Lister
	opts   Options
}

func NewSource(lister Lister, opts Options) *Source {
	if opts.CacheDir == "" {
		opts.CacheDir = ".cache"
	}
	if opts.Log == nil {
		opts.Log = slog.Default()
	}
	if opts.Gate == nil {
		opts.Gate = NewAuthGate()
	}
	return &Source{lister: lister, opts: opts}
}

func (s *Source) Gate() *AuthGate { return s.opts.Gate }

func (s *Source) Providers(ctx context.Context) ([]domain.Provider, error) {
	tenants, err := s.lister.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Provider, 0, len(tenants))
	for _, t := range tenants {
		if reason, ok := s.opts.Gate.Blocked(t.AppID); ok {
			s.opts.Log.Warn("skip tenant: auth blocked", "app_id", t.AppID, "reason", reason)
			continue
		}
		inner, err := s.build(t)
		if err != nil {
			s.opts.Log.Error("skip tenant", "app_id", t.AppID, "err", err)
			continue
		}
		out = append(out, &guardedProvider{
			appID: t.AppID,
			inner: &provider.Tenant{AppID: t.AppID, Inner: inner},
			src:   s,
		})
	}
	return out, nil
}

func (s *Source) build(t *domain.Tenant) (domain.Provider, error) {
	switch strings.ToLower(t.Provider) {
	case "gobiz":
		return gobiz.NewProvider(gobiz.Config{
			LoginMethod: t.LoginMethod,
			Email:       t.Email,
			Password:    t.Password,
			Phone:       t.Phone,
			AccessToken: t.AccessToken,
			MerchantID:  t.MerchantID,
			CachePath:   filepath.Join(s.opts.CacheDir, "gobiz-"+t.AppID+".json"),
			HistoryDays: s.opts.HistoryDays,
			HistorySize: s.opts.HistorySize,
			Log:         s.opts.Log.With("app_id", t.AppID, "provider", "gobiz"),
		}, nil), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", t.Provider)
	}
}

func (s *Source) blockAuth(ctx context.Context, appID string, cause error) {
	s.opts.Gate.Block(appID, cause.Error())
	s.opts.Log.Error("auth fatal: stopping polls for tenant (fix credentials then re-enable)",
		"app_id", appID, "err", cause)
	if s.opts.Disabler == nil {
		return
	}
	t, err := s.opts.Disabler.GetByAppID(ctx, appID)
	if err != nil {
		s.opts.Log.Error("disable tenant failed", "app_id", appID, "err", err)
		return
	}
	if !t.Enabled {
		return
	}
	t.Enabled = false
	if err := s.opts.Disabler.Update(ctx, t); err != nil {
		s.opts.Log.Error("disable tenant failed", "app_id", appID, "err", err)
	}
}

type guardedProvider struct {
	appID string
	inner domain.Provider
	src   *Source
}

func (g *guardedProvider) Name() string { return g.inner.Name() }

func (g *guardedProvider) Poll(ctx context.Context) ([]domain.IncomingPayment, error) {
	list, err := g.inner.Poll(ctx)
	if err != nil && errors.Is(err, domain.ErrAuthFatal) {
		g.src.blockAuth(ctx, g.appID, err)
	}
	return list, err
}
