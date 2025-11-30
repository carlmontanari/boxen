package boxen

import (
	"context"
	"fmt"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxencontainertypes "github.com/carlmontanari/boxen/container/types"
	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenutil "github.com/carlmontanari/boxen/util"
)

// Build runs the build process -- this is the build process from the users perspective -- i.e. on
// their laptop.
func (b *Boxen) Build(
	ctx context.Context,
	imageRegistry,
	imageTag,
	diskImage,
	profile string,
) error {
	b.l.Info("boxen build starting...")

	b.l.Debug(
		"starting with args",
		"registry",
		imageRegistry,
		"tag",
		imageTag,
		"disk",
		diskImage,
		"profile",
		profile,
	)

	b.disk = diskImage

	var err error

	b.p, err = b.resolveProfile(profile)
	if err != nil {
		b.l.Error("failed resolving profile", "error", err.Error())

		return err
	}

	lis, err := b.getListener(ctx)
	if err != nil {
		b.l.Error("failed creating listener", "error", err.Error())

		return err
	}

	b.l.Debug(
		"starting server listener",
		"listen",
		lis.Addr().String(),
	)

	errs := make(chan error, 1)

	go func() {
		err = b.s.Serve(lis)
		if err != nil {
			errs <- err
		}
	}()

	ourAddr, err := b.getAddr(ctx)
	if err != nil {
		b.l.Error("failed gleaning usable address to pass to builder", "error", err.Error())

		return err
	}

	err = b.c.Run(
		ctx,
		b.l,
		boxencontainertypes.RunConfig{
			Name:  fmt.Sprintf("boxen-%s-builder", b.p.Name),
			Image: getBuilderImage(),
			Env: []string{
				fmt.Sprintf("%s=%s", boxenconstants.EnvServerHost, ourAddr),
			},
			Detached:   true,
			Remove:     false,
			Privileged: true,
		},
	)
	if err != nil {
		b.l.Error("failed running builder image", "error", err.Error())

		return err
	}

	b.l.Debug("build container launched")

	select {
	case err = <-errs:
		b.l.Error("received error from grpc server, exiting", "error", err.Error())

		return err
	case <-ctx.Done():
		b.l.Info("context cancelled, exiting")

		return ctx.Err()
	case <-b.agentDone:
		b.l.Info("done reported, exiting")

		return nil
	case <-b.agentExited:
		return fmt.Errorf("%w: agent exited, stopping boxen", boxenerrors.ErrBoxen)
	}
}

func getBuilderImage() string {
	defaultImage := fmt.Sprintf("ghcr.io/carlmontanari/boxen:%s", boxenconstants.Version)

	return boxenutil.GetEnvStrOrDefault(boxenconstants.EnvBuilderImage, defaultImage)
}
