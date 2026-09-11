package tenant

import (
	"context"
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

type Options struct {
	HistoryDays int
	HistorySize int
	CacheDir    string
	Log         *slog.Logger
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
	return &Source{lister: lister, opts: opts}
}

func (s *Source) Providers(ctx context.Context) ([]domain.Provider, error) {
	tenants, err := s.lister.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Provider, 0, len(tenants))
	for _, t := range tenants {
		inner, err := s.build(t)
		if err != nil {
			s.opts.Log.Error("skip tenant", "app_id", t.AppID, "err", err)
			continue
		}
		out = append(out, &provider.Tenant{AppID: t.AppID, Inner: inner})
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
