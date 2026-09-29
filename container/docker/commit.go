package docker

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	boxenprofile "github.com/carlmontanari/boxen/profile"
)

// Commit commits any changes in the image to the image.
func (r *Runtime) Commit(
	ctx context.Context,
	l *slog.Logger,
	containerID,
	imageID string,
	natPorts []boxenprofile.NatPort,
) error {
	args := []string{ //nolint: prealloc
		"commit",
		"--change",
		`ENTRYPOINT ["/boxen/boxen", "run"]`,
	}

	for idx := range natPorts {
		args = append(
			args,
			"--change",
			fmt.Sprintf(
				`EXPOSE %d/%s`,
				natPorts[idx].LocalPort,
				strings.ToUpper(string(natPorts[idx].Type)),
			),
		)
	}

	args = append(
		args,
		containerID,
		imageID,
	)

	l.Info("committing container", "command", docker, "with args", args)

	cmd := exec.CommandContext(ctx, docker, args...)

	b, err := cmd.CombinedOutput()
	if err != nil {
		l.Error("committing container failed", "output", string(b), "error", err.Error())

		return err
	}

	return nil
}
