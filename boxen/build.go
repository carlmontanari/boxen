package boxen

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxencontainertypes "github.com/carlmontanari/boxen/container/types"
	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenutil "github.com/carlmontanari/boxen/util"
)

const serialConsolePort = 5_001

// BuildOptions controls optional build behavior.
type BuildOptions struct {
	OnlyStartVM bool
}

// Build runs the build process -- this is the build process from the users perspective -- i.e. on
// their laptop.
func (b *Boxen) Build(
	ctx context.Context,
	imageRegistry,
	imageTag,
	diskImage,
	profile,
	platform string,
	options *BuildOptions,
) error {
	options = normalizeBuildOptions(options)

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
		"platform",
		platform,
		"onlyStartVM",
		options.OnlyStartVM,
	)

	if err := b.prepareBuild(diskImage, profile); err != nil {
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

	ourAddr, err := b.getAddr()
	if err != nil {
		b.l.Error("failed gleaning usable address to pass to builder", "error", err.Error())

		return err
	}

	containerID, err := b.c.Run(
		ctx,
		b.l,
		&boxencontainertypes.RunConfig{
			Name:  fmt.Sprintf("boxen-%s-builder", b.p.Name),
			Image: buildGetBuilderImage(),
			// Platform: platform,
			Env:      buildBuilderEnv(ourAddr, options),
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
		if options.OnlyStartVM {
			return b.attachStartedVM(ctx, containerID)
		}

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

	if imageTag == "latest" && b.p.ResolvedVersion != "" {
		imageTag = b.p.ResolvedVersion
	}

	fmt.Fprintf(&imageID, "boxen-%s:%s", b.p.Name, imageTag)

	err = b.c.Commit(
		ctx,
		b.l,
		containerID,
		imageID.String(),
		b.p.VirtualMachine.NatPorts,
	)
	if err != nil {
		return err
	}

	err = b.c.Rm(ctx, b.l, containerID)
	if err != nil {
		return err
	}

	return nil
}

func normalizeBuildOptions(options *BuildOptions) *BuildOptions {
	if options != nil {
		return options
	}

	return &BuildOptions{}
}

func (b *Boxen) prepareBuild(diskImage, profile string) error {
	b.disk = boxenutil.MustExpandPath(diskImage)

	diskInfo, err := os.Stat(b.disk)
	if err != nil {
		b.l.Error("failed resolving disk image", "disk", b.disk, "error", err.Error())

		return fmt.Errorf("%w: disk image %q not found: %w", boxenerrors.ErrBoxen, b.disk, err)
	}

	if diskInfo.IsDir() {
		b.l.Error("disk image path is a directory", "disk", b.disk)

		return fmt.Errorf("%w: disk image %q is a directory", boxenerrors.ErrBoxen, b.disk)
	}

	b.p, err = b.resolveProfile(profile)
	if err != nil {
		b.l.Error("failed resolving profile", "error", err.Error())

		return err
	}

	// We'll emit a warning log if we cant compile the pattern.
	_ = b.resolveVersion()

	return nil
}

func buildBuilderEnv(host string, options *BuildOptions) []string {
	options = normalizeBuildOptions(options)

	env := []string{
		fmt.Sprintf("%s=%s", boxenconstants.EnvServerHost, host),
	}

	if options.OnlyStartVM {
		env = append(env, fmt.Sprintf("%s=true", boxenconstants.EnvOnlyStartVM))
	}

	return env
}

func (b *Boxen) attachStartedVM(ctx context.Context, containerID string) error {
	command := buildConsoleAttachCommand(containerID)

	b.l.Info("vm started; attaching to console", "command", command)

	err := b.c.Exec(
		ctx,
		b.l,
		&boxencontainertypes.ExecConfig{
			ContainerID: containerID,
			Command: []string{
				"telnet",
				"localhost",
				fmt.Sprint(serialConsolePort),
			},
			Interactive: true,
			TTY:         true,
		},
	)
	if err != nil {
		b.l.Error("failed attaching to console", "command", command, "error", err.Error())
		b.l.Info("connect to the console manually", "command", command)

		return nil
	}

	b.l.Info("console detached; reconnect manually if needed", "command", command)

	return nil
}

func buildConsoleAttachCommand(containerID string) string {
	return fmt.Sprintf(
		"docker exec -i -t %s telnet localhost %d",
		containerID,
		serialConsolePort,
	)
}

func buildGetBuilderImage() string {
	defaultImage := fmt.Sprintf("ghcr.io/carlmontanari/boxen:%s", boxenconstants.Version)

	return boxenutil.GetEnvStrOrDefault(boxenconstants.EnvBuilderImage, defaultImage)
}
