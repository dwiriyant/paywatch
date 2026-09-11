package auth

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/dwiriyant/paywatch/internal/response"
)

func AdminAuth(token string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if token == "" {
				return c.JSON(http.StatusServiceUnavailable, response.ErrorBody{Error: "admin not configured"})
			}
			auth := c.Request().Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(auth, prefix) || auth[len(prefix):] != token {
				return response.Unauthorized(c)
			}
			return next(c)
		}
	}
}
