package util

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

var onlyOneSignalHandler = make(chan struct{}) //nolint: gochecknoglobals

// SignalHandledContext returns a context that will be canceled if a SIGINT or SIGTERM is
// received.
func SignalHandledContext(
	logf func(format string, a ...any) (n int, err error),
) (context.Context, context.CancelFunc) {
	// panics when called twice, this way there can only be one signal handled context
	close(onlyOneSignalHandler)

	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 2) //nolint:mnd

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs

		_, _ = logf("received signal '%s', canceling context\n", sig)

		cancel()

		<-sigs

		_, _ = logf("received signal '%s', exiting program\n", sig)

		os.Exit(1)
	}()

	return ctx, cancel
}
