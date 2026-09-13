package gobiz

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/dwiriyant/paywatch/internal/cache"
	"github.com/dwiriyant/paywatch/internal/domain"
)

type tokenCache struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	MerchantID   string `json:"merchant_id"`
}

// SessionSaver persists rotated tokens (e.g. after refresh) back to the tenant store.
type SessionSaver func(ctx context.Context, accessToken, refreshToken, merchantID string) error

// Config wires the GoBiz provider.
type Config struct {
	LoginMethod  string // token (password/otp only used at verify time, not for polling)
	AccessToken  string
	RefreshToken string
	MerchantID   string
	CachePath    string
	HistoryDays  int
	HistorySize  int
	Log          *slog.Logger
	SaveSession  SessionSaver
}

// Provider implements domain.Provider for GoBiz / GoPay Merchant.
type Provider struct {
	cfg    Config
	client *Client
	store  *cache.FileStore
	log    *slog.Logger
	mu     sync.Mutex
}

func NewProvider(cfg Config, client *Client) *Provider {
	if client == nil {
		client = NewClient(nil)
	}
	log := cfg.Log
	if log == nil {
		log = slog.Default()
	}
	path := cfg.CachePath
	if path == "" {
		path = ".gobiz_cache.json"
	}
	return &Provider{cfg: cfg, client: client, store: cache.NewFileStore(path), log: log}
}

func (p *Provider) Name() string { return "gobiz" }

func (p *Provider) Poll(ctx context.Context) ([]domain.IncomingPayment, error) {
	if err := p.ensureSession(ctx); err != nil {
		return nil, err
	}
	list, err := p.client.History(ctx, p.cfg.HistoryDays, p.cfg.HistorySize)
	if err == errUnauthorized {
		p.log.Warn("gobiz token expired, refreshing")
		p.mu.Lock()
		p.client.SetToken("")
		p.mu.Unlock()
		if err := p.ensureSession(ctx); err != nil {
			return nil, err
		}
		return p.client.History(ctx, p.cfg.HistoryDays, p.cfg.HistorySize)
	}
	return list, err
}

func (p *Provider) ensureSession(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var cached tokenCache
	_ = p.store.Load(&cached)

	if p.client.Token() == "" {
		switch {
		case cached.AccessToken != "":
			p.client.SetToken(cached.AccessToken)
			p.log.Info("gobiz token loaded from cache")
		case p.cfg.AccessToken != "":
			p.client.SetToken(p.cfg.AccessToken)
		}
	}
	if p.client.RefreshToken() == "" {
		switch {
		case cached.RefreshToken != "":
			p.client.SetRefreshToken(cached.RefreshToken)
		case p.cfg.RefreshToken != "":
			p.client.SetRefreshToken(p.cfg.RefreshToken)
		}
	}
	if p.client.MerchantID() == "" {
		if p.cfg.MerchantID != "" {
			p.client.SetMerchantID(p.cfg.MerchantID)
		} else if cached.MerchantID != "" {
			p.client.SetMerchantID(cached.MerchantID)
			p.log.Info("gobiz merchant id loaded from cache", "merchant_id", cached.MerchantID)
		}
	}

	if p.client.Token() == "" || !p.client.TokenValid(ctx) {
		p.log.Info("gobiz access token invalid; attempting refresh")
		if err := p.refresh(ctx); err != nil {
			return err
		}
	}

	if p.client.MerchantID() == "" {
		id, err := p.client.ResolveMerchantID(ctx)
		if err != nil {
			return err
		}
		p.log.Info("gobiz merchant resolved", "merchant_id", id)
	}

	return p.persistLocked(ctx)
}

func (p *Provider) refresh(ctx context.Context) error {
	rt := p.client.RefreshToken()
	if rt == "" {
		rt = p.cfg.RefreshToken
	}
	if rt == "" {
		return fmt.Errorf("%w: access token expired and no refresh token; re-verify credentials", domain.ErrAuthFatal)
	}
	if err := p.client.RefreshAccessToken(ctx, rt); err != nil {
		return err
	}
	p.log.Info("gobiz token refreshed")
	return nil
}

func (p *Provider) persistLocked(ctx context.Context) error {
	access := p.client.Token()
	refresh := p.client.RefreshToken()
	merchant := p.client.MerchantID()
	_ = p.store.Save(tokenCache{
		AccessToken:  access,
		RefreshToken: refresh,
		MerchantID:   merchant,
	})
	if p.cfg.SaveSession != nil {
		if err := p.cfg.SaveSession(ctx, access, refresh, merchant); err != nil {
			p.log.Warn("persist gobiz session failed", "err", err)
		}
	}
	return nil
}
