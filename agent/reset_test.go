package agent

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

func TestReset(t *testing.T) {
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = lis.Close() })

	orig := monitorAddress
	monitorAddress = lis.Addr().String()

	t.Cleanup(func() { monitorAddress = orig })

	received := make(chan string, 1)

	go func() {
		conn, err := lis.Accept()
		if err != nil {
			received <- err.Error()

			return
		}

		defer func() { _ = conn.Close() }()

		_, _ = conn.Write(
			[]byte("QEMU 7.2.17 monitor - type 'help' for more information\r\n(qemu) "),
		)

		line, _ := bufio.NewReader(conn).ReadString('\n')

		_, _ = conn.Write([]byte("system_reset\r\n(qemu) "))

		received <- strings.TrimSpace(line)
	}()

	if err := Reset(t.Context()); err != nil {
		t.Fatal(err)
	}

	if got := <-received; got != "system_reset" {
		t.Fatalf("expected system_reset to be sent, got %q", got)
	}
}

func TestResetMonitorUnavailable(t *testing.T) {
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	addr := lis.Addr().String()
	_ = lis.Close()

	orig := monitorAddress
	monitorAddress = addr

	t.Cleanup(func() { monitorAddress = orig })

	if err := Reset(t.Context()); err == nil {
		t.Fatal("expected an error without a monitor")
	}
}
