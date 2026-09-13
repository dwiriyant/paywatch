package response

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type ErrorBody struct {
	Error string `json:"error"`
}

func OK(c echo.Context, body any) error {
	return c.JSON(http.StatusOK, body)
}

func Created(c echo.Context, body any) error {
	return c.JSON(http.StatusCreated, body)
}

func BadRequest(c echo.Context, msg string) error {
	return c.JSON(http.StatusBadRequest, ErrorBody{Error: msg})
}

func Unauthorized(c echo.Context) error {
	return c.JSON(http.StatusUnauthorized, ErrorBody{Error: "unauthorized"})
}

func TooManyRequests(c echo.Context) error {
	return c.JSON(http.StatusTooManyRequests, ErrorBody{Error: "rate limit exceeded"})
}

func NotFound(c echo.Context) error {
	return c.JSON(http.StatusNotFound, ErrorBody{Error: "not found"})
}

func Conflict(c echo.Context, msg string) error {
	return c.JSON(http.StatusConflict, ErrorBody{Error: msg})
}

func Internal(c echo.Context) error {
	return c.JSON(http.StatusInternalServerError, ErrorBody{Error: "internal server error"})
}
