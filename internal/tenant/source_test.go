package tenant

import (
	"context"
	"errors"
	"testing"

	"github.com/dwiriyant/paywatch/internal/domain"
)

func TestAuthGate(t *testing.T) {
	g := NewAuthGate()
	g.Block("a", "locked")
	if reason, ok := g.Blocked("a"); !ok || reason != "locked" {
		t.Fatalf("got %v %v", reason, ok)
	}
	g.Unblock("a")
	if _, ok := g.Blocked("a"); ok {
		t.Fatal("expected unblocked")
	}
}

type stubLister struct {
	tenants []*domain.Tenant
}

func (s stubLister) ListEnabled(ctx context.Context) ([]*domain.Tenant, error) {
	out := make([]*domain.Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		if t.Enabled {
			out = append(out, t)
		}
	}
	return out, nil
}

type stubDisabler struct {
	t       *domain.Tenant
	updates int
}

func (s *stubDisabler) GetByAppID(ctx context.Context, appID string) (*domain.Tenant, error) {
	if s.t == nil || s.t.AppID != appID {
		return nil, domain.ErrNotFound
	}
	cp := *s.t
	return &cp, nil
}

func (s *stubDisabler) Update(ctx context.Context, t *domain.Tenant) error {
	s.updates++
	*s.t = *t
	return nil
}

func TestBlockAuth_DisablesAndSkips(t *testing.T) {
	ten := &domain.Tenant{
		ID: "1", AppID: "app-1", Provider: "gobiz", Enabled: true,
		LoginMethod: "token", AccessToken: "x", RefreshToken: "r",
	}
	dis := &stubDisabler{t: ten}
	gate := NewAuthGate()
	src := NewSource(stubLister{tenants: []*domain.Tenant{ten}}, Options{
		Gate: gate, Disabler: dis, CacheDir: t.TempDir(),
	})

	src.blockAuth(context.Background(), "app-1", errors.Join(domain.ErrAuthFatal, errors.New("diblok sementara")))
	if dis.updates != 1 || ten.Enabled {
		t.Fatalf("updates=%d enabled=%v", dis.updates, ten.Enabled)
	}
	if _, ok := gate.Blocked("app-1"); !ok {
		t.Fatal("expected gate block")
	}

	providers, err := src.Providers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 0 {
		t.Fatalf("want 0 providers after block, got %d", len(providers))
	}
}

func TestGuardedProvider_PollAuthFatal(t *testing.T) {
	ten := &domain.Tenant{ID: "1", AppID: "app-1", Provider: "gobiz", Enabled: true}
	dis := &stubDisabler{t: ten}
	gate := NewAuthGate()
	src := NewSource(stubLister{tenants: []*domain.Tenant{ten}}, Options{
		Gate: gate, Disabler: dis, CacheDir: t.TempDir(),
	})
	g := &guardedProvider{
		appID: "app-1",
		inner: boomProvider{},
		src:   src,
	}
	_, err := g.Poll(context.Background())
	if !errors.Is(err, domain.ErrAuthFatal) {
		t.Fatalf("err=%v", err)
	}
	if ten.Enabled || dis.updates != 1 {
		t.Fatalf("enabled=%v updates=%d", ten.Enabled, dis.updates)
	}
}

type boomProvider struct{}

func (boomProvider) Name() string { return "app-1/gobiz" }
func (boomProvider) Poll(ctx context.Context) ([]domain.IncomingPayment, error) {
	return nil, errors.Join(domain.ErrAuthFatal, errors.New("Anda telah diblok sementara"))
}
