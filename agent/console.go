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
	consoleHost           = "localhost"
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
				scrapligooptions.WithPort(5_001), //nolint: mnd
				scrapligooptions.WithLogger(a.l.l),
				scrapligooptions.WithLoggerLevel(
					scrapligologging.LogLevel(
						boxenutil.GetEnvStrOrDefault(
							boxenconstants.EnvScrapliLogLevel,
							string(scrapligologging.Debug),
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
	}
}

func (a *Agent) closeConsoleConn(ctx context.Context) error {
	a.l.Info("closing console connection...")

	_, err := a.conn.Close(ctx)
	if err != nil {
		return err
	}

	return nil
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
		b, err := a.conn.Read()
		if err != nil {
			return err
		}

		_, err = buf.Write(b)
		if err != nil {
			return err
		}

		contents := bytes.ReplaceAll(buf.Bytes(), []byte{0}, nil)

		a.l.Debug("checking contents", "until", s, "contents", string(contents))

		if bytes.Contains(contents, []byte(s)) {
			return nil
		}

		time.Sleep(time.Second)
	}
}
