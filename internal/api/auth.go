package api

import (
	"github.com/labstack/echo/v4"
	"github.com/yakoovad/avito-winter-2025/internal/auth"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
	"net/http"
	"time"
)

type GenerateTokenRequest struct {
	Type     auth.TokenType `json:"type" validate:"required,oneof=user admin"`
	Duration time.Duration  `json:"duration" validate:"required,gt=0"`
}

func (h *Handler) GenerateToken(e echo.Context) error {
	l := logger.FromContext(e.Request().Context())

	req := GenerateTokenRequest{}

	if err := h.decodeRequest(e, &req); err != nil {
		l.Error("invalid request", zap.Any("error", err))
		return h.transportError(e, err)
	}

	l.Info("generating token", zap.String("type", string(req.Type)), zap.Duration("duration", req.Duration))

	token, err := auth.GenerateToken(req.Type, req.Duration)
	if err != nil {
		l.Error("failed to generate token", zap.Any("error", err))
		return h.transportError(e, model.NewError(model.ErrorCodeUnspecified, "failed to generate token"))
	}

	return e.JSON(http.StatusOK, token)
}
