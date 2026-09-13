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
	cp.Password = ""
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
	cp.Password = ""
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

func okVerifier(ctx context.Context, email, password string) (admin.AuthSession, error) {
	if email == "" || password == "" {
		return admin.AuthSession{}, domain.ErrInvalidInput
	}
	return admin.AuthSession{AccessToken: "atk", RefreshToken: "rtk", MerchantID: "G123"}, nil
}

func TestCreateTenant_exchangesPasswordForTokens(t *testing.T) {
	store := &memStore{}
	svc := admin.NewService(store).WithPasswordVerifier(okVerifier)
	out, err := svc.Create(context.Background(), admin.CreateTenantInput{
		AppID: "app-1", Name: "Shop",
		Gobiz: &admin.GobizInput{LoginMethod: "password", Email: "a@b.c", Password: "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Enabled || out.HasPassword || !out.HasToken || !out.HasRefresh || out.LoginMethod != "token" {
		t.Fatalf("%+v", out)
	}
	got, _ := store.GetByID(context.Background(), out.ID)
	if got.Password != "" || got.AccessToken != "atk" || got.RefreshToken != "rtk" {
		t.Fatalf("%+v", got)
	}
}

func TestUpdateGobiz_disablesAndClearsPassword(t *testing.T) {
	store := &memStore{}
	svc := admin.NewService(store).WithPasswordVerifier(okVerifier)
	en := true
	created, err := svc.Create(context.Background(), admin.CreateTenantInput{
		AppID: "app-1", Enabled: &en,
		Gobiz: &admin.GobizInput{Email: "a@b.c", Password: "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.Update(context.Background(), created.ID, admin.UpdateTenantInput{
		Gobiz: &admin.GobizInput{Email: "a@b.c", Password: "new-secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Enabled || out.HasPassword {
		t.Fatalf("%+v", out)
	}
}

func TestVerify_ok(t *testing.T) {
	svc := admin.NewService(&memStore{}).WithPasswordVerifier(okVerifier)
	out, err := svc.Verify(context.Background(), admin.VerifyInput{
		Gobiz: &admin.GobizInput{LoginMethod: "password", Email: "a@b.c", Password: "secret"},
	})
	if err != nil || !out.OK || out.MerchantID != "G123" || !out.HasRefresh || out.SaveWindowSec != 60 {
		t.Fatalf("out=%+v err=%v", out, err)
	}
}

func TestVerifyThenCreate_usesPendingCache(t *testing.T) {
	store := &memStore{}
	var live int
	svc := admin.NewService(store).WithPasswordVerifier(func(ctx context.Context, email, password string) (admin.AuthSession, error) {
		live++
		return admin.AuthSession{AccessToken: "atk", RefreshToken: "rtk", MerchantID: "G1"}, nil
	})
	if _, err := svc.Verify(context.Background(), admin.VerifyInput{
		Gobiz: &admin.GobizInput{Email: "a@b.c", Password: "secret"},
	}); err != nil {
		t.Fatal(err)
	}
	if live != 1 {
		t.Fatalf("live=%d", live)
	}
	out, err := svc.Create(context.Background(), admin.CreateTenantInput{
		AppID: "app-1",
		Gobiz: &admin.GobizInput{Email: "a@b.c", Password: "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if live != 1 {
		t.Fatalf("create must reuse pending session, live=%d", live)
	}
	if !out.HasToken || out.MerchantID != "G1" {
		t.Fatalf("%+v", out)
	}
}

func TestVerify_authFail(t *testing.T) {
	svc := admin.NewService(&memStore{}).WithPasswordVerifier(func(ctx context.Context, email, password string) (admin.AuthSession, error) {
		return admin.AuthSession{}, errors.Join(domain.ErrAuthFatal, errors.New("gobiz password login failed: diblok"))
	})
	_, err := svc.Verify(context.Background(), admin.VerifyInput{
		Gobiz: &admin.GobizInput{Email: "a@b.c", Password: "x"},
	})
	if !errors.Is(err, domain.ErrAuthFatal) {
		t.Fatalf("err=%v", err)
	}
}
