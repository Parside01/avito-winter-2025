package api

import (
	"github.com/hellofresh/health-go/v5"
	"github.com/labstack/echo/v4"
	"log"
)

type HealthChecker interface {
	HealthCheck() echo.HandlerFunc
}

type healthChecker struct {
	health *health.Health
}

func MustNewHealthChecker(checks ...health.Config) HealthChecker {
	h, _ := health.New(health.WithComponent(health.Component{Name: "app", Version: "v0.1.0"}))

	for _, check := range checks {
		if err := h.Register(check); err != nil {
			log.Fatal("failed to register health check:", err)
		}
	}

	return &healthChecker{
		health: h,
	}
}

func (h *healthChecker) HealthCheck() echo.HandlerFunc {
	return echo.WrapHandler(h.health.Handler())
}
