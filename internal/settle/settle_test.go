package settle_test

import (
	"context"
	"testing"

	"github.com/dwiriyant/paywatch/internal/domain"
	"github.com/dwiriyant/paywatch/internal/settle"
)

func TestQRISGateSettler_DryRun(t *testing.T) {
	s := &settle.QRISGateSettler{DryRun: true}
	err := s.Settle(context.Background(), domain.IncomingPayment{
		AppID: "app-1", Provider: "gobiz", ExternalID: "tx", Amount: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
}
