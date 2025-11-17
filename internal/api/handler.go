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

	e.POST("/users/setIsActive", h.SetUserIsActive, AuthMiddleware(auth.TokenTypeAdmin))
	e.GET("/users/getReview", h.GetUserReview, AuthMiddleware(auth.TokenTypeUser, auth.TokenTypeAdmin))
	e.GET("/stats", h.GetStats, AuthMiddleware(auth.TokenTypeAdmin))

	e.GET("/team/get", h.GetTeam, AuthMiddleware(auth.TokenTypeUser, auth.TokenTypeAdmin))
	e.POST("/team/add", h.AddTeam, AuthMiddleware(auth.TokenTypeAdmin))
	e.POST("/team/deactivateMembers", h.DeactivateTeamMembers, AuthMiddleware(auth.TokenTypeAdmin))

	e.POST("/pullRequest/create", h.CreatePullRequest, AuthMiddleware(auth.TokenTypeAdmin))
	e.POST("/pullRequest/merge", h.MergePullRequest, AuthMiddleware(auth.TokenTypeAdmin))
	e.POST("/pullRequest/reassign", h.ReassignPullRequest, AuthMiddleware(auth.TokenTypeAdmin))
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
		return h.transportError(e, model.NewError(model.ErrorCodeUnspecified, "failed to generate token"))
	}

	return e.JSON(http.StatusOK, token)
}

func (h *Handler) decodeRequest(e echo.Context, req any) *model.Error {
	if err := e.Bind(req); err != nil {
		return model.NewError(model.ErrorCodeInvalidBody, "invalid request body")
	}

	if err := e.Validate(req); err != nil {
		return model.NewError(model.ErrorCodeInvalidBody, errors.Wrap(err, "request validation failed").Error())
	}
	return nil
}

func (h *Handler) transportError(e echo.Context, err *model.Error) error {
	response := struct {
		Error *model.Error `json:"error"`
	}{Error: err}

	switch err.Code {
	case model.ErrorCodeNotFound:
		return e.JSON(http.StatusNotFound, response)
	case model.ErrorCodeTeamExists:
		return e.JSON(http.StatusBadRequest, response)
	case model.ErrorCodePRExists, model.ErrorCodePRMerged, model.ErrorCodeNotAssigned, model.ErrorCodeNoCandidate:
		return e.JSON(http.StatusConflict, response)
	case model.ErrorCodeInvalidBody:
		return e.JSON(http.StatusBadRequest, response)
	case model.ErrorCodeUserInactive:
		return e.JSON(http.StatusConflict, response)
	default:
		return e.JSON(http.StatusInternalServerError, response)
	}
}
