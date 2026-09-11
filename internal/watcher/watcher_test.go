package watcher_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dwiriyant/paywatch/internal/domain"
	"github.com/dwiriyant/paywatch/internal/watcher"
)

type stubProvider struct {
	name string
	n    int
	list [][]domain.IncomingPayment
}

func (s *stubProvider) Name() string { return s.name }
func (s *stubProvider) Poll(ctx context.Context) ([]domain.IncomingPayment, error) {
	i := s.n
	if i >= len(s.list) {
		i = len(s.list) - 1
	} else {
		s.n++
	}
	return s.list[i], nil
}

type stubSource struct{ p []domain.Provider }

func (s stubSource) Providers(ctx context.Context) ([]domain.Provider, error) { return s.p, nil }

type stubSettler struct {
	got []domain.IncomingPayment
}

func (s *stubSettler) Settle(ctx context.Context, p domain.IncomingPayment) error {
	s.got = append(s.got, p)
	return nil
}

func TestWatcher_SeedsPerAppThenSettles(t *testing.T) {
	state := filepath.Join(t.TempDir(), "seen.json")
	p := &stubProvider{
		name: "app-1/gobiz",
		list: [][]domain.IncomingPayment{
			{{AppID: "app-1", Provider: "gobiz", ExternalID: "a", Amount: 1000}},
			{
				{AppID: "app-1", Provider: "gobiz", ExternalID: "a", Amount: 1000},
				{AppID: "app-1", Provider: "gobiz", ExternalID: "b", Amount: 2000},
			},
		},
	}
	s := &stubSettler{}
	w := watcher.New(stubSource{p: []domain.Provider{p}}, s, 5*time.Second, state, nil)

	w.Tick(context.Background())
	if len(s.got) != 0 {
		t.Fatalf("seed must not settle, got %#v", s.got)
	}

	w.Tick(context.Background())
	if len(s.got) != 1 || s.got[0].ExternalID != "b" {
		t.Fatalf("expected settle b, got %#v", s.got)
	}
}
