package boxen

import (
	"errors"
	"fmt"
	"io"
	"log/slog"

	boxenlogging "github.com/carlmontanari/boxen/logging"
	boxenprotov1 "github.com/carlmontanari/boxen/proto/v1"
	"google.golang.org/grpc"
)

// Logger is rpc endpoint for the builder to stream logs to.
func (b *Boxen) Logger(
	stream grpc.ClientStreamingServer[boxenprotov1.LoggerRequest, boxenprotov1.LoggerResponse],
) error {
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return stream.SendAndClose(&boxenprotov1.LoggerResponse{})
		}

		if err != nil {
			return err
		}

		msg := req.GetMessage()
		fields := req.GetFields()

		args := make([]any, 0, len(fields)*2) //nolint: mnd

		for k, v := range fields {
			args = append(args, k, v)
		}

		switch boxenlogging.LevelFromString(req.GetLevel()) {
		case slog.LevelDebug:
			b.l.Debug(fmt.Sprintf("builder message: %s", msg), args...)
		case slog.LevelInfo:
			b.l.Info(fmt.Sprintf("builder message: %s", msg), args...)
		case slog.LevelWarn:
			b.l.Warn(fmt.Sprintf("builder message: %s", msg), args...)
		case slog.LevelError:
			b.l.Error(fmt.Sprintf("builder message: %s", msg), args...)

			b.agentExited <- struct{}{}
		}
	}
}
