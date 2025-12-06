package boxen

import (
	"context"
	"fmt"
	"strings"
	"time"

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

	containerID, err := b.c.Run(
		ctx,
		b.l,
		boxencontainertypes.RunConfig{
			Name:  fmt.Sprintf("boxen-%s-builder", b.p.Name),
			Image: buildGetBuilderImage(),
			Env: []string{
				fmt.Sprintf("%s=%s", boxenconstants.EnvServerHost, ourAddr),
			},
			Detached: true,
			// dont remove! we'll be committing the image to our new final image
			// once the packaging process is complete
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
		b.l.Info("done reported, finalizing image")

		// tiny sleep to ensure container has exited
		time.Sleep(time.Second)
	case <-b.agentExited:
		return fmt.Errorf("%w: agent exited, stopping boxen", boxenerrors.ErrBoxen)
	}

	var imageID strings.Builder

	if imageRegistry != "" {
		imageID.WriteString(imageRegistry)
		imageID.WriteString("/")
	}

	imageID.WriteString(fmt.Sprintf("boxen-%s:%s", b.p.Name, imageTag))

	err = b.c.Commit(ctx, b.l, containerID, imageID.String())
	if err != nil {
		return err
	}

	err = b.c.Rm(ctx, b.l, containerID)
	if err != nil {
		return err
	}

	return nil
}

func buildGetBuilderImage() string {
	defaultImage := fmt.Sprintf("ghcr.io/carlmontanari/boxen:%s", boxenconstants.Version)

	return boxenutil.GetEnvStrOrDefault(boxenconstants.EnvBuilderImage, defaultImage)
}
