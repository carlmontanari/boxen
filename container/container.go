package container

import (
	"context"
	"fmt"
	"log/slog"

	boxencontainerdocker "github.com/carlmontanari/boxen/container/docker"
	boxencontainertypes "github.com/carlmontanari/boxen/container/types"
	boxenerrors "github.com/carlmontanari/boxen/errors"
)

// RuntimeKind is an enum-ish for the supported container runtimes, for now its just docker.
type RuntimeKind string

// enumerations of RuntimeKind.
const (
	RuntimeKindDocker RuntimeKind = "docker"
)

// Runtime is the interface a container runtime needs to satisfy to work with boxen.
type Runtime interface {
	Run(ctx context.Context, l *slog.Logger, cfg boxencontainertypes.RunConfig) (string, error)
	Commit(ctx context.Context, l *slog.Logger, containerID, imageID string) error
	Rm(ctx context.Context, l *slog.Logger, containerID string) error
}

// NewRuntime dispatches a runtime based on the provided kind.
func NewRuntime(kind RuntimeKind) (Runtime, error) { //nolint: ireturn
	switch kind {
	case RuntimeKindDocker:
		return boxencontainerdocker.NewRuntime(), nil
	default:
		return nil, fmt.Errorf("%w: unsupported runtime %q", boxenerrors.ErrBoxen, kind)
	}
}
