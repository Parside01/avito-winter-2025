package main

import (
	"context"
	"github.com/hellofresh/health-go/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/spf13/cobra"
	"github.com/yakoovad/avito-winter-2025/internal/api"
	"github.com/yakoovad/avito-winter-2025/internal/db"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
	"github.com/yakoovad/avito-winter-2025/internal/service"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
	"log"
	"os"
	"time"
)

var env string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the http service",
	Run: func(cmd *cobra.Command, args []string) {
		_ = godotenv.Load(env)
		runServer()
	},
}

func init() {
	serveCmd.Flags().StringVarP(&env, "env", "e", ".env", "Path to .env file")
}

func runServer() {
	l, err := logger.NewLogger()
	if err != nil {
		log.Fatal("failed to initialize logger: ", err)
	}
	defer func() {
		_ = l.Sync()
	}()

	l.Info("starting application")

	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		l.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	if err = pool.Ping(context.Background()); err != nil {
		l.Fatal("failed to ping database", zap.Error(err))
	}

	l.Info("database connection established")

	transactor := db.NewPgxTransactor(pool, 5)

	teamRepo := repository.NewPgxTeamRepository(pool)
	prRepo := repository.NewPgxPullRequestRepository(pool)
	userRepo := repository.NewPgxUserRepository(pool)
	reviewRepo := repository.NewPgxReviewRepository(pool)

	team := service.NewTeamService(transactor).
		WithTeamRepo(teamRepo).
		WithUserRepo(userRepo).
		WithReviewRepo(reviewRepo)

	user := service.NewUserService(transactor).
		WithUserRepo(userRepo).
		WithTeamRepo(teamRepo).
		WithReviewRepo(reviewRepo)

	pr := service.NewPullRequestService(transactor).
		WithPullRequestRepo(prRepo).
		WithTeamRepo(teamRepo).
		WithUserRepo(userRepo).
		WithReviewRepo(reviewRepo)

	e := echo.New()

	healthChecker := api.MustNewHealthChecker(
		health.Config{
			Name: "database",
			Check: func(ctx context.Context) error {
				return transactor.Ping(ctx)
			},
			Timeout: time.Second * 5,
		},
	)

	handler := api.NewHandler(l).
		WithTeamService(team).
		WithUserService(user).
		WithPullRequestService(pr).
		WithHealthChecker(healthChecker)

	handler.RegisterRoutes(e)

	if err = e.Start(":8080"); err != nil {
		l.Fatal("fatal server error", zap.Error(err))
	}
}
