package agent

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	boxenprofile "github.com/carlmontanari/boxen/profile"
)

// fakeConsoleOutput makes the agent read the given chunks, one per read call, then nothing.
func fakeConsoleOutput(a *Agent, chunks ...string) {
	a.readConsoleChunk = func() ([]byte, error) {
		if len(chunks) == 0 {
			return nil, nil
		}

		chunk := chunks[0]
		chunks = chunks[1:]

		return []byte(chunk), nil
	}
}

func readUntilStep(until string) *boxenprofile.Step {
	return &boxenprofile.Step{
		Type: boxenprofile.StepTypeReadUntil,
		ReadUntil: boxenprofile.StepReadUntil{
			Timeout: "1s",
			Until:   boxenprofile.Contains{Contains: until},
		},
	}
}

func TestReadUntilKeepsOutputPastMatch(t *testing.T) {
	a := NewAgent(slog.LevelError)

	// the save progress and the following prompt arrive in one read
	fakeConsoleOutput(a, "[#####] 100%\r\nCopy complete.\r\nswitch# ")

	if err := a.processStepReadUntil(t.Context(), readUntilStep("100%")); err != nil {
		t.Fatal(err)
	}

	if err := a.processStepReadUntil(t.Context(), readUntilStep("switch#")); err != nil {
		t.Fatalf("the prompt read by the previous step was lost: %v", err)
	}
}

func TestReadUntilMatchEarlyInLargeRead(t *testing.T) {
	a := NewAgent(slog.LevelError)

	fakeConsoleOutput(a, "FreeBSD/amd64"+strings.Repeat("boot output\r\n", 1_000))

	if err := a.processStepReadUntil(t.Context(), readUntilStep("FreeBSD/amd64")); err != nil {
		t.Fatalf("a match early in a large read was missed: %v", err)
	}
}

func TestReadUntilMatchSpanningReads(t *testing.T) {
	a := NewAgent(slog.LevelError)

	fakeConsoleOutput(a, "commit com", "plete\r\n")

	if err := a.processStepReadUntil(t.Context(), readUntilStep("commit complete")); err != nil {
		t.Fatalf("a match spanning reads was missed: %v", err)
	}
}

func TestReadUntilTimeout(t *testing.T) {
	a := NewAgent(slog.LevelError)

	fakeConsoleOutput(a, "nothing to see")

	start := time.Now()

	err := a.processStepReadUntil(t.Context(), readUntilStep("login:"))
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected a timeout, got %v", err)
	}

	if time.Since(start) > 5*time.Second {
		t.Fatal("timeout was not honored")
	}
}

func TestCaptureKeepsEndMarker(t *testing.T) {
	a := NewAgent(slog.LevelError)

	fakeConsoleOutput(a, "BEGIN\r\nconfig\r\nEND\r\nleaf1# ")

	output, err := a.readCapture(t.Context(), &boxenprofile.StepCapture{
		Start: boxenprofile.Contains{Contains: "BEGIN"},
		End:   boxenprofile.Contains{Contains: "END"},
	})
	if err != nil || output != "config\n" {
		t.Fatalf("got %q, %v", output, err)
	}

	if err := a.processStepReadUntil(t.Context(), readUntilStep("leaf1#")); err != nil {
		t.Fatalf("the prompt after the capture was lost: %v", err)
	}
}
