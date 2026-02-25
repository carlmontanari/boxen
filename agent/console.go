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

func (a *Agent) openConsoleConn(ctx context.Context, logFilename string) error {
	a.l.Info("opening console connection...")

	var err error

	a.conn, err = scrapligocli.NewCli(
		"localhost",
		scrapligooptions.WithDefintionFileOrName(".scrapligo_definition.yaml"),
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
		scrapligooptions.WithReturnChar("\r\n"),
		scrapligooptions.WithBypassInSessionAuth(),
		scrapligooptions.WithSessionRecorderPath(logFilename),
	)
	if err != nil {
		a.l.Error("failed creating console connection", "error", err.Error())

		return err
	}

	_, err = a.conn.Open(ctx)
	if err != nil {
		a.l.Error("failed opening console connection", "error", err.Error())

		return err
	}

	a.l.Info("console connection opened")

	return nil
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
