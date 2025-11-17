package api

import (
	"github.com/labstack/echo/v4"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
	"net/http"
)

func (h *Handler) ReassignPullRequest(e echo.Context) error {
	l := logger.FromContext(e.Request().Context())

	var req struct {
		ID     string `json:"pull_request_id" validate:"required"`
		UserID string `json:"old_user_id" validate:"required"`
	}

	if err := h.decodeRequest(e, &req); err != nil {
		l.Error("invalid request", zap.Any("error", err))
		return h.transportError(e, err)
	}

	l.Info("reassigning pull request",
		zap.String("pr_id", req.ID),
		zap.String("old_user_id", req.UserID))

	pr, err := h.pr.ReassignPullRequest(e.Request().Context(), req.ID, req.UserID)
	if err != nil {
		l.Error("failed to reassign pull request",
			zap.String("pr_id", req.ID),
			zap.String("old_user_id", req.UserID),
			zap.Any("error", err))
		return h.transportError(e, err)
	}

	return e.JSON(http.StatusOK, map[string]interface{}{"pr": pr})
}

func (h *Handler) MergePullRequest(e echo.Context) error {
	l := logger.FromContext(e.Request().Context())

	var req struct {
		ID string `json:"pull_request_id" validate:"required"`
	}

	if err := h.decodeRequest(e, &req); err != nil {
		l.Error("invalid request", zap.Any("error", err))
		return h.transportError(e, err)
	}

	l.Info("merging pull request", zap.String("pr_id", req.ID))

	pr, err := h.pr.MergePullRequest(e.Request().Context(), req.ID)
	if err != nil {
		l.Error("failed to merge pull request", zap.String("pr_id", req.ID), zap.Any("error", err))
		return h.transportError(e, err)
	}

	return e.JSON(http.StatusOK, pr)
}

func (h *Handler) CreatePullRequest(e echo.Context) error {
	l := logger.FromContext(e.Request().Context())

	var req struct {
		ID       string `json:"pull_request_id" validate:"required"`
		Name     string `json:"pull_request_name" validate:"required"`
		AuthorID string `json:"author_id" validate:"required"`
	}

	if err := h.decodeRequest(e, &req); err != nil {
		l.Error("invalid request", zap.Any("error", err))
		return h.transportError(e, err)
	}

	l.Info("creating pull request",
		zap.String("pr_id", req.ID),
		zap.String("pr_name", req.Name),
		zap.String("author_id", req.AuthorID))

	short := &model.PullRequestShort{
		ID:       req.ID,
		Name:     req.Name,
		AuthorID: req.AuthorID,
	}

	pr, err := h.pr.CreatePullRequest(e.Request().Context(), short)
	if err != nil {
		l.Error("failed to create pull request",
			zap.String("pr_id", req.ID),
			zap.Any("error", err))
		return h.transportError(e, err)
	}

	return e.JSON(http.StatusCreated, map[string]interface{}{"pr": pr})
}
