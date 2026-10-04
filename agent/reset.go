package agent

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenerrors "github.com/carlmontanari/boxen/errors"
)

const (
	monitorPrompt  = "(qemu)"
	monitorTimeout = 30 * time.Second
)

// monitorAddress is the QEMU monitor listener of the VM.
var monitorAddress = net.JoinHostPort("127.0.0.1", fmt.Sprint(boxenconstants.MonitorPort))

// Reset hard resets the VM through the QEMU monitor, like pressing the reset button: the guest
// reboots from its disk, so only saved guest configuration survives. Provisioning does not run
// again.
func Reset(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, monitorTimeout)
	defer cancel()

	var dialer net.Dialer

	conn, err := dialer.DialContext(ctx, "tcp", monitorAddress)
	if err != nil {
		return fmt.Errorf(
			"%w: failed connecting to the qemu monitor: %w",
			boxenerrors.ErrBoxen,
			err,
		)
	}

	defer func() {
		_ = conn.Close()
	}()

	deadline, _ := ctx.Deadline()

	err = conn.SetDeadline(deadline)
	if err != nil {
		return err
	}

	r := bufio.NewReader(conn)

	err = readMonitorPrompt(r)
	if err != nil {
		return err
	}

	_, err = conn.Write([]byte("system_reset\n"))
	if err != nil {
		return err
	}

	return readMonitorPrompt(r)
}

// readMonitorPrompt reads monitor output until the next prompt.
func readMonitorPrompt(r *bufio.Reader) error {
	var out strings.Builder

	for {
		b, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf(
				"%w: failed reading qemu monitor prompt: %w, output: %q",
				boxenerrors.ErrBoxen,
				err,
				out.String(),
			)
		}

		out.WriteByte(b)

		if strings.HasSuffix(out.String(), monitorPrompt) {
			return nil
		}
	}
}
