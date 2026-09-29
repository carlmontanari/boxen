package boxen

import (
	"log/slog"

	boxencontainer "github.com/carlmontanari/boxen/container"
	boxenlogging "github.com/carlmontanari/boxen/logging"
	boxenprofile "github.com/carlmontanari/boxen/profile"
	boxenprotov1 "github.com/carlmontanari/boxen/proto/v1"
	"google.golang.org/grpc"
)

// Boxen is the primary actor in the boxen process -- it is the thing that the cli kicks off for
// both packaging from a user perspective. It communicates with the Agent inside the build container
// via a bidir grpc stream.
type Boxen struct {
	boxenprotov1.UnimplementedBoxenServiceServer

	agentDone   chan struct{}
	agentExited chan struct{}

	l *slog.Logger
	s *grpc.Server

	c boxencontainer.Runtime

	p    *boxenprofile.Profile
	disk string
}

// NewBoxen returns a new boxen instance.
func NewBoxen(
	logLevel slog.Level,
	runtime boxencontainer.RuntimeKind,
) (*Boxen, error) {
	l := boxenlogging.NewLogger(logLevel)

	c, err := boxencontainer.NewRuntime(runtime)
	if err != nil {
		l.Error("failed creating container runtime", "error", err.Error())

		return nil, err
	}

	b := &Boxen{
		agentDone:   make(chan struct{}),
		agentExited: make(chan struct{}, 1),
		c:           c,
		l:           l,
		s:           grpc.NewServer(),
	}

	boxenprotov1.RegisterBoxenServiceServer(b.s, b)

	return b, nil
}
