package admin_test

import (
	"context"
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

func TestCreateTenant(t *testing.T) {
	svc := admin.NewService(&memStore{})
	en := true
	out, err := svc.Create(context.Background(), admin.CreateTenantInput{
		AppID: "app-1", Name: "Shop", Enabled: &en,
		Gobiz: &admin.GobizInput{LoginMethod: "password", Email: "a@b.c", Password: "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.AppID != "app-1" || !out.HasPassword || out.Email != "a@b.c" {
		t.Fatalf("%+v", out)
	}
}
