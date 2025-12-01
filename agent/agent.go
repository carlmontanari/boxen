package agent

import (
	"context"
	"log/slog"
	"os"

	boxenlogging "github.com/carlmontanari/boxen/logging"
	boxenprofile "github.com/carlmontanari/boxen/profile"
	boxenprotov1 "github.com/carlmontanari/boxen/proto/v1"
	scrapligocli "github.com/scrapli/scrapligo/cli"
	scrapligologging "github.com/scrapli/scrapligo/logging"
	scrapligooptions "github.com/scrapli/scrapligo/options"
	"google.golang.org/grpc"
)

// Agent is process that runs the container half of things for boxen -- its the thing that the main
// boxen process talks with during the packaging process, and it also runs the vm inside the built
// image for working with containerlab in "normal" run operations.
type Agent struct {
	done chan struct{}

	l *wrappedSlogger
	c *grpc.ClientConn
	s boxenprotov1.BoxenServiceClient

	p    *boxenprofile.Profile
	disk string

	stdoutF *os.File

	conn *scrapligocli.Cli
}

// NewAgent returns a new boxen Agent instance.
func NewAgent(
	logLevel slog.Level,
) *Agent {
	return &Agent{
		done: make(chan struct{}),
		l:    adaptSlog(boxenlogging.NewLogger(logLevel)),
	}
}

func (a *Agent) openConsoleConn(ctx context.Context) error {
	a.l.Info("opening console connection...")

	var err error

	a.conn, err = scrapligocli.NewCli(
		"localhost",
		scrapligooptions.WithDefintionFileOrName(".scrapligo_definition.yaml"),
		scrapligooptions.WithPort(5_001), //nolint: mnd
		scrapligooptions.WithLogger(a.l.l),
		scrapligooptions.WithLoggerLevel(scrapligologging.Debug),
		scrapligooptions.WithTransportTelnet(),
		scrapligooptions.WithReturnChar("\r\n"),
		scrapligooptions.WithBypassInSessionAuth(),
		scrapligooptions.WithSessionRecorderPath("console.log"),
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
