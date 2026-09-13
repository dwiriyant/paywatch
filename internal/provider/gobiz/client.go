package gobiz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	baseURL  = "https://api.gobiz.co.id"
	clientID = "go-biz-web-new"
)

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	http    httpDoer
	token   string
	refresh string
	merch   string
	unique  string
}

func NewClient(httpClient httpDoer) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{http: httpClient, unique: uuid.NewString()}
}

func (c *Client) SetToken(token string)          { c.token = token }
func (c *Client) Token() string                  { return c.token }
func (c *Client) SetRefreshToken(token string)   { c.refresh = token }
func (c *Client) RefreshToken() string           { return c.refresh }
func (c *Client) SetMerchantID(id string)        { c.merch = id }
func (c *Client) MerchantID() string             { return c.merch }

func (c *Client) authHeaders(withToken bool) http.Header {
	h := make(http.Header)
	h.Set("Accept", "application/json, text/plain, */*")
	h.Set("Accept-Language", "id")
	h.Set("Authentication-Type", "go-id")
	h.Set("Content-Type", "application/json")
	h.Set("Gojek-Country-Code", "ID")
	h.Set("Gojek-Timezone", "Asia/Jakarta")
	h.Set("Origin", "https://portal.gofoodmerchant.co.id")
	h.Set("Referer", "https://portal.gofoodmerchant.co.id/")
	h.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36")
	h.Set("X-AppVersion", "platform-v3.107.0-94ce5d57")
	h.Set("X-PhoneMake", "Windows 10 64-bit")
	h.Set("X-PhoneModel", "Chrome 149.0.0.0 on Windows 10 64-bit")
	h.Set("X-Platform", "Web")
	h.Set("X-User-Locale", "en-US")
	h.Set("X-User-Type", "merchant")
	h.Set("x-DeviceOS", "Web")
	h.Set("x-appId", "go-biz-web-dashboard")
	h.Set("x-uniqueid", c.unique)
	if withToken && c.token != "" {
		h.Set("Authorization", "Bearer "+c.token)
	} else {
		h.Set("Authorization", "Bearer")
	}
	return h
}

func (c *Client) doJSON(ctx context.Context, method, url string, headers http.Header, body any, out any) (int, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return 0, err
	}
	req.Header = headers
	res, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return res.StatusCode, err
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return res.StatusCode, fmt.Errorf("decode %s: %w (body=%s)", url, err, truncate(string(raw), 200))
		}
	}
	return res.StatusCode, nil
}

type apiErrorBody struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (e apiErrorBody) message() string {
	if len(e.Errors) > 0 && e.Errors[0].Message != "" {
		return e.Errors[0].Message
	}
	return "unknown api error"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
