package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"unicode"

	charmlog "charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

// Handler formats logs with Charm and cleans terminal control characters for display.
type Handler struct {
	slog.Handler
}

// NewHandler returns a terminal-aware handler with quoted multiline fields.
func NewHandler(out io.Writer, opts *slog.HandlerOptions) *Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	level := slog.LevelInfo
	if opts.Level != nil {
		level = opts.Level.Level()
	}

	logger := charmlog.NewWithOptions(out, charmlog.Options{
		Level:           charmlog.Level(level),
		ReportTimestamp: true,
		TimeFormat:      "15:04:05",
		ReportCaller:    opts.AddSource,
	})
	if os.Getenv("NO_COLOR") != "" {
		logger.SetColorProfile(colorprofile.NoTTY)
	}

	return &Handler{Handler: logger}
}

// Handle cleans a copy of the record; prompt matching and recordings keep the original bytes.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error { //nolint:gocritic
	clean := slog.NewRecord(r.Time, r.Level, cleanText(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		clean.AddAttrs(cleanAttr(a))

		return true
	})

	return h.Handler.Handle(ctx, clean)
}

// WithAttrs keeps display cleaning on child loggers.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, len(attrs))
	for idx, attr := range attrs {
		clean[idx] = cleanAttr(attr)
	}

	return &Handler{Handler: h.Handler.WithAttrs(clean)}
}

// WithGroup keeps display cleaning on grouped loggers.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{Handler: h.Handler.WithGroup(cleanText(name))}
}

func cleanAttr(a slog.Attr) slog.Attr {
	a.Key = cleanText(a.Key)
	a.Value = a.Value.Resolve()

	switch a.Value.Kind() { //nolint:exhaustive // Scalar values need no display cleaning.
	case slog.KindString:
		a.Value = slog.StringValue(cleanText(a.Value.String()))
	case slog.KindGroup:
		attrs := a.Value.Group()
		clean := make([]slog.Attr, len(attrs))
		for idx, attr := range attrs {
			clean[idx] = cleanAttr(attr)
		}

		a.Value = slog.GroupValue(clean...)
	case slog.KindAny:
		a.Value = slog.StringValue(cleanText(fmt.Sprintf("%+v", a.Value.Any())))
	}

	return a
}

func cleanText(s string) string {
	s = ansi.Strip(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\t", "    ")

	return strings.Map(func(r rune) rune {
		if r != '\n' && unicode.IsControl(r) {
			return -1
		}

		return r
	}, s)
}
