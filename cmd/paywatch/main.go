package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pressly/goose/v3"
	"github.com/joho/godotenv"

	"github.com/dwiriyant/paywatch/internal/admin"
	"github.com/dwiriyant/paywatch/internal/auth"
	"github.com/dwiriyant/paywatch/internal/config"
	"github.com/dwiriyant/paywatch/internal/domain"
	"github.com/dwiriyant/paywatch/internal/qrisgate"
	"github.com/dwiriyant/paywatch/internal/repository/postgres"
	"github.com/dwiriyant/paywatch/internal/settle"
	"github.com/dwiriyant/paywatch/internal/tenant"
	"github.com/dwiriyant/paywatch/internal/watcher"
)

func main() {
	_ = godotenv.Load()
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := runMigrations(cfg.DatabaseURL); err != nil {
		log.Error("migrations failed", "err", err)
		os.Exit(1)
	}

	tenantRepo := postgres.NewTenantRepository(pool)
	adminSvc := admin.NewService(tenantRepo)
	adminHandler := admin.NewHandler(adminSvc)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())
	e.GET("/healthz", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	v1 := e.Group("/v1", auth.AdminAuth(cfg.AdminToken))
	v1.POST("/tenants", adminHandler.Create)
	v1.GET("/tenants", adminHandler.List)
	v1.GET("/tenants/:id", adminHandler.Get)
	v1.PATCH("/tenants/:id", adminHandler.Update)
	v1.DELETE("/tenants/:id", adminHandler.Delete)

	server := &http.Server{Addr: cfg.Addr, Handler: e, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second}

	var settler domain.Settler = &settle.QRISGateSettler{
		Client: qrisgate.NewClient(cfg.QRISGateBaseURL, cfg.QRISGateAdminToken),
		Log:    log,
		DryRun: cfg.DryRun,
	}
	source := tenant.NewSource(tenantRepo, tenant.Options{
		HistoryDays: cfg.GobizHistoryDays,
		HistorySize: cfg.GobizHistorySize,
		CacheDir:    cfg.CacheDir,
		Log:         log,
	})
	w := watcher.New(source, settler, cfg.PollInterval, cfg.SeenStatePath, log)

	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("api listening", "addr", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()
	go func() {
		log.Info("watcher starting", "interval", cfg.PollInterval.String(), "dry_run", cfg.DryRun, "qrisgate", cfg.QRISGateBaseURL)
		if err := w.Run(runCtx); err != nil && err != context.Canceled {
			log.Error("watcher stopped", "err", err)
		}
	}()

	<-runCtx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func runMigrations(databaseURL string) error {
	db, err := goose.OpenDBWithDriver("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	dir := "db/migrations"
	if _, err := os.Stat("/db/migrations"); err == nil {
		dir = "/db/migrations"
	}
	return goose.Up(db, dir)
}
