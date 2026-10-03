package agent

import (
	"bytes"
	"context"
	"time"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenutil "github.com/carlmontanari/boxen/util"
	scrapligocli "github.com/scrapli/scrapligo/v2/cli"
	scrapligologging "github.com/scrapli/scrapligo/v2/logging"
	scrapligooptions "github.com/scrapli/scrapligo/v2/options"
)

const (
	consoleHost           = "127.0.0.1"
	consoleOpenAttempts   = 5
	consoleOpenRetryDelay = 3 * time.Second
)

// openConsoleConn opens the console session used by profile steps. With wake set, the guest is sent
// a return so that an idle console prints its prompt; leave it unset while the guest boots, where a
// stray return could answer a boot dialog.
func (a *Agent) openConsoleConn(ctx context.Context, logFilename string, wake bool) error {
	a.l.Info("opening console connection...")

	returnChar := "\r\n"
	if a.p.ScrapliReturnChar != "" {
		returnChar = a.p.ScrapliReturnChar
	}

	success := make(chan struct{}, 1)
	errs := make(chan error, 1)

	go func() {
		for attempt := 1; attempt <= consoleOpenAttempts; attempt++ {
			relay, err := startConsoleRelay(ctx, consoleAddress, wake)
			if err != nil {
				errs <- err

				return
			}

			conn, err := scrapligocli.NewCli(
				consoleHost,
				scrapligooptions.WithDefinitionFileOrName(".scrapligo_definition.yaml"),
				scrapligooptions.WithPort(relay.port),
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
				relay.Close()
				a.l.Error("failed creating console connection", "error", err.Error())
				errs <- err

				return
			}

			_, err = conn.Open(ctx)
			if err == nil {
				a.conn = conn
				a.readConsoleChunk = func() ([]byte, error) { return conn.Read() }
				a.consoleRelay = relay
				success <- struct{}{}

				return
			}

			// a failed open can leave its connection behind, holding the single console session
			relay.Close()

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

	// always free the console session for other clients
	if a.consoleRelay != nil {
		a.consoleRelay.Close()
		a.consoleRelay = nil
	}

	return err
}

// maxDrainReads bounds a single drain of the console session, so that a guest flooding the console
// cannot keep a reader from checking its conditions.
const maxDrainReads = 1024

// drainReads calls read until it returns no data, at most maxDrainReads times, and returns
// everything read: a single console read returns at most one small chunk.
func drainReads(read func() ([]byte, error)) ([]byte, error) {
	var out []byte

	for range maxDrainReads {
		b, err := read()
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

func (a *Agent) readUntil(ctx context.Context, s string) error {
	var buf bytes.Buffer

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// this read cant block because its only reading off the internally buffered
		// bits that the session has already read
		b, err := a.readConsole()
		if err != nil {
			return err
		}

		_, err = buf.Write(b)
		if err != nil {
			return err
		}

		contents := bytes.ReplaceAll(buf.Bytes(), []byte{0}, nil)

		if len(b) > 0 {
			a.l.Debug("console output", "content", string(b))
		}

		if idx := bytes.Index(contents, []byte(s)); idx >= 0 {
			a.unreadConsole(contents[idx+len(s):])

			return nil
		}

		time.Sleep(time.Second)
	}
}
