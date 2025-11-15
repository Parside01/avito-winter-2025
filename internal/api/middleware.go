package api

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/yakoovad/avito-winter-2025/internal/auth"
	"github.com/yakoovad/avito-winter-2025/internal/service"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
	"net/http"
	"time"
)

func AuthMiddleware(types ...auth.TokenType) echo.MiddlewareFunc {
	return middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		Skipper:   middleware.DefaultSkipper,
		KeyLookup: "header:X-Api-Key,cookie:X-Api-Key,header:Authorization:Bearer ",
		Validator: func(t string, c echo.Context) (bool, error) {
			tokenType, valid := auth.IsValidToken(t)
			if !valid {
				return false, nil
			}
			if len(types) == 0 {
				return true, nil
			}

			for _, tt := range types {
				if tt == tokenType {
					return true, nil
				}
			}
			return false, nil
		},
		ErrorHandler: func(err error, c echo.Context) error {
			l := logger.FromContext(c.Request().Context())
			l.Error("unauthorized access attempt", zap.Error(err))

			return c.JSON(http.StatusUnauthorized, service.NewError(service.ErrorCodeUnauthorized, err.Error()))
		},
		ContinueOnIgnoredError: true,
	})
}

func ZapLoggerMiddleware(l *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			req := c.Request()
			res := c.Response()

			requestID := c.Response().Header().Get(echo.HeaderXRequestID)

			reqLogger := l.With(
				zap.String("request_id", requestID),
			)

			c.Set("logger", reqLogger)

			ctx := logger.WithLogger(req.Context(), reqLogger)
			c.SetRequest(req.WithContext(ctx))

			err := next(c)

			latency := time.Since(start)

			fields := []zap.Field{
				zap.String("method", req.Method),
				zap.String("uri", req.RequestURI),
				zap.String("remote_ip", c.RealIP()),
				zap.Int("status", res.Status),
				zap.Duration("latency", latency),
				zap.Int64("bytes_in", req.ContentLength),
				zap.Int64("bytes_out", res.Size),
			}

			if err != nil {
				fields = append(fields, zap.Error(err))
				reqLogger.Error("request failed", fields...)
			} else {
				reqLogger.Info("request completed", fields...)
			}

			return err
		}
	}
}
