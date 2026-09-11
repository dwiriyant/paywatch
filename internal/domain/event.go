package domain

import (
	"context"
	"time"
)

// IncomingPayment is a normalized pay-in detected by any provider for one qrisgate app.
type IncomingPayment struct {
	AppID      string    // qrisgate app_id (required for claim)
	Provider   string    // e.g. "gobiz"
	ExternalID string    // provider transaction id (dedupe key)
	Amount     int64     // rupiah (whole units)
	PaidAt     time.Time // UTC when known
}

// Settler notifies the payment source of truth (qrisgate) that a pay-in arrived.
type Settler interface {
	Settle(ctx context.Context, p IncomingPayment) error
}

// Provider polls a payment source for new pay-ins.
type Provider interface {
	Name() string
	// Poll returns recent pay-ins (newest first). Watcher handles seed/dedupe.
	Poll(ctx context.Context) ([]IncomingPayment, error)
}
