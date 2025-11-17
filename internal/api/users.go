package api

import (
	"github.com/labstack/echo/v4"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
	"net/http"
)

func (h *Handler) SetUserIsActive(e echo.Context) error {
	l := logger.FromContext(e.Request().Context())

	var req struct {
		UserID   string `json:"user_id" validate:"required"`
		IsActive bool   `json:"is_active"`
	}

	if err := h.decodeRequest(e, &req); err != nil {
		l.Error("invalid request", zap.Any("error", err))
		return h.transportError(e, err)
	}

	l.Info("setting user active status",
		zap.String("user_id", req.UserID),
		zap.Bool("is_active", req.IsActive))

	user, err := h.user.SetUserIsActive(e.Request().Context(), req.UserID, req.IsActive)
	if err != nil {
		l.Error("failed to set user active status",
			zap.String("user_id", req.UserID),
			zap.Any("error", err))
		return h.transportError(e, err)
	}

	return e.JSON(http.StatusOK, user)
}

func (h *Handler) GetUserReview(e echo.Context) error {
	l := logger.FromContext(e.Request().Context())

	userID := e.QueryParam("user_id")

	l.Info("getting user reviews", zap.String("user_id", userID))

	reviews, err := h.pr.GetUserReview(e.Request().Context(), userID)
	if err != nil {
		l.Error("failed to get user reviews", zap.String("user_id", userID), zap.Any("error", err))
		return h.transportError(e, err)
	}

	return e.JSON(http.StatusOK, reviews)
}
