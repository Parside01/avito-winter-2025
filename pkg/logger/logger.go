package logger

import (
	"go.uber.org/zap"
)

func NewLogger() (*zap.Logger, error) {
	// Use production logger by default — structured, performant.
	return zap.NewProduction()
}
