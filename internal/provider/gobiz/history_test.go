package gobiz

import "testing"

func TestMapAnalytics_CentsToRupiah(t *testing.T) {
	got := mapAnalytics([]analyticsTx{{
		TransactionID:   "tx1",
		GrossAmount:     250000, // cents
		TransactionTime: "2026-09-11T10:00:00Z",
	}})
	if len(got) != 1 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Amount != 2500 {
		t.Fatalf("amount=%d", got[0].Amount)
	}
	if got[0].Provider != "gobiz" || got[0].ExternalID != "tx1" {
		t.Fatalf("%+v", got[0])
	}
}
