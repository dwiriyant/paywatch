package provider

import (
	"context"

	"github.com/dwiriyant/paywatch/internal/domain"
)

// Tenant stamps AppID onto every payment from an underlying provider.
type Tenant struct {
	AppID string
	Inner domain.Provider
}

func (t *Tenant) Name() string {
	return t.AppID + "/" + t.Inner.Name()
}

func (t *Tenant) Poll(ctx context.Context) ([]domain.IncomingPayment, error) {
	list, err := t.Inner.Poll(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].AppID = t.AppID
	}
	return list, nil
}
