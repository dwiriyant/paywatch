package gobiz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/dwiriyant/paywatch/internal/domain"
)

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	apiErrorBody
}

// LoginPassword exchanges email/password for an access token.
func (c *Client) LoginPassword(ctx context.Context, email, password string) error {
	headers := c.authHeaders(false)

	var reqOut apiErrorBody
	status, err := c.doJSON(ctx, http.MethodPost, baseURL+"/goid/login/request", headers, map[string]any{
		"email":      email,
		"login_type": "password",
		"client_id":  clientID,
	}, &reqOut)
	if err != nil {
		return err
	}
	_ = status // validation warnings are non-fatal

	var tok tokenResponse
	status, err = c.doJSON(ctx, http.MethodPost, baseURL+"/goid/token", headers, map[string]any{
		"client_id":  clientID,
		"grant_type": "password",
		"data":       map[string]string{"email": email, "password": password},
	}, &tok)
	if err != nil {
		return err
	}
	if status >= 400 || tok.AccessToken == "" {
		return loginAuthError("gobiz password login failed", tok.message(), status)
	}
	c.applyTokenResponse(tok)
	return nil
}

// LoginOTPRequest sends an OTP to the phone and returns otp_token when present.
func (c *Client) LoginOTPRequest(ctx context.Context, phone string) (otpToken string, err error) {
	normalized := normalizePhone(phone)
	headers := c.authHeaders(false)

	var out map[string]any
	status, err := c.doJSON(ctx, http.MethodPost, baseURL+"/goid/login/request", headers, map[string]any{
		"client_id":     clientID,
		"phone_number":  normalized,
		"country_code":  "62",
		"login_type":    "otp",
	}, &out)
	if err != nil {
		return "", err
	}
	if status >= 400 {
		var ae apiErrorBody
		_ = mapToStruct(out, &ae)
		return "", fmt.Errorf("gobiz otp request failed: %s", ae.message())
	}
	data, _ := out["data"].(map[string]any)
	if data != nil {
		if v, ok := data["otp_token"].(string); ok {
			return v, nil
		}
		if v, ok := data["token"].(string); ok {
			return v, nil
		}
	}
	if v, ok := out["otp_token"].(string); ok {
		return v, nil
	}
	return "", nil
}

// LoginOTPVerify exchanges OTP (+ optional otp_token) for an access token.
func (c *Client) LoginOTPVerify(ctx context.Context, phone, otp, otpToken string) error {
	headers := c.authHeaders(false)
	data := map[string]string{"otp": otp}
	if otpToken != "" {
		data["otp_token"] = otpToken
	} else {
		data["phone_number"] = normalizePhone(phone)
	}

	var tok tokenResponse
	status, err := c.doJSON(ctx, http.MethodPost, baseURL+"/goid/token", headers, map[string]any{
		"client_id":  clientID,
		"grant_type": "otp",
		"data":       data,
	}, &tok)
	if err != nil {
		return err
	}
	if status >= 400 || tok.AccessToken == "" {
		return loginAuthError("gobiz otp login failed", tok.message(), status)
	}
	c.applyTokenResponse(tok)
	return nil
}

// RefreshAccessToken exchanges a refresh_token for a new access token (unofficial /goid/token).
func (c *Client) RefreshAccessToken(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		refreshToken = c.refresh
	}
	if refreshToken == "" {
		return fmt.Errorf("%w: missing refresh token", domain.ErrAuthFatal)
	}
	headers := c.authHeaders(false)
	var tok tokenResponse
	status, err := c.doJSON(ctx, http.MethodPost, baseURL+"/goid/token", headers, map[string]any{
		"client_id":  clientID,
		"grant_type": "refresh_token",
		"data":       map[string]string{"refresh_token": refreshToken},
	}, &tok)
	if err != nil {
		return err
	}
	if status >= 400 || tok.AccessToken == "" {
		// fallback: some gateways expect refresh_token at top level
		status, err = c.doJSON(ctx, http.MethodPost, baseURL+"/goid/token", headers, map[string]any{
			"client_id":     clientID,
			"grant_type":    "refresh_token",
			"refresh_token": refreshToken,
		}, &tok)
		if err != nil {
			return err
		}
		if status >= 400 || tok.AccessToken == "" {
			return loginAuthError("gobiz refresh failed", tok.message(), status)
		}
	}
	if tok.RefreshToken == "" {
		tok.RefreshToken = refreshToken // keep previous if API omits rotation
	}
	c.applyTokenResponse(tok)
	return nil
}

