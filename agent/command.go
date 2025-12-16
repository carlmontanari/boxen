package agent

import (
	"context"
	"os/exec"
)

func (a *Agent) invokeCommand(ctx context.Context, command string) error {
	a.l.Info("invoking command...")

	cmd := exec.CommandContext(ctx, commandBinary, "-c", command)

	b, err := cmd.CombinedOutput()
	if err != nil {
		a.l.Error("invoking command failed", "error", err.Error())

		return err
	}

	a.l.Debug("command executed", "command", command, "output", string(b))

	return nil
}
