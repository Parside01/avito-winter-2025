package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/yakoovad/avito-winter-2025/internal/api"
	"github.com/yakoovad/avito-winter-2025/internal/db"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
	"github.com/yakoovad/avito-winter-2025/internal/service"
)

func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://postgres:postgres@localhost:5432/avito_test?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	if err = pool.Ping(context.Background()); err != nil {
		panic(err)
	}

	transactor := db.NewPgxTransactor(pool)

	teamRepo := repository.NewPgxTeamRepository(pool)
	prRepo := repository.NewPgxPullRequestRepository(pool)
	userRepo := repository.NewPgxUserRepository(pool)
	reviewRepo := repository.NewPgxReviewRepository(pool)

	team := service.NewTeamService(transactor).WithTeamRepo(teamRepo).WithUserRepo(userRepo).WithReviewRepo(reviewRepo)
	user := service.NewUserService(transactor).WithUserRepo(userRepo).WithTeamRepo(teamRepo).WithReviewRepo(reviewRepo)
	pr := service.NewPullRequestService(transactor).WithPullRequestRepo(prRepo).WithTeamRepo(teamRepo).WithUserRepo(userRepo).WithReviewRepo(reviewRepo)

	e := echo.New()
	handler := api.NewHandler().WithTeamService(team).WithUserService(user).WithPullRequestService(pr)

	handler.RegisterRoutes(e)
	if err = e.Start(":8080"); err != nil {
		panic(err)
	}
}
