package docker

import (
	"context"
	"log/slog"
	"os/exec"
)

// Commit commits any changes in the image to the image.
func (r *Runtime) Commit(ctx context.Context, l *slog.Logger, containerID, imageID string) error {
	args := []string{
		"commit",
		"--change",
		`ENTRYPOINT ["/boxen/boxen", "run"]`,
		containerID,
		imageID,
	}

	l.Info("committing container", "command", docker, "with args", args)

	cmd := exec.CommandContext(ctx, docker, args...) //nolint: gosec

	b, err := cmd.CombinedOutput()
	if err != nil {
		l.Error("committing container failed", "output", string(b), "error", err.Error())

		return err
	}

	return nil
}
