package agent

import (
	"log/slog"
	"os"

	boxenlogging "github.com/carlmontanari/boxen/logging"
	boxenprofile "github.com/carlmontanari/boxen/profile"
	boxenprotov1 "github.com/carlmontanari/boxen/proto/v1"
	scrapligocli "github.com/scrapli/scrapligo/v2/cli"
	"google.golang.org/grpc"
)

const (
	profileFilename    = "profile.yaml"
	profilePermissions = 0o644
)

// Agent is process that runs the container half of things for boxen -- its the thing that the main
// boxen process talks with during the packaging process, and it also runs the vm inside the built
// image for working with containerlab in "normal" run operations.
type Agent struct {
	done chan struct{}

	l *wrappedSlogger
	c *grpc.ClientConn
	s boxenprotov1.BoxenServiceClient

	p *boxenprofile.Profile
	f *boxenprofile.Formatters

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
		p:    &boxenprofile.Profile{},
	}
}
