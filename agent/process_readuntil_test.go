package agent

import (
	"bytes"
	"context"
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

func TestWaitForEchoIgnoresSurroundingWhitespace(t *testing.T) {
	a := NewAgent(slog.LevelError)

	// the CLI does not echo the indentation of the written line
	fakeConsoleOutput(a, "csr1(config-cert-chain)#  quit\r\ncsr1(config)#")

	if err := a.waitForEcho(t.Context(), "      quit"); err != nil {
		t.Fatal(err)
	}

	// output after the echo stays available
	if err := a.processStepReadUntil(t.Context(), readUntilStep("csr1(config)#")); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForEchoSkipsBlankLines(t *testing.T) {
	a := NewAgent(slog.LevelError)

	fakeConsoleOutput(a)

	if err := a.waitForEcho(t.Context(), "   "); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForEchoHonorsContext(t *testing.T) {
	a := NewAgent(slog.LevelError)

	fakeConsoleOutput(a, "nothing echoed")

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	if err := a.waitForEcho(ctx, "write memory"); err == nil {
		t.Fatal("expected the missing echo to fail the wait")
	}
}

func TestWaitForEchoIgnoresLineWrapping(t *testing.T) {
	a := NewAgent(slog.LevelError)

	// the CLI wraps the echoed line at 80 columns, inserting whitespace
	fakeConsoleOutput(
		a,
		"n9k1(config)# username clab password 5 $5$KIMDOK$lHm/.buQQV4wh.phni5K730KPrcImsJ",
		" \bGfB9zIaaUvI/  role network-admin\r\nn9k1(config)# ",
	)

	err := a.waitForEcho(
		t.Context(),
		"username clab password 5 $5$KIMDOK$lHm/.buQQV4wh.phni5K730KPrcImsJGfB9zIaaUvI/"+
			"  role network-admin",
	)
	if err != nil {
		t.Fatal(err)
	}

	// the prompt after the echo stays available
	if err := a.processStepReadUntil(t.Context(), readUntilStep("n9k1(config)#")); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForEchoIgnoresOutputFromPreviousSteps(t *testing.T) {
	a := NewAgent(slog.LevelError)

	// output read before the write, carried over from a previous step, can never be its echo
	a.pendingConsole = []byte("exit\r\n")

	fakeConsoleOutput(a)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	if err := a.waitForEcho(ctx, "exit"); err == nil {
		t.Fatal("output from previous steps must not satisfy the echo wait")
	}
}

func TestWaitForEchoConsumesTheEcho(t *testing.T) {
	a := NewAgent(slog.LevelError)

	fakeConsoleOutput(a, "exit\r\nswitch# ")

	if err := a.waitForEcho(t.Context(), "exit"); err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(a.pendingConsole, []byte("exit")) {
		t.Fatal("the echo must be consumed so repeated identical lines wait for their own echo")
	}

	if err := a.processStepReadUntil(t.Context(), readUntilStep("switch#")); err != nil {
		t.Fatalf("output past the echo was lost: %v", err)
	}
}
