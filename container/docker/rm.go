package docker

import (
	"context"
	"log/slog"
	"os/exec"
)

// Rm removes a container.
func (r *Runtime) Rm(ctx context.Context, l *slog.Logger, containerID string) error {
	args := []string{
		"rm",
		containerID,
	}

	l.Info("removing container", "command", docker, "with args", args)

	cmd := exec.CommandContext(ctx, docker, args...) //nolint: gosec

	b, err := cmd.CombinedOutput()
	if err != nil {
		l.Error("removing container failed", "output", string(b), "error", err.Error())

		return err
	}

	return nil
}
