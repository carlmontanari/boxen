package logging

import (
	"log/slog"
	"os"
	"strings"
)

// LevelFromString returns the slog level from the string, defaulting to info if no match.
func LevelFromString(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewLogger return a slog logger with the given name/leve/dev mode setting.
func NewLogger(level slog.Level) *slog.Logger {
	var h slog.Handler

	opts := &slog.HandlerOptions{
		Level: level,
	}

	if level == slog.LevelDebug {
		opts.AddSource = true
	}

	h = NewHandler(
		os.Stdout,
		opts,
	)

	return slog.New(h)
}
