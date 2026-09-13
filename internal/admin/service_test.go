package admin_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dwiriyant/paywatch/internal/admin"
	"github.com/dwiriyant/paywatch/internal/domain"
)

type memStore struct {
	byID map[string]*domain.Tenant
}

func (m *memStore) Create(ctx context.Context, t *domain.Tenant) error {
	if m.byID == nil {
		m.byID = map[string]*domain.Tenant{}
	}
	for _, x := range m.byID {
		if x.AppID == t.AppID {
			return domain.ErrConflict
		}
	}
	if t.ID == "" {
		t.ID = "t1"
	}
	cp := *t
	m.byID[t.ID] = &cp
	return nil
}
func (m *memStore) GetByID(ctx context.Context, id string) (*domain.Tenant, error) {
	t, ok := m.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *t
	return &cp, nil
}
func (m *memStore) List(ctx context.Context) ([]*domain.Tenant, error) {
	var out []*domain.Tenant
	for _, t := range m.byID {
		cp := *t
		out = append(out, &cp)
	}
	return out, nil
}
func (m *memStore) Update(ctx context.Context, t *domain.Tenant) error {
	if _, ok := m.byID[t.ID]; !ok {
		return domain.ErrNotFound
	}
	cp := *t
	m.byID[t.ID] = &cp
	return nil
}
func (m *memStore) Delete(ctx context.Context, id string) error {
	if _, ok := m.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.byID, id)
	return nil
}

func TestCreateTenant_defaultsDisabled(t *testing.T) {
	svc := admin.NewService(&memStore{})
	out, err := svc.Create(context.Background(), admin.CreateTenantInput{
		AppID: "app-1", Name: "Shop",
		Gobiz: &admin.GobizInput{LoginMethod: "password", Email: "a@b.c", Password: "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Enabled {
		t.Fatal("new tenants must be disabled until explicitly enabled")
	}
	if out.AppID != "app-1" || !out.HasPassword || out.Email != "a@b.c" {
		t.Fatalf("%+v", out)
	}
}

func TestUpdateGobiz_disablesPolling(t *testing.T) {
	store := &memStore{}
	svc := admin.NewService(store)
	en := true
	created, err := svc.Create(context.Background(), admin.CreateTenantInput{
		AppID: "app-1", Enabled: &en,
		Gobiz: &admin.GobizInput{LoginMethod: "password", Email: "a@b.c", Password: "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.Update(context.Background(), created.ID, admin.UpdateTenantInput{
		Gobiz: &admin.GobizInput{Password: "new-secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Enabled {
		t.Fatal("credential change must disable tenant")
	}
}

func TestVerify_ok(t *testing.T) {
	svc := admin.NewService(&memStore{}).WithPasswordVerifier(func(ctx context.Context, email, password string) (string, error) {
		if email != "a@b.c" || password != "secret" {
			return "", errors.Join(domain.ErrAuthFatal, errors.New("bad"))
		}
		return "G123", nil
	})
	out, err := svc.Verify(context.Background(), admin.VerifyInput{
		Gobiz: &admin.GobizInput{LoginMethod: "password", Email: "a@b.c", Password: "secret"},
	})
	if err != nil || !out.OK || out.MerchantID != "G123" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
}

func TestVerify_authFail(t *testing.T) {
	svc := admin.NewService(&memStore{}).WithPasswordVerifier(func(ctx context.Context, email, password string) (string, error) {
		return "", errors.Join(domain.ErrAuthFatal, errors.New("gobiz password login failed: diblok"))
	})
	_, err := svc.Verify(context.Background(), admin.VerifyInput{
		Gobiz: &admin.GobizInput{Email: "a@b.c", Password: "x"},
	})
	if !errors.Is(err, domain.ErrAuthFatal) {
		t.Fatalf("err=%v", err)
	}
}

func TestVerifyTenant_usesStored(t *testing.T) {
	store := &memStore{}
	svc := admin.NewService(store).WithPasswordVerifier(func(ctx context.Context, email, password string) (string, error) {
		return "G9", nil
	})
	created, err := svc.Create(context.Background(), admin.CreateTenantInput{
		AppID: "app-1",
		Gobiz: &admin.GobizInput{LoginMethod: "password", Email: "a@b.c", Password: "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.VerifyTenant(context.Background(), created.ID)
	if err != nil || !out.OK || out.MerchantID != "G9" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
}
