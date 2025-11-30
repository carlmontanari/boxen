package docker

import (
	"context"
	"log/slog"
	"os/exec"

	boxencontainertypes "github.com/carlmontanari/boxen/container/types"
)

// Run runs an image with the given config.
func (r *Runtime) Run(
	ctx context.Context,
	l *slog.Logger,
	cfg boxencontainertypes.RunConfig,
) error {
	args := []string{
		"run",
	}

	if cfg.Name != "" {
		args = append(args, "--name", cfg.Name)
	}

	for _, env := range cfg.Env {
		args = append(args, "-e", env)
	}

	if cfg.Detached {
		args = append(args, "-d")
	}

	if cfg.Remove {
		args = append(args, "--rm")
	}

	if cfg.Privileged {
		args = append(args, "--privileged")
	}

	args = append(args, cfg.Image)

	l.Info("running container", "command", docker, "with args", args)

	cmd := exec.CommandContext(ctx, docker, args...)

	b, err := cmd.CombinedOutput()
	if err != nil {
		l.Error("running container failed", "output", string(b), "error", err.Error())

		return err
	}

	l.Debug("running container succeeded", "output", string(b))

	return nil
}
