package gobiz

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/dwiriyant/paywatch/internal/domain"
)

type analyticsTx struct {
	TransactionID   string  `json:"transaction_id"`
	ID              string  `json:"id"`
	OrderID         string  `json:"order_id"`
	GrossAmount     float64 `json:"gross_amount"` // cents
	TransactionTime string  `json:"transaction_time"`
}

type analyticsResp struct {
	Transactions []analyticsTx `json:"transactions"`
}

// History fetches recent settle/capture pay-ins (Analytics API, Journal fallback).
func (c *Client) History(ctx context.Context, days, size int) ([]domain.IncomingPayment, error) {
	if days < 1 {
		days = 1
	}
	if size < 1 {
		size = 30
	}
	list, err := c.historyAnalytics(ctx, days, size)
	if err == nil && len(list) > 0 {
		return list, nil
	}
	journal, jerr := c.historyJournal(ctx, days, size)
	if jerr != nil {
		if err != nil {
			return nil, fmt.Errorf("analytics: %v; journal: %w", err, jerr)
		}
		return nil, jerr
	}
	return journal, nil
}

func (c *Client) historyAnalytics(ctx context.Context, days, size int) ([]domain.IncomingPayment, error) {
	if c.merch == "" {
		return nil, fmt.Errorf("merchant id required")
	}
	start := time.Now().Add(-time.Duration(days) * 24 * time.Hour).UTC()
	end := time.Now().UTC()

	u, _ := url.Parse("https://api.gojekapi.com/merchant-analytics/v2/merchants/transactions")
	q := u.Query()
	q.Set("from", "0")
	q.Set("size", strconv.Itoa(size))
	q.Set("statuses", "SETTLEMENT,CAPTURE,REFUND,PARTIAL_REFUND")
	q.Set("payment_types", "QRIS,GOPAY,OFFLINE_CREDIT_CARD,OFFLINE_DEBIT_CARD,CREDIT_CARD")
	q.Set("start_time", start.Format(time.RFC3339))
	q.Set("end_time", end.Format(time.RFC3339))
	q.Set("merchant_ids", c.merch)
	u.RawQuery = q.Encode()

	headers := c.authHeaders(true)
	headers.Set("accept-language", "id-ID,id;q=0.9")

	var out analyticsResp
	status, err := c.doJSON(ctx, http.MethodGet, u.String(), headers, nil, &out)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized {
		return nil, errUnauthorized
	}
	if status >= 400 {
		return nil, fmt.Errorf("analytics status %d", status)
	}
	return mapAnalytics(out.Transactions), nil
}

func (c *Client) historyJournal(ctx context.Context, days, size int) ([]domain.IncomingPayment, error) {
	if c.merch == "" {
		return nil, fmt.Errorf("merchant id required")
	}
	start := time.Now().Add(-time.Duration(days) * 24 * time.Hour).UTC().Format(time.RFC3339)
	end := time.Now().UTC().Format(time.RFC3339)

	body := map[string]any{
		"from": 0,
		"size": size,
		"sort": map[string]any{"time": map[string]string{"order": "desc"}},
		"included_categories": map[string]any{
			"incoming": []string{"transaction_share", "action"},
		},
		"query": []any{
			map[string]any{
				"op": "and",
				"clauses": []any{
					map[string]any{
						"field": "metadata.transaction.status",
						"op":    "in",
						"value": []string{"settlement", "capture", "refund", "partial_refund"},
					},
					map[string]any{
						"field": "metadata.transaction.payment_type",
						"op":    "in",
						"value": []string{"qris", "gopay", "offline_credit_card", "offline_debit_card", "credit_card"},
					},
					map[string]any{
						"field": "metadata.transaction.transaction_time",
						"op":    "gte",
						"value": start,
					},
					map[string]any{
						"field": "metadata.transaction.transaction_time",
						"op":    "lte",
						"value": end,
					},
					map[string]any{
						"field": "metadata.transaction.merchant_id",
						"op":    "equal",
						"value": c.merch,
					},
				},
			},
		},
	}

	headers := c.authHeaders(true)
	headers.Set("accept", "application/json, text/plain, */*, application/vnd.journal.v1+json")

	var out map[string]any
	status, err := c.doJSON(ctx, http.MethodPost, baseURL+"/journals/search", headers, body, &out)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized {
		return nil, errUnauthorized
	}
	if status >= 400 {
		return nil, fmt.Errorf("journal status %d", status)
	}
	return mapJournal(out), nil
}

var errUnauthorized = fmt.Errorf("gobiz unauthorized")

func mapAnalytics(txs []analyticsTx) []domain.IncomingPayment {
	out := make([]domain.IncomingPayment, 0, len(txs))
	for _, tx := range txs {
		id := firstNonEmpty(tx.TransactionID, tx.ID, tx.OrderID)
		if id == "" {
			continue
		}
		paidAt, _ := time.Parse(time.RFC3339, tx.TransactionTime)
		out = append(out, domain.IncomingPayment{
			Provider:   "gobiz",
			ExternalID: id,
			Amount:     int64(tx.GrossAmount / 100),
			PaidAt:     paidAt.UTC(),
		})
	}
	return out
}

func mapJournal(out map[string]any) []domain.IncomingPayment {
	arr, _ := out["data"].([]any)
	res := make([]domain.IncomingPayment, 0, len(arr))
	for _, item := range arr {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		meta, _ := m["metadata"].(map[string]any)
		tx, _ := meta["transaction"].(map[string]any)
		if tx == nil {
			continue
		}
		id := firstNonEmpty(str(tx["transaction_id"]), str(tx["id"]), str(tx["order_id"]), str(m["id"]))
		if id == "" {
			continue
		}
		gross := num(tx["gross_amount"])
		paidAt, _ := time.Parse(time.RFC3339, str(tx["transaction_time"]))
		res = append(res, domain.IncomingPayment{
			Provider:   "gobiz",
			ExternalID: id,
			Amount:     int64(gross / 100),
			PaidAt:     paidAt.UTC(),
		})
	}
	return res
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}
