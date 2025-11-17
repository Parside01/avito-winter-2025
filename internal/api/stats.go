package api

import (
	"github.com/labstack/echo/v4"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
	"net/http"
)

func (h *Handler) GetStats(e echo.Context) error {
	l := logger.FromContext(e.Request().Context())

	l.Info("getting statistics")

	stats, err := h.pr.GetStats(e.Request().Context())
	if err != nil {
		l.Error("failed to get statistics", zap.Any("error", err))
		return h.transportError(e, err)
	}

	return e.JSON(http.StatusOK, stats)
}
