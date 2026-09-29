package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"strings"
	"time"
)

const (
	// ColorStop is the ansi code to terminate colorizing output.
	ColorStop = "\033[0m"
)

// LogLevelColorMap is the mapping of log levels (debug/info/warn/error) to their respective
// color ansi color code (blue/green/yellow/red respectively).
var LogLevelColorMap = map[string]string{ //nolint:gochecknoglobals
	slog.LevelDebug.String(): "\033[34m",
	slog.LevelInfo.String():  "\033[32m",
	slog.LevelWarn.String():  "\033[33m",
	slog.LevelError.String(): "\033[31m",
}

// Handler is a slog handler , t does pretty color printing and things you wouldnt
// need/want in prod, but... this is a cli tool so we can go nutty.
type Handler struct {
	slog.Handler

	l *log.Logger
}

// NewHandler returns a new Handler instance.
func NewHandler(
	out io.Writer,
	opts *slog.HandlerOptions,
) *Handler {
	h := &Handler{
		Handler: slog.NewJSONHandler(out, opts),
		l:       log.New(out, "", 0),
	}

	return h
}

// Handle implements the slog handler interface.
func (h *Handler) Handle(_ context.Context, r slog.Record) error { //nolint: gocritic
	l := r.Level
	ls := l.String()

	if strings.Contains(ls, "+") {
		ls = ls[0 : len(ls)-2]
	}

	level := fmt.Sprintf(
		"%s%s%s",
		LogLevelColorMap[ls],
		ls,
		ColorStop,
	)

	fields := make(map[string]any, r.NumAttrs())
	r.Attrs(
		func(a slog.Attr) bool {
			fields[a.Key] = a.Value.Any()

			return true
		},
	)

	b, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}

	h.l.Println(r.Time.Format(time.RFC822), level, r.Message, string(b))

	return nil
}
