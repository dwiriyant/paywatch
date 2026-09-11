package settle

import (
	"context"
	"log/slog"

	"github.com/dwiriyant/paywatch/internal/domain"
	"github.com/dwiriyant/paywatch/internal/qrisgate"
)

// QRISGateSettler claims a pending payment on qrisgate for an incoming provider event.
type QRISGateSettler struct {
	Client *qrisgate.Client
	Log    *slog.Logger
	DryRun bool
}

func (s *QRISGateSettler) Settle(ctx context.Context, p domain.IncomingPayment) error {
	log := s.Log
	if log == nil {
		log = slog.Default()
	}
	if s.DryRun {
		log.Info("dry-run settle", "provider", p.Provider, "external_id", p.ExternalID, "amount", p.Amount)
		return nil
	}
	out, err := s.Client.Claim(ctx, p)
	if err != nil {
		return err
	}
	log.Info("settled", "payment_id", out.ID, "order_id", out.OrderID, "amount", out.Amount, "provider", p.Provider, "external_id", p.ExternalID)
	return nil
}
