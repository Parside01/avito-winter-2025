package api

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/internal/service"
	"net/http"
)

type Handler struct {
	pr   *service.PullRequestService
	team *service.TeamService
	user *service.UserService
}

func NewHandler() *Handler {
	return &Handler{}
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
	e.Use(middleware.Logger())
	//e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.CORS())

	e.POST("/team/add", h.AddTeam)
	e.GET("/team/get", h.GetTeam)

	e.POST("/user/setIsActive", h.SetUserIsActive)
	e.GET("/users/getReview", h.GetUserReview)

	e.POST("/pullRequest/create", h.CreatePullRequest)
	e.POST("/pullRequest/merge", h.MergePullRequest)
	e.POST("/pullRequest/reassign", h.ReassignPullRequest)
}

func (h *Handler) GetUserReview(e echo.Context) error {
	userID := e.QueryParam("user_id")

	reviews, err := h.pr.GetUserReview(e.Request().Context(), userID)
	if err != nil {
		return e.JSON(http.StatusBadRequest, err)
	}

	return e.JSON(http.StatusOK, reviews)
}

// MergePullRequest TODO: Обработка ошибок по спеке.
func (h *Handler) ReassignPullRequest(e echo.Context) error {
	var req struct {
		ID     string `json:"pull_request_id" validate:"required"`
		UserID string `json:"old_user_id" validate:"required"`
	}

	if err := e.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, errors.Wrap(err, "invalid request body"))
	}

	if err := e.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, errors.Wrap(err, "validation error"))
	}

	pr, err := h.pr.ReassignPullRequest(e.Request().Context(), req.ID, req.UserID)
	if err != nil {
		return e.JSON(http.StatusBadRequest, err)
	}

	return e.JSON(http.StatusOK, pr)

}

// MergePullRequest TODO: Обработка ошибок по спеке.
func (h *Handler) MergePullRequest(e echo.Context) error {
	var req struct {
		ID string `json:"pull_request_id" validate:"required"`
	}

	if err := e.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, errors.Wrap(err, "invalid request body"))
	}

	if err := e.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, errors.Wrap(err, "validation error"))
	}

	pr, err := h.pr.MergePullRequest(e.Request().Context(), req.ID)
	if err != nil {
		return e.JSON(http.StatusBadRequest, err)
	}

	return e.JSON(http.StatusOK, pr)
}

// CreatePullRequest TODO: Обработка ошибок по спеке.
func (h *Handler) CreatePullRequest(e echo.Context) error {
	var req struct {
		ID       string `json:"pull_request_id" validate:"required"`
		Name     string `json:"pull_request_name" validate:"required"`
		AuthorID string `json:"author_id" validate:"required"`
	}

	if err := e.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, errors.Wrap(err, "invalid request body"))
	}

	if err := e.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, errors.Wrap(err, "validation error"))
	}

	short := &model.PullRequestShort{
		ID:       req.ID,
		Name:     req.Name,
		AuthorID: req.AuthorID,
	}

	pr, err := h.pr.CreatePullRequest(e.Request().Context(), short)
	if err != nil {
		return e.JSON(http.StatusBadRequest, err)
	}

	return e.JSON(http.StatusCreated, pr)
}

func (h *Handler) SetUserIsActive(e echo.Context) error {
	var req struct {
		UserID   string `json:"user_id" validate:"required"`
		IsActive bool   `json:"is_active"`
	}

	if err := e.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, errors.Wrap(err, "invalid request body"))
	}

	if err := e.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, errors.Wrap(err, "validation error"))
	}

	user, err := h.user.SetUserIsActive(e.Request().Context(), req.UserID, req.IsActive)
	if err != nil {
		return e.JSON(http.StatusNotFound, err)
	}

	return e.JSON(http.StatusOK, user)
}

func (h *Handler) AddTeam(e echo.Context) error {
	team := &model.Team{}

	if err := e.Bind(&team); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, errors.Wrap(err, "invalid request body"))
	}

	if err := e.Validate(team); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, errors.Wrap(err, "validation error"))
	}

	if err := h.team.AddTeam(e.Request().Context(), team); err != nil {
		return e.JSON(http.StatusBadRequest, err)
	}

	return e.JSON(http.StatusCreated, team)
}

func (h *Handler) GetTeam(e echo.Context) error {
	teamName := e.QueryParam("team_name")
	team, err := h.team.GetTeam(e.Request().Context(), teamName)
	if err != nil {
		return e.JSON(http.StatusBadRequest, err)
	}

	return e.JSON(http.StatusOK, team)
}
