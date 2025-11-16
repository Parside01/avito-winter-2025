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
	"time"
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
	//e.POST("/token/generate", h.GenerateToken)

	e.GET("/team/get", h.GetTeam, AuthMiddleware(auth.TokenTypeUser, auth.TokenTypeAdmin))
	e.GET("/users/getReview", h.GetUserReview, AuthMiddleware(auth.TokenTypeUser, auth.TokenTypeAdmin))
	e.GET("/stats", h.GetStats, AuthMiddleware(auth.TokenTypeAdmin))

	e.POST("/team/add", h.AddTeam, AuthMiddleware(auth.TokenTypeAdmin))
	e.POST("/team/deactivateMembers", h.DeactivateTeamMembers, AuthMiddleware(auth.TokenTypeAdmin))

	e.POST("/users/setIsActive", h.SetUserIsActive, AuthMiddleware(auth.TokenTypeAdmin))
	e.POST("/pullRequest/create", h.CreatePullRequest, AuthMiddleware(auth.TokenTypeAdmin))
	e.POST("/pullRequest/merge", h.MergePullRequest, AuthMiddleware(auth.TokenTypeAdmin))
	e.POST("/pullRequest/reassign", h.ReassignPullRequest, AuthMiddleware(auth.TokenTypeAdmin))
}

func (h *Handler) DeactivateTeamMembers(e echo.Context) error {
	l := logger.FromContext(e.Request().Context())

	var req struct {
		TeamName string   `json:"team_name" validate:"required"`
		Users    []string `json:"users" validate:"required,min=1"`
	}

	if err := h.decodeRequest(e, &req); err != nil {
		l.Error("invalid request", zap.Any("error", err))
		return h.transportError(e, err)
	}

	l.Info("deactivating team members",
		zap.String("team_name", req.TeamName),
		zap.Strings("users", req.Users))

	if err := h.pr.DeactivateTeamMembers(e.Request().Context(), req.TeamName, req.Users); err != nil {
		l.Error("failed to deactivate team members",
			zap.String("team_name", req.TeamName),
			zap.Any("error", err))
		return h.transportError(e, err)
	}

	return e.NoContent(http.StatusOK)
}

func (h *Handler) GenerateToken(e echo.Context) error {
	l := logger.FromContext(e.Request().Context())

	var req struct {
		Type     auth.TokenType `json:"type" validate:"required,oneof=user admin"`
		Duration time.Duration  `json:"duration" validate:"required,gt=0"`
	}

	if err := h.decodeRequest(e, &req); err != nil {
		l.Error("invalid request", zap.Any("error", err))
		return h.transportError(e, err)
	}

	l.Info("generating token", zap.String("type", string(req.Type)), zap.Duration("duration", req.Duration))

	token, err := auth.GenerateToken(req.Type, req.Duration)
	if err != nil {
		l.Error("failed to generate token", zap.Any("error", err))
		return h.transportError(e, service.model.NewError(service.model.ErrorCodeUnspecified, "failed to generate token"))
	}

	return e.JSON(http.StatusOK, token)
}

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

	if err := h.decodeRequest(e, team); err != nil {
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
		return service.model.NewError(service.model.ErrorCodeInvalidBody, "invalid request body")
	}

	if err := e.Validate(req); err != nil {
		return service.model.NewError(service.model.ErrorCodeInvalidBody, errors.Wrap(err, "request validation failed").Error())
	}
	return nil
}

func (h *Handler) transportError(e echo.Context, err *service.Error) error {
	response := struct {
		Error *service.Error `json:"error"`
	}{Error: err}

	switch err.Code {
	case service.model.ErrorCodeNotFound:
		return e.JSON(http.StatusNotFound, response)
	case service.model.ErrorCodeTeamExists:
		return e.JSON(http.StatusBadRequest, response)
	case service.model.ErrorCodePRExists, service.model.ErrorCodePRMerged, service.model.ErrorCodeNotAssigned, service.model.ErrorCodeNoCandidate:
		return e.JSON(http.StatusConflict, response)
	case service.model.ErrorCodeInvalidBody:
		return e.JSON(http.StatusBadRequest, response)
	case service.model.ErrorCodeUserInactive:
		return e.JSON(http.StatusConflict, response)
	default:
		return e.JSON(http.StatusInternalServerError, response)
	}
}
