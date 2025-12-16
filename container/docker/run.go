package docker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	boxencontainertypes "github.com/carlmontanari/boxen/container/types"
)

// Run runs an image with the given config.
func (r *Runtime) Run(
	ctx context.Context,
	l *slog.Logger,
	cfg boxencontainertypes.RunConfig,
) (string, error) {
	tmpDir, err := os.MkdirTemp("", "boxen")
	if err != nil {
		return "", err
	}

	cidFileName := fmt.Sprintf("%s/boxenbuild", tmpDir)

	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	args := []string{
		"run",
		"--cidfile",
		cidFileName,
	}

	if cfg.Name != "" {
		args = append(args, "--name", cfg.Name)
	}

	if cfg.Platform != "" {
		args = append(args, "--platform", cfg.Platform)
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

	cmd := exec.CommandContext(ctx, docker, args...) //nolint: gosec

	b, err := cmd.CombinedOutput()
	if err != nil {
		l.Error("running container failed", "output", string(b), "error", err.Error())

		return "", err
	}

	l.Debug("running container succeeded", "output", string(b))

	cidFileContent, err := os.ReadFile(cidFileName) //nolint: gosec
	if err != nil {
		l.Error("failed reading cidfile", "error", err.Error())

		return "", err
	}

	return strings.TrimSpace(string(cidFileContent)), nil
}
