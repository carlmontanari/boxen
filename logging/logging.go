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

// NewLogger returns a logger with readable, terminal-aware console output.
func NewLogger(level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}

	return slog.New(NewHandler(os.Stdout, opts))
}
