package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestHandlerReadableConsoleOutput(t *testing.T) {
	var out bytes.Buffer
	h := NewHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(h).With("component", "builder").WithGroup("console")
	raw := "\x00\x1b[32mBooting\x1b[0m\r\n\tReady\x07\r\n"
	logger.Debug(
		"console output",
		"content",
		raw,
		"until",
		struct{ Contains string }{Contains: "Ready"},
	)
	logger.Error("failed", "error", consoleTestError{})
	output := out.String()

	for _, want := range []string{
		"DEBU", "console", "component=builder", "│ Booting", "│     Ready", "Contains:Ready",
		"│ first", "│ second",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in output:\n%s", want, output)
		}
	}
	for _, unwanted := range []string{"\x00", "\x1b", "\r", "\x07", `\x00`, `\r`, `\n`, "{}"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("unexpected %q in output:\n%s", unwanted, output)
		}
	}

	// Formatting must not change the data used by console matching or other handlers.
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "guest", 0)
	record.AddAttrs(slog.Group("session", slog.String("content", raw)))
	if err := h.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	record.Attrs(func(attr slog.Attr) bool {
		if got := attr.Value.Group()[0].Value.String(); got != raw {
			t.Fatalf("original record changed: got %q, want %q", got, raw)
		}

		return true
	})
}

type consoleTestError struct{}

func (consoleTestError) Error() string {
	return "\x1b[31mfirst\r\nsecond\x1b[0m"
}

func TestHandlerLevelFiltering(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(NewHandler(&out, nil))
	logger.Debug("hidden")
	logger.Info("visible")
	if output := out.String(); strings.Contains(output, "hidden") ||
		!strings.Contains(output, "visible") {
		t.Fatalf("incorrect level filtering: %q", output)
	}
}

func TestHandlerNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var out bytes.Buffer
	logger := slog.New(NewHandler(&out, nil))
	logger.Info("booting", "step", 1)
	if output := out.String(); strings.Contains(output, "\x1b") {
		t.Fatalf("NO_COLOR output contains ANSI escapes: %q", output)
	}
}
