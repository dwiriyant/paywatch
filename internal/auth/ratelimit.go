package auth

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/dwiriyant/paywatch/internal/response"
	"golang.org/x/time/rate"
)

// VerifyRateLimit limits live credential-check endpoints (per client IP).
func VerifyRateLimit(perMin int) echo.MiddlewareFunc {
	if perMin <= 0 {
		perMin = 1
	}
	store := middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
		Rate:      rate.Limit(float64(perMin) / 60.0),
		Burst:     perMin,
		ExpiresIn: 3 * time.Minute,
	})
	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: store,
		DenyHandler: func(c echo.Context, identifier string, err error) error {
			return response.TooManyRequests(c)
		},
	})
}