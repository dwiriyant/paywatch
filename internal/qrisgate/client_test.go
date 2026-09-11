package qrisgate_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dwiriyant/paywatch/internal/domain"
	"github.com/dwiriyant/paywatch/internal/qrisgate"
)

func TestClient_ClaimAndMarkPaid(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/payments/claim", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer admin" {
			http.Error(w, "unauth", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(qrisgate.ClaimResponse{
			ID: "pay-1", OrderID: "ORD-1", Status: "paid", Amount: 2000,
		})
	})
	mux.HandleFunc("/v1/payments/pay-1/paid", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := qrisgate.NewClient(srv.URL, "admin")
	out, err := c.Claim(context.Background(), domain.IncomingPayment{
		AppID: "app-1", Provider: "gobiz", ExternalID: "tx", Amount: 2000, PaidAt: time.Now().UTC(),
	})
	if err != nil || out.ID != "pay-1" {
		t.Fatalf("%v %+v", err, out)
	}
	if err := c.MarkPaid(context.Background(), "pay-1"); err != nil {
		t.Fatal(err)
	}
}

func TestClient_ClaimNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c := qrisgate.NewClient(srv.URL, "admin")
	_, err := c.Claim(context.Background(), domain.IncomingPayment{AppID: "app-1", Amount: 1, Provider: "gobiz", ExternalID: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}
