package config_test

import (
	"testing"
	"time"

	"github.com/dwiriyant/paywatch/internal/config"
)

func TestLoad_ok(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "admin")
	t.Setenv("DRY_RUN", "true")
	t.Setenv("POLL_INTERVAL_MS", "6000")
	t.Setenv("DATABASE_URL", "postgres://x")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PollInterval != 6*time.Second || cfg.Addr == "" {
		t.Fatalf("%+v", cfg)
	}
}

func TestLoad_requiresAdmin(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "")
	t.Setenv("DRY_RUN", "true")
	t.Setenv("DATABASE_URL", "postgres://x")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error")
	}
}
