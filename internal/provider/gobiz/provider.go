package gobiz

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/dwiriyant/paywatch/internal/cache"
	"github.com/dwiriyant/paywatch/internal/domain"
)

type tokenCache struct {
	AccessToken string `json:"access_token"`
	MerchantID  string `json:"merchant_id"`
}

// Config wires the GoBiz provider.
type Config struct {
	LoginMethod string // password | otp | token
	Email       string
	Password    string
	Phone       string
	AccessToken string
	MerchantID  string
	CachePath   string
	HistoryDays int
	HistorySize int
	Log         *slog.Logger
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
		p.log.Warn("gobiz token expired, re-login")
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
		case p.cfg.AccessToken != "":
			p.client.SetToken(p.cfg.AccessToken)
		case cached.AccessToken != "":
			p.client.SetToken(cached.AccessToken)
			p.log.Info("gobiz token loaded from cache")
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
		p.log.Info("gobiz login required", "method", p.cfg.LoginMethod)
		if err := p.login(ctx); err != nil {
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

	return p.store.Save(tokenCache{
		AccessToken: p.client.Token(),
		MerchantID:  p.client.MerchantID(),
	})
}

func (p *Provider) login(ctx context.Context) error {
	method := strings.ToLower(p.cfg.LoginMethod)
	if method == "" {
		method = "password"
	}
	switch method {
	case "token":
		if p.cfg.AccessToken == "" {
			return fmt.Errorf("GOBIZ_ACCESS_TOKEN required for login method token")
		}
		p.client.SetToken(p.cfg.AccessToken)
		return nil
	case "password":
		if p.cfg.Email == "" || p.cfg.Password == "" {
			return fmt.Errorf("GOBIZ_EMAIL and GOBIZ_PASSWORD required")
		}
		return p.client.LoginPassword(ctx, p.cfg.Email, p.cfg.Password)
	case "otp":
		if p.cfg.Phone == "" {
			return fmt.Errorf("GOBIZ_PHONE required for otp login")
		}
		otpToken, err := p.client.LoginOTPRequest(ctx, p.cfg.Phone)
		if err != nil {
			return err
		}
		otp := strings.TrimSpace(os.Getenv("GOBIZ_OTP"))
		if otp == "" {
			fmt.Fprintf(os.Stderr, "Enter GoBiz OTP for %s: ", p.cfg.Phone)
			line, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				return err
			}
			otp = strings.TrimSpace(line)
		}
		if otp == "" {
			return fmt.Errorf("empty OTP")
		}
		return p.client.LoginOTPVerify(ctx, p.cfg.Phone, otp, otpToken)
	default:
		return fmt.Errorf("unknown GOBIZ_LOGIN_METHOD %q", method)
	}
}
