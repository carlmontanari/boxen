package agent

import (
	"context"
	"fmt"
	"log/slog"

	boxenprotov1 "github.com/carlmontanari/boxen/proto/v1"
	"google.golang.org/grpc"
)

type wrappedSlogger struct {
	l      *slog.Logger
	s      boxenprotov1.BoxenServiceClient
	stream grpc.ClientStreamingClient[boxenprotov1.LoggerRequest, boxenprotov1.LoggerResponse]
}

func adaptSlog(l *slog.Logger) *wrappedSlogger {
	return &wrappedSlogger{
		l: l,
	}
}

func (l *wrappedSlogger) Debug(msg string, args ...any) {
	l.tee("debug", msg, args...)
	l.l.Debug(msg, args...)
}

func (l *wrappedSlogger) Info(msg string, args ...any) {
	l.tee("info", msg, args...)
	l.l.Info(msg, args...)
}

func (l *wrappedSlogger) Warn(msg string, args ...any) {
	l.tee("warn", msg, args...)
	l.l.Warn(msg, args...)
}

func (l *wrappedSlogger) Error(msg string, args ...any) {
	l.tee("error", msg, args...)
	l.l.Error(msg, args...)
}

func (l *wrappedSlogger) setLogStream(
	ctx context.Context,
	s boxenprotov1.BoxenServiceClient,
) error {
	stream, err := s.Logger(ctx)
	if err != nil {
		return err
	}

	l.s = s
	l.stream = stream

	return nil
}

func (l *wrappedSlogger) tee(level, msg string, args ...any) {
	if l.s == nil {
		return
	}

	// convert all fields to string for shipping over grpc
	fields := sliceToMap(args)

	err := l.stream.Send(
		&boxenprotov1.LoggerRequest{
			Level:   level,
			Message: msg,
			Fields:  fields,
		},
	)
	if err != nil {
		l.l.Error("error streaming logs to server, ignoring", "error", err.Error())
	}
}

func sliceToMap(args []any) map[string]string {
	m := make(map[string]string, len(args)/2) //nolint:mnd

	for i := 0; i < len(args)-1; i += 2 {
		key, ok := args[i].(string)

		if !ok {
			key = fmt.Sprintf("%v", args[i])
		}

		m[key] = anyToString(args[i+1])
	}

	return m
}

func anyToString(v any) string {
	if v == nil {
		return ""
	}

	return fmt.Sprintf("%v", v)
}
