package api

import (
	"github.com/labstack/echo/v4"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
	"time"
)

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

func GetLoggerFromContext(c echo.Context) *zap.Logger {
	if l, ok := c.Get("l").(*zap.Logger); ok {
		return l
	}
	return zap.NewNop()
}
