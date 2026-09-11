package admin

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/dwiriyant/paywatch/internal/domain"
	"github.com/dwiriyant/paywatch/internal/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Create(c echo.Context) error {
	var in CreateTenantInput
	if err := c.Bind(&in); err != nil {
		return response.BadRequest(c, "invalid json")
	}
	out, err := h.svc.Create(c.Request().Context(), in)
	if err != nil {
		return mapErr(c, err, "app_id and gobiz credentials required")
	}
	return response.Created(c, out)
}

func (h *Handler) List(c echo.Context) error {
	out, err := h.svc.List(c.Request().Context())
	if err != nil {
		return response.Internal(c)
	}
	return response.OK(c, map[string]any{"tenants": out})
}

func (h *Handler) Get(c echo.Context) error {
	out, err := h.svc.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapErr(c, err, "")
	}
	return response.OK(c, out)
}

func (h *Handler) Update(c echo.Context) error {
	var in UpdateTenantInput
	if err := c.Bind(&in); err != nil {
		return response.BadRequest(c, "invalid json")
	}
	out, err := h.svc.Update(c.Request().Context(), c.Param("id"), in)
	if err != nil {
		return mapErr(c, err, "invalid tenant update")
	}
	return response.OK(c, out)
}

func (h *Handler) Delete(c echo.Context) error {
	if err := h.svc.Delete(c.Request().Context(), c.Param("id")); err != nil {
		return mapErr(c, err, "")
	}
	return c.NoContent(http.StatusNoContent)
}

func mapErr(c echo.Context, err error, invalidMsg string) error {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		if invalidMsg == "" {
			invalidMsg = "invalid input"
		}
		return response.BadRequest(c, invalidMsg)
	case errors.Is(err, domain.ErrNotFound):
		return response.NotFound(c)
	case errors.Is(err, domain.ErrConflict):
		return response.Conflict(c, "app_id already registered")
	default:
		return response.Internal(c)
	}
}
