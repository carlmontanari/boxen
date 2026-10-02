package boxen

import (
	"errors"
	"io"
	"log/slog"
	"slices"

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

		args := []any{"component", "builder"}
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		slices.Sort(keys)

		for _, k := range keys {
			args = append(args, k, fields[k])
		}

		level := boxenlogging.LevelFromString(req.GetLevel())
		b.l.Log(stream.Context(), level, msg, args...)

		if level == slog.LevelError {
			b.agentExited <- struct{}{}
		}
	}
}
