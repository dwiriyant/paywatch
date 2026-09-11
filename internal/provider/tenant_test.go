package provider_test

import (
	"context"
	"testing"

	"github.com/dwiriyant/paywatch/internal/domain"
	"github.com/dwiriyant/paywatch/internal/provider"
)

type stub struct{}

func (stub) Name() string { return "gobiz" }
func (stub) Poll(ctx context.Context) ([]domain.IncomingPayment, error) {
	return []domain.IncomingPayment{{Provider: "gobiz", ExternalID: "tx", Amount: 1000}}, nil
}

func TestTenant_stampsAppID(t *testing.T) {
	p := &provider.Tenant{AppID: "app-1", Inner: stub{}}
	list, err := p.Poll(context.Background())
	if err != nil || len(list) != 1 || list[0].AppID != "app-1" {
		t.Fatalf("%v %+v", err, list)
	}
	if p.Name() != "app-1/gobiz" {
		t.Fatal(p.Name())
	}
}