func (c *Client) applyTokenResponse(tok tokenResponse) {
	c.token = tok.AccessToken
	if tok.RefreshToken != "" {
		c.refresh = tok.RefreshToken
	}
}

func (c *Client) TokenValid(ctx context.Context) bool {
	if c.token == "" {
		return false
	}
	status, err := c.doJSON(ctx, http.MethodPost, baseURL+"/v1/merchants/search", c.authHeaders(true), map[string]any{
		"from": 0, "to": 1, "_source": []string{"id"},
	}, &map[string]any{})
	if err != nil {
		return false
	}
	return status != http.StatusUnauthorized
}

func (c *Client) ResolveMerchantID(ctx context.Context) (string, error) {
	var out map[string]any
	status, err := c.doJSON(ctx, http.MethodPost, baseURL+"/v1/merchants/search", c.authHeaders(true), map[string]any{
		"from": 0, "to": 50, "_source": []string{"id", "merchant_name"},
	}, &out)
	if err != nil {
		return "", err
	}
	if status == http.StatusUnauthorized {
		return "", fmt.Errorf("gobiz unauthorized while listing merchants")
	}
	if status >= 400 {
		return "", fmt.Errorf("gobiz merchant search status %d", status)
	}
	list := extractMerchants(out)
	if len(list) == 0 {
		return "", fmt.Errorf("gobiz: no merchants on account")
	}
	id, _ := list[0]["id"].(string)
	if id == "" {
		id, _ = list[0]["merchant_id"].(string)
	}
	if id == "" {
		return "", fmt.Errorf("gobiz: merchant id missing in response")
	}
	c.merch = id
	return id, nil
}

func normalizePhone(phone string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, strings.TrimSpace(phone))
	switch {
	case strings.HasPrefix(digits, "62"):
		return digits[2:]
	case strings.HasPrefix(digits, "0"):
		return digits[1:]
	default:
		return digits
	}
}

func extractMerchants(out map[string]any) []map[string]any {
	if arr, ok := out["merchants"].([]any); ok {
		return asMaps(arr)
	}
	if hits, ok := out["hits"].(map[string]any); ok {
		if arr, ok := hits["hits"].([]any); ok {
			res := make([]map[string]any, 0, len(arr))
			for _, h := range arr {
				m, _ := h.(map[string]any)
				if m == nil {
					continue
				}
				if src, ok := m["_source"].(map[string]any); ok {
					res = append(res, src)
				} else {
					res = append(res, m)
				}
			}
			return res
		}
	}
	if arr, ok := out["hits"].([]any); ok {
		return asMaps(arr)
	}
	if arr, ok := out["data"].([]any); ok {
		return asMaps(arr)
	}
	return nil
}

func asMaps(arr []any) []map[string]any {
	out := make([]map[string]any, 0, len(arr))
	for _, v := range arr {
		if m, ok := v.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func mapToStruct(in map[string]any, out *apiErrorBody) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func loginAuthError(prefix, msg string, status int) error {
	// ponytail: failed password/otp exchange is always fatal — retrying locks GoBiz accounts
	_ = status
	return fmt.Errorf("%w: %s: %s", domain.ErrAuthFatal, prefix, msg)
}

// isAuthFatalMessage detects wrong-password / lockout style GoBiz errors (id + en).
func isAuthFatalMessage(msg string) bool {
	m := strings.ToLower(msg)
	needles := []string{
		"diblok", "blokir", "terlalu banyak", "kesalahan saat mencoba masuk",
		"password", "kata sandi", "salah", "invalid", "unauthorized",
		"credential", "tidak valid", "login gagal", "try again in", "coba lagi",
	}
	for _, n := range needles {
		if strings.Contains(m, n) {
			return true
		}
	}
	return false
}
