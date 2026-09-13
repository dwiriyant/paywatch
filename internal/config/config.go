package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr         string
	DatabaseURL  string
	AdminToken   string
	PollInterval time.Duration
	DryRun       bool

	QRISGateBaseURL    string
	QRISGateAdminToken string

	GobizHistoryDays int
	GobizHistorySize int
	CacheDir         string
	SeenStatePath    string

	// VerifyRateLimitPerMin caps live GoBiz credential checks (spam / lockout risk).
	VerifyRateLimitPerMin int
}

func Load() (Config, error) {
	cfg := Config{
		Addr:               getenv("ADDR", ":8081"),
		DatabaseURL:        getenv("DATABASE_URL", "postgres://paywatch:paywatch@localhost:5434/paywatch?sslmode=disable"),
		AdminToken:         os.Getenv("ADMIN_TOKEN"),
		PollInterval:       time.Duration(getenvInt("POLL_INTERVAL_MS", 6000)) * time.Millisecond,
		DryRun:             getenvBool("DRY_RUN", false),
		QRISGateBaseURL:    strings.TrimRight(getenv("QRISGATE_BASE_URL", "http://127.0.0.1:8080"), "/"),
		QRISGateAdminToken: os.Getenv("QRISGATE_ADMIN_TOKEN"),
		GobizHistoryDays:   getenvInt("GOBIZ_HISTORY_DAYS", 1),
		GobizHistorySize:   getenvInt("GOBIZ_HISTORY_SIZE", 30),
		CacheDir:              getenv("CACHE_DIR", ".cache"),
		SeenStatePath:         getenv("SEEN_STATE_PATH", ".paywatch_seen.json"),
		VerifyRateLimitPerMin: getenvInt("VERIFY_RATE_LIMIT_PER_MIN", 1),
	}
	if cfg.PollInterval < 5*time.Second {
		return cfg, fmt.Errorf("POLL_INTERVAL_MS must be >= 5000 (ban risk)")
	}
	if cfg.AdminToken == "" {
		return cfg, fmt.Errorf("ADMIN_TOKEN is required")
	}
	if !cfg.DryRun && cfg.QRISGateAdminToken == "" {
		return cfg, fmt.Errorf("QRISGATE_ADMIN_TOKEN is required unless DRY_RUN=true")
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
