package api

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"github.com/yakoovad/avito-winter-2025/internal/auth"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/internal/service"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
	"net/http"
)

type Handler struct {
	pr   *service.PullRequestService
	team *service.TeamService
	user *service.UserService

	healthChecker HealthChecker

	logger *zap.Logger
}

func NewHandler(logger *zap.Logger) *Handler {
	return &Handler{
		logger: logger,
	}
}

func (h *Handler) WithHealthChecker(c HealthChecker) *Handler {
	h.healthChecker = c
	return h
}

func (h *Handler) WithTeamService(team *service.TeamService) *Handler {
	h.team = team
	return h
}

func (h *Handler) WithUserService(user *service.UserService) *Handler {
	h.user = user
	return h
}

func (h *Handler) WithPullRequestService(pr *service.PullRequestService) *Handler {
	h.pr = pr
	return h
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.Validator = NewValidator()
	e.Use(middleware.RequestID())
	e.Use(ZapLoggerMiddleware(h.logger))
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.GET("/health", h.healthChecker.HealthCheck())

	userSecurity := e.Group("", AuthMiddleware(auth.TokenTypeUser, auth.TokenTypeAdmin))

	userSecurity.POST("/team/get", h.GetTeam)
	userSecurity.GET("/users/getReview", h.GetUserReview)

	adminSecurity := e.Group("", AuthMiddleware(auth.TokenTypeAdmin))

	adminSecurity.POST("/team/add", h.AddTeam)
	adminSecurity.POST("/users/setIsActive", h.SetUserIsActive)
	adminSecurity.POST("/pullRequest/create", h.CreatePullRequest)
	adminSecurity.POST("/pullRequest/merge", h.MergePullRequest)
	adminSecurity.POST("/pullRequest/reassign", h.ReassignPullRequest)
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

	return e.JSON(http.StatusOK, pr)
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

	return e.JSON(http.StatusCreated, pr)
}

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

func (h *Handler) AddTeam(e echo.Context) error {
	l := logger.FromContext(e.Request().Context())

	team := &model.Team{}

	if err := h.decodeRequest(e, &team); err != nil {
		l.Error("invalid request", zap.Any("error", err))
		return h.transportError(e, err)
	}

	l.Info("adding team", zap.String("team_name", team.Name))

	if err := h.team.AddTeam(e.Request().Context(), team); err != nil {
		l.Error("failed to add team", zap.String("team_name", team.Name), zap.Any("error", err))
		return h.transportError(e, err)
	}

	return e.JSON(http.StatusCreated, team)
}

func (h *Handler) GetTeam(e echo.Context) error {
	l := logger.FromContext(e.Request().Context())

	teamName := e.QueryParam("team_name")

	l.Info("getting team", zap.String("team_name", teamName))

	team, err := h.team.GetTeam(e.Request().Context(), teamName)
	if err != nil {
		l.Error("failed to get team", zap.String("team_name", teamName), zap.Any("error", err))
		return h.transportError(e, err)
	}

	return e.JSON(http.StatusOK, team)
}

func (h *Handler) decodeRequest(e echo.Context, req any) *service.Error {
	if err := e.Bind(req); err != nil {
		return service.NewError(service.ErrorCodeInvalidBody, "invalid request body")
	}

	if err := e.Validate(req); err != nil {
		return service.NewError(service.ErrorCodeInvalidBody, errors.Wrap(err, "request validation failed").Error())
	}
	return nil
}

func (h *Handler) transportError(e echo.Context, err *service.Error) error {
	response := struct {
		Error *service.Error `json:"error"`
	}{Error: err}

	switch err.Code {
	case service.ErrorCodeNotFound:
		return e.JSON(http.StatusNotFound, response)
	case service.ErrorCodeTeamExists:
		return e.JSON(http.StatusBadRequest, response)
	case service.ErrorCodePRExists, service.ErrorCodePRMerged, service.ErrorCodeNotAssigned, service.ErrorCodeNoCandidate:
		return e.JSON(http.StatusConflict, response)
	case service.ErrorCodeInvalidBody:
		return e.JSON(http.StatusBadRequest, response)
	case service.ErrorCodeUserInactive:
		return e.JSON(http.StatusConflict, response)
	default:
		return e.JSON(http.StatusInternalServerError, response)
	}
}
