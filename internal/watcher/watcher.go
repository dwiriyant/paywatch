package watcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/dwiriyant/paywatch/internal/cache"
	"github.com/dwiriyant/paywatch/internal/domain"
)

type seenState struct {
	Seeded    bool     `json:"seeded"` // legacy global flag
	SeededApps []string `json:"seeded_apps"`
	IDs       []string `json:"ids"`
}

type ProviderSource interface {
	Providers(ctx context.Context) ([]domain.Provider, error)
}

// Watcher polls providers from a source, seeds per-app, then settles new pay-ins.
type Watcher struct {
	source   ProviderSource
	settler  domain.Settler
	interval time.Duration
	store    *cache.FileStore
	log      *slog.Logger

	mu         sync.Mutex
	seen       map[string]struct{}
	seededApps map[string]struct{}
	maxSeen    int
}

func New(source ProviderSource, settler domain.Settler, interval time.Duration, statePath string, log *slog.Logger) *Watcher {
	if log == nil {
		log = slog.Default()
	}
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	w := &Watcher{
		source:     source,
		settler:    settler,
		interval:   interval,
		store:      cache.NewFileStore(statePath),
		log:        log,
		seen:       make(map[string]struct{}),
		seededApps: make(map[string]struct{}),
		maxSeen:    2000,
	}
	w.load()
	return w
}

func (w *Watcher) load() {
	var st seenState
	if err := w.store.Load(&st); err != nil {
		w.log.Warn("load seen state failed", "err", err)
		return
	}
	for _, id := range st.IDs {
		w.seen[id] = struct{}{}
	}
	for _, a := range st.SeededApps {
		w.seededApps[a] = struct{}{}
	}
	// migrate legacy global seed → treat as no per-app seed (will re-seed safely)
	_ = st.Seeded
}

func (w *Watcher) persistLocked() {
	ids := make([]string, 0, len(w.seen))
	for id := range w.seen {
		ids = append(ids, id)
	}
	if len(ids) > w.maxSeen {
		ids = ids[len(ids)-w.maxSeen:]
	}
	apps := make([]string, 0, len(w.seededApps))
	for a := range w.seededApps {
		apps = append(apps, a)
	}
	_ = w.store.Save(seenState{SeededApps: apps, IDs: ids})
}

func key(p domain.IncomingPayment) string {
	return p.AppID + ":" + p.Provider + ":" + p.ExternalID
}

func (w *Watcher) Run(ctx context.Context) error {
	w.log.Info("watcher started", "interval", w.interval.String())
	t := time.NewTicker(w.interval)
	defer t.Stop()

	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			w.mu.Lock()
			w.persistLocked()
			w.mu.Unlock()
			return ctx.Err()
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *Watcher) Tick(ctx context.Context) { w.tick(ctx) }

func (w *Watcher) tick(ctx context.Context) {
	providers, err := w.source.Providers(ctx)
	if err != nil {
		w.log.Error("list providers failed", "err", err)
		return
	}
	for _, p := range providers {
		list, err := p.Poll(ctx)
		if err != nil {
			w.log.Error("poll failed", "provider", p.Name(), "err", err)
			continue
		}
		w.handle(ctx, list)
	}
}

func (w *Watcher) handle(ctx context.Context, list []domain.IncomingPayment) {
	w.mu.Lock()
	newOnes := make([]domain.IncomingPayment, 0)
	firstSeen := map[string][]domain.IncomingPayment{}

	for _, pay := range list {
		if pay.ExternalID == "" || pay.AppID == "" {
			continue
		}
		if _, seeded := w.seededApps[pay.AppID]; !seeded {
			firstSeen[pay.AppID] = append(firstSeen[pay.AppID], pay)
			continue
		}
		k := key(pay)
		if _, ok := w.seen[k]; ok {
			continue
		}
		w.seen[k] = struct{}{}
		newOnes = append(newOnes, pay)
	}

	for appID, pays := range firstSeen {
		for _, pay := range pays {
			w.seen[key(pay)] = struct{}{}
		}
		w.seededApps[appID] = struct{}{}
		w.log.Info("seed complete", "app_id", appID, "known", len(pays))
	}
	w.persistLocked()
	w.mu.Unlock()

	for _, pay := range newOnes {
		w.log.Info("new payment", "app_id", pay.AppID, "provider", pay.Provider, "external_id", pay.ExternalID, "amount", pay.Amount)
		if err := w.settler.Settle(ctx, pay); err != nil {
			w.log.Error("settle failed", "app_id", pay.AppID, "external_id", pay.ExternalID, "err", err)
		}
	}
}
