package api

import (
	"github.com/labstack/echo/v4"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
	"net/http"
)

type DeactivateTeamMembersRequest struct {
	TeamName string   `json:"team_name" validate:"required"`
	Users    []string `json:"users" validate:"required,min=1"`
}

func (h *Handler) DeactivateTeamMembers(e echo.Context) error {
	l := logger.FromContext(e.Request().Context())

	req := DeactivateTeamMembersRequest{}

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
