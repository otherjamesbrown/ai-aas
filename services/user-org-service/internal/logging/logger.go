package logging

import (
	"github.com/ai-aas/shared-go/logging"
	"go.uber.org/zap"
)

// New creates a logger using the shared logging package.
// This is a compatibility wrapper that maintains the same function signature
// while using the shared logging package internally.
func New(serviceName, level string) *zap.Logger {
	cfg := logging.DefaultConfig().
		WithServiceName(serviceName).
		WithLogLevel(level)

	logger := logging.MustNew(cfg)
	return logger.Logger
}
