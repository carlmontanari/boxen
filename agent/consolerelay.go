package agent

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenerrors "github.com/carlmontanari/boxen/errors"
)

const consoleWakeDelay = 500 * time.Millisecond

// consoleAddress is the VM serial console listener.
var consoleAddress = net.JoinHostPort("127.0.0.1", strconv.Itoa(boxenconstants.ConsolePort))

// consoleRelay forwards one console session between a local listener and the VM console. QEMU
// serves a single console client at a time, and a console client that fails to open can leave its
// connection behind; relaying lets the agent always drop its connection to the VM console.
type consoleRelay struct {
	port uint16

	mu     sync.Mutex
	closed bool
	closer []io.Closer
}

// startConsoleRelay starts relaying the first client of the returned relay's port to the console
// at target. With wake set, the relay sends a return to the guest once connected: opening a
// console session only completes once the guest printed something, and an idle guest prints
// nothing until it receives input. Telnet negotiation passes through unchanged.
func startConsoleRelay(ctx context.Context, target string, wake bool) (*consoleRelay, error) {
	var lc net.ListenConfig

	lis, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	addr, ok := lis.Addr().(*net.TCPAddr)
	if !ok {
		_ = lis.Close()

		return nil, fmt.Errorf("%w: unexpected relay address %s", boxenerrors.ErrBoxen, lis.Addr())
	}

	r := &consoleRelay{
		port:   uint16(addr.Port), //nolint: gosec // tcp ports fit
		closer: []io.Closer{lis},
	}

	go r.serve(ctx, lis, target, wake)

	return r, nil
}

// Close ends the relayed session, including the connection to the VM console.
func (r *consoleRelay) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	r.closed = true

	for _, c := range r.closer {
		_ = c.Close()
	}
}

func (r *consoleRelay) serve(ctx context.Context, lis net.Listener, target string, wake bool) {
	client, err := lis.Accept()

	_ = lis.Close()

	if err != nil || !r.track(client) {
		return
	}

	var dialer net.Dialer

	console, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil || !r.track(console) {
		r.Close()

		return
	}

	go func() {
		_, _ = io.Copy(console, client)

		r.Close()
	}()

	if wake {
		// give the telnet negotiation a head start before waking the guest
		timer := time.AfterFunc(consoleWakeDelay, func() {
			_, _ = console.Write([]byte("\r"))
		})
		defer timer.Stop()
	}

	_, _ = io.Copy(client, console)

	r.Close()
}

// track registers c to be closed with the relay, closing it right away when the relay is closed.
func (r *consoleRelay) track(c io.Closer) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		_ = c.Close()

		return false
	}

	r.closer = append(r.closer, c)

	return true
}
