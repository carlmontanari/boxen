package docker

import (
	"context"
	"log/slog"
	"os"
	"os/exec"

	boxencontainertypes "github.com/carlmontanari/boxen/container/types"
)

// Exec runs a command in a container.
func (r *Runtime) Exec(
	ctx context.Context,
	l *slog.Logger,
	cfg *boxencontainertypes.ExecConfig,
) error {
	args := []string{"exec"} //nolint: prealloc

	if cfg.Interactive {
		args = append(args, "-i")
	}

	if cfg.TTY {
		args = append(args, "-t")
	}

	args = append(args, cfg.ContainerID)
	args = append(args, cfg.Command...)

	l.Info("executing container command", "command", docker, "with args", args)

	cmd := exec.CommandContext(ctx, docker, args...) //nolint: gosec
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		l.Error("executing container command failed", "error", err.Error())

		return err
	}

	return nil
}
