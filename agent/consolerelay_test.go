package agent

import (
	"errors"
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

// fakeConsole accepts one console connection and hands it to the test.
func fakeConsole(t *testing.T) (addr string, accepted <-chan net.Conn) {
	t.Helper()

	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = lis.Close() })

	conns := make(chan net.Conn, 1)

	go func() {
		conn, err := lis.Accept()
		if err != nil {
			return
		}

		t.Cleanup(func() { _ = conn.Close() })

		conns <- conn
	}()

	return lis.Addr().String(), conns
}

func dialRelay(t *testing.T, r *consoleRelay) net.Conn {
	t.Helper()

	client, err := (&net.Dialer{}).DialContext(
		t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(r.port))),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = client.Close() })

	_ = client.SetDeadline(time.Now().Add(5 * time.Second))

	return client
}

func readString(t *testing.T, c net.Conn) string {
	t.Helper()

	buf := make([]byte, 64)

	n, err := c.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	return string(buf[:n])
}

func TestConsoleRelayCloseFreesConsole(t *testing.T) {
	target, conns := fakeConsole(t)

	r, err := startConsoleRelay(t.Context(), target)
	if err != nil {
		t.Fatal(err)
	}

	client := dialRelay(t, r)
	console := <-conns

	_ = console.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := client.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}

	// nothing but client input reaches the console
	if got := readString(t, console); got != "x" {
		t.Fatalf("expected only client input, got %q", got)
	}

	r.Close()

	// the console connection is closed even though the client never closed its side
	if _, err := console.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("expected the console connection to be closed, got %v", err)
	}
}
