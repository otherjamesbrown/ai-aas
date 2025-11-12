package logging

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// New builds a zerolog.Logger configured with the provided service metadata.
func New(serviceName, level string) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339

	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	logger := zerolog.New(os.Stdout).
		With().
		Timestamp().
		Str("service", serviceName).
		Logger().
		Level(lvl)

	return logger
}
