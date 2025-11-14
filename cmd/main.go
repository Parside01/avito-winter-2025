package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/yakoovad/avito-winter-2025/internal/api"
	"github.com/yakoovad/avito-winter-2025/internal/db"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
	"github.com/yakoovad/avito-winter-2025/internal/service"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	// Инициализируем logger
	logger, err := logger.NewLogger()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	logger.Info("starting application")

	pool, err := pgxpool.New(context.Background(), "postgres://postgres:postgres@localhost:5432/avito_test?sslmode=disable")
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	if err = pool.Ping(context.Background()); err != nil {
		logger.Fatal("failed to ping database", zap.Error(err))
	}

	logger.Info("database connection established")

	transactor := db.NewPgxTransactor(pool)

	teamRepo := repository.NewPgxTeamRepository(pool)
	prRepo := repository.NewPgxPullRequestRepository(pool)
	userRepo := repository.NewPgxUserRepository(pool)
	reviewRepo := repository.NewPgxReviewRepository(pool)

	team := service.NewTeamService(transactor).WithTeamRepo(teamRepo).WithUserRepo(userRepo).WithReviewRepo(reviewRepo)
	user := service.NewUserService(transactor).WithUserRepo(userRepo).WithTeamRepo(teamRepo).WithReviewRepo(reviewRepo)
	pr := service.NewPullRequestService(transactor).WithPullRequestRepo(prRepo).WithTeamRepo(teamRepo).WithUserRepo(userRepo).WithReviewRepo(reviewRepo)

	e := echo.New()

	handler := api.NewHandler(logger).WithTeamService(team).WithUserService(user).WithPullRequestService(pr)

	handler.RegisterRoutes(e)

	logger.Info("server starting on :8080")
	if err = e.Start(":8080"); err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}
