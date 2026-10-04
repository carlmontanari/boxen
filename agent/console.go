package agent

import (
	"bytes"
	"context"
	"fmt"
	"time"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenutil "github.com/carlmontanari/boxen/util"
	scrapligocli "github.com/scrapli/scrapligo/v2/cli"
	scrapligologging "github.com/scrapli/scrapligo/v2/logging"
	scrapligooptions "github.com/scrapli/scrapligo/v2/options"
)

const (
	echoTimeout      = 2 * time.Minute
	echoPollInterval = 20 * time.Millisecond
)

const (
	consoleHost           = "127.0.0.1"
	consoleOpenAttempts   = 5
	consoleOpenRetryDelay = 3 * time.Second
)

func (a *Agent) openConsoleConn(ctx context.Context, logFilename string) error {
	a.l.Info("opening console connection...")

	returnChar := "\r\n"
	if a.p.ScrapliReturnChar != "" {
		returnChar = a.p.ScrapliReturnChar
	}

	success := make(chan struct{}, 1)
	errs := make(chan error, 1)

	go func() {
		for attempt := 1; attempt <= consoleOpenAttempts; attempt++ {
			conn, err := scrapligocli.NewCli(
				consoleHost,
				scrapligooptions.WithDefinitionFileOrName(".scrapligo_definition.yaml"),
				scrapligooptions.WithPort(boxenconstants.ConsolePort),
				scrapligooptions.WithLogger(a.l.l),
				scrapligooptions.WithLoggerLevel(
					scrapligologging.LogLevel(
						boxenutil.GetEnvStrOrDefault(
							boxenconstants.EnvScrapliLogLevel,
							string(scrapligologging.Warn),
						),
					),
				),
				scrapligooptions.WithTransportTelnet(),
				scrapligooptions.WithReturnChar(returnChar),
				scrapligooptions.WithBypassInSessionAuth(),
				scrapligooptions.WithSessionRecorderPath(logFilename),
			)
			if err != nil {
				a.l.Error("failed creating console connection", "error", err.Error())
				errs <- err

				return
			}

			_, err = conn.Open(ctx)
			if err == nil {
				a.conn = conn
				a.readConsoleChunk = func() ([]byte, error) { return conn.Read() }
				success <- struct{}{}

				return
			}

			if attempt == consoleOpenAttempts {
				errs <- err

				return
			}

			a.l.Warn(
				"failed opening console connection, retrying",
				"attempt",
				attempt,
				"attempts",
				consoleOpenAttempts,
				"retryDelay",
				consoleOpenRetryDelay,
				"error",
				err.Error(),
			)

			timer := time.NewTimer(consoleOpenRetryDelay)

			select {
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}

				errs <- ctx.Err()

				return
			case <-timer.C:
			}
		}
	}()

	select {
	case <-success:
		a.l.Info("console connection opened")

		return nil
	case err := <-errs:
		a.l.Error("failed opening console connection", "error", err.Error())

		return err
	case <-ctx.Done():
		// opening a telnet session can block until the guest prints something
		a.l.Error("failed opening console connection", "error", ctx.Err().Error())

		return ctx.Err()
	}
}

func (a *Agent) closeConsoleConn(ctx context.Context) error {
	a.l.Info("closing console connection...")

	_, err := a.conn.Close(ctx)

	return err
}

// maxDrainReads bounds a single drain of the console session, so that a guest flooding the console
// cannot keep a reader from checking its conditions.
const maxDrainReads = 1024

const (
	readAttempts   = 5
	readRetryDelay = 200 * time.Millisecond
)

// drainReads calls read until it returns no data, at most maxDrainReads times, and returns
// everything read: a single console read returns at most one small chunk. A failed read is retried
// a few times, since reading right after the session opened can fail transiently.
func drainReads(read func() ([]byte, error)) ([]byte, error) {
	var out []byte

	for range maxDrainReads {
		b, err := readWithRetry(read)
		if err != nil {
			return out, err
		}

		if len(b) == 0 {
			break
		}

		out = append(out, b...)
	}

	return out, nil
}

func readWithRetry(read func() ([]byte, error)) ([]byte, error) {
	var err error

	for attempt := range readAttempts {
		if attempt > 0 {
			time.Sleep(readRetryDelay)
		}

		var b []byte

		b, err = read()
		if err == nil {
			return b, nil
		}
	}

	return nil, err
}

// readConsole returns output put back by a previous step followed by everything the console
// session has buffered so far, without blocking.
func (a *Agent) readConsole() ([]byte, error) {
	pending := a.pendingConsole
	a.pendingConsole = nil

	b, err := drainReads(a.readConsoleChunk)

	return append(pending, b...), err
}

// unreadConsole puts output read past a step's match back, so the following steps see it.
func (a *Agent) unreadConsole(b []byte) {
	if len(b) == 0 {
		return
	}

	a.pendingConsole = append(append([]byte(nil), b...), a.pendingConsole...)
}

// waitForEcho waits until the console echoed written text, so that following input is not sent
// before the guest processed it. Whitespace is ignored, since CLIs do not always echo it verbatim
// and wrap long lines, and an echo that never arrives fails the step instead of blocking it.
func (a *Agent) waitForEcho(ctx context.Context, s string) error {
	want := withoutWhitespace([]byte(s))
	if len(want) == 0 {
		return nil
	}

	echoCtx, cancel := context.WithTimeout(ctx, echoTimeout)
	defer cancel()

	var buf bytes.Buffer

	for {
		// this read cant block because its only reading off the internally buffered
		// bits that the session has already read
		b, err := a.readConsole()
		if err != nil {
			return err
		}

		if len(b) > 0 {
			a.l.Debug("console output", "content", string(b))
		}

		buf.Write(b)

		contents := withoutWhitespace(buf.Bytes())

		if bytes.Contains(contents, want) {
			// put everything read back, so the following steps see output past the echo
			a.unreadConsole(buf.Bytes())

			return nil
		}

		select {
		case <-echoCtx.Done():
			if ctx.Err() != nil {
				return ctx.Err()
			}

			return fmt.Errorf("%w: console did not echo %q", boxenerrors.ErrBoxen, s)
		case <-time.After(echoPollInterval):
		}
	}
}

// withoutWhitespace returns b without whitespace, and without the NUL, bell, and backspace bytes
// that line editors emit, for example when wrapping a line.
func withoutWhitespace(b []byte) []byte {
	out := make([]byte, 0, len(b))

	for _, c := range b {
		switch c {
		case ' ', '\t', '\r', '\n', 0, '\a', '\b':
			continue
		}

		out = append(out, c)
	}

	return out
}
