package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestVerifyRateLimit(t *testing.T) {
	e := echo.New()
	rl := VerifyRateLimit(2)
	e.POST("/v", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) }, rl)

	hit := func() int {
		req := httptest.NewRequest(http.MethodPost, "/v", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := hit(); code != http.StatusNoContent {
		t.Fatalf("1st=%d", code)
	}
	if code := hit(); code != http.StatusNoContent {
		t.Fatalf("2nd=%d", code)
	}
	if code := hit(); code != http.StatusTooManyRequests {
		t.Fatalf("3rd=%d want 429", code)
	}
}
