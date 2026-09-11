package qrisgate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dwiriyant/paywatch/internal/domain"
)

// Client talks to qrisgate admin APIs.
type Client struct {
	baseURL    string
	adminToken string
	http       *http.Client
}

func NewClient(baseURL, adminToken string) *Client {
	return &Client{
		baseURL:    baseURL,
		adminToken: adminToken,
		http:       &http.Client{Timeout: 15 * time.Second},
	}
}

// ClaimRequest asks qrisgate to match a provider pay-in to a pending payment and mark it paid.
// Endpoint (to be added on qrisgate): POST /v1/payments/claim
type ClaimRequest struct {
	AppID      string    `json:"app_id"`
	Amount     int64     `json:"amount"`
	Provider   string    `json:"provider"`
	ExternalID string    `json:"external_id"`
	PaidAt     time.Time `json:"paid_at"`
}

type ClaimResponse struct {
	ID      string `json:"id"`
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
	Amount  int64  `json:"amount"`
}

// Claim settles via qrisgate claim API (amount + provider external id).
func (c *Client) Claim(ctx context.Context, p domain.IncomingPayment) (*ClaimResponse, error) {
	if p.AppID == "" {
		return nil, fmt.Errorf("qrisgate claim: app_id required")
	}
	body, err := json.Marshal(ClaimRequest{
		AppID:      p.AppID,
		Amount:     p.Amount,
		Provider:   p.Provider,
		ExternalID: p.ExternalID,
		PaidAt:     p.PaidAt,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/payments/claim", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.adminToken)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("qrisgate claim: no pending payment matched amount=%d (body=%s)", p.Amount, truncate(string(raw), 200))
	}
	if res.StatusCode == http.StatusNotImplemented || res.StatusCode == http.StatusMethodNotAllowed {
		return nil, fmt.Errorf("qrisgate claim endpoint missing — add POST /v1/payments/claim (status=%d body=%s)", res.StatusCode, truncate(string(raw), 200))
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("qrisgate claim status %d: %s", res.StatusCode, truncate(string(raw), 200))
	}
	var out ClaimResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MarkPaid calls the existing admin endpoint when payment id is already known.
func (c *Client) MarkPaid(ctx context.Context, paymentID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/payments/"+paymentID+"/paid", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.adminToken)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		return fmt.Errorf("qrisgate mark paid status %d: %s", res.StatusCode, truncate(string(raw), 200))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
