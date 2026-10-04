package main

import (
	"context"
	"fmt"
	"os"
	"time"

	boxenagent "github.com/carlmontanari/boxen/agent"
	boxen "github.com/carlmontanari/boxen/boxen"
	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxencontainer "github.com/carlmontanari/boxen/container"
	boxenlogging "github.com/carlmontanari/boxen/logging"
	boxenutil "github.com/carlmontanari/boxen/util"
	urfavecli "github.com/urfave/cli/v3"
)

const (
	logLevelUsage = "log level, one of: debug, info, warn, error"
	saveTimeout   = 5 * time.Minute
)

func main() {
	ctx, cancel := boxenutil.SignalHandledContext(fmt.Printf) //nolint: forbidigo

	cmd := &urfavecli.Command{
		Name:    "boxen",
		Version: boxenconstants.Version,
		Usage:   "run boxen",
		Commands: []*urfavecli.Command{
			buildCommand(),
			packageCommand(),
			runCommand(),
			healthCommand(),
			saveCommand(),
		},
	}

	err := cmd.Run(ctx, os.Args)

	cancel()

	if err != nil {
		os.Exit(1)
	}
}

func buildCommand() *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "build",
		Usage: "build an instance with boxen, for use with containerlab",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{
				Name:     boxenconstants.FlagLogLevel,
				Usage:    logLevelUsage,
				Required: false,
				Value:    boxenconstants.DefaultLogLevel,
				Sources:  urfavecli.EnvVars(boxenconstants.EnvLoggingLevel),
			},
			&urfavecli.StringFlag{
				Name:     boxenconstants.FlagRuntime,
				Usage:    "container runtime, for now only docker is supported",
				Required: false,
				Value:    "docker",
				Sources:  urfavecli.EnvVars(boxenconstants.EnvRuntime),
			},
			&urfavecli.StringFlag{
				Name: boxenconstants.FlagImageRegistry,
				Aliases: []string{
					boxenconstants.FlagImageRegistryShort,
				},
				Usage:    "the registry to set in the image reference",
				Required: false,
				Sources:  urfavecli.EnvVars(boxenconstants.EnvImageRegistry),
			},
			&urfavecli.StringFlag{
				Name: boxenconstants.FlagImageTag,
				Aliases: []string{
					boxenconstants.FlagImageTagShort,
				},
				Usage: "the tag to set in the image reference, latest if unset *and* " +
					"no version is parsable from the disk image (based on versionPattern setting)",
				Required: false,
				Value:    "latest",
				Sources:  urfavecli.EnvVars(boxenconstants.EnvImageTag),
			},
			&urfavecli.StringFlag{
				Name: boxenconstants.FlagDiskImage,
				Aliases: []string{
					boxenconstants.FlagDiskImageShort,
				},
				Usage:    "the disk to target for packaging",
				Required: true,
			},
			&urfavecli.StringFlag{
				Name: boxenconstants.FlagProfileNameOrPath,
				Aliases: []string{
					boxenconstants.FlagProfileNameOrPathShort,
				},
				Usage: "the name or path to yaml definition of profile to use for packaging" +
					" if unset, will attempt to auto select based on provided image",
				Required: false,
			},
			&urfavecli.StringFlag{
				Name:     boxenconstants.FlagTargetPlatform,
				Usage:    "the docker platform to target, defaulting to x86 linux",
				Required: false,
				Value:    boxenconstants.DockerLinuxX86Platform,
				Sources:  urfavecli.EnvVars(boxenconstants.EnvTargetPlatform),
			},
			&urfavecli.BoolFlag{
				Name:  boxenconstants.FlagVMConsole,
				Usage: "start the VM and attach to its console without completing the image build",
			},
		},
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			b, err := boxen.NewBoxen(
				boxenlogging.LevelFromString(cmd.String(boxenconstants.FlagLogLevel)),
				boxencontainer.RuntimeKind(cmd.String(boxenconstants.FlagRuntime)),
			)
			if err != nil {
				return err
			}

			return b.Build(
				ctx,
				cmd.String(boxenconstants.FlagImageRegistry),
				cmd.String(boxenconstants.FlagImageTag),
				cmd.String(boxenconstants.FlagDiskImage),
				cmd.String(boxenconstants.FlagProfileNameOrPath),
				cmd.String(boxenconstants.FlagTargetPlatform),
				cmd.Bool(boxenconstants.FlagVMConsole),
			)
		},
	}
}

func packageCommand() *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "package",
		Usage: "run the container-side of the packaging process",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{
				Name:     boxenconstants.FlagLogLevel,
				Usage:    logLevelUsage,
				Required: false,
				Value:    boxenconstants.DefaultLogLevel,
				Sources:  urfavecli.EnvVars(boxenconstants.EnvLoggingLevel),
			},
			&urfavecli.StringFlag{
				Name:     boxenconstants.FlagServerHost,
				Usage:    "the boxen server host to communicate with",
				Required: true,
				Sources:  urfavecli.EnvVars(boxenconstants.EnvServerHost),
			},
		},
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			a := boxenagent.NewAgent(
				boxenlogging.LevelFromString(cmd.String(boxenconstants.FlagLogLevel)),
			)

			return a.Package(
				ctx,
				cmd.String(boxenconstants.FlagServerHost),
			)
		},
	}
}

func runCommand() *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "run",
		Usage: "run the packaged image under containerlab",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{
				Name:     boxenconstants.FlagLogLevel,
				Usage:    logLevelUsage,
				Required: false,
				Value:    boxenconstants.DefaultLogLevel,
				Sources:  urfavecli.EnvVars(boxenconstants.EnvLoggingLevel),
			},
			&urfavecli.StringFlag{
				Name:  boxenconstants.FlagContainerlabUsername,
				Usage: "the username to configure on the host",
			},
			&urfavecli.StringFlag{
				Name:  boxenconstants.FlagContainerlabPassword,
				Usage: "the password to configure on the host",
			},
			&urfavecli.StringFlag{
				Name:  boxenconstants.FlagContainerlabHostname,
				Usage: "the hostname to configure on the host",
			},
			&urfavecli.StringFlag{
				Name:  boxenconstants.FlagContainerlabConnectionMode,
				Usage: "the interface connection mode",
			},
			&urfavecli.BoolFlag{
				Name:  boxenconstants.FlagContainerlabTrace,
				Usage: "trace flag is ignored, but exists for containerlab compatibility",
			},
			&urfavecli.StringFlag{
				Name: boxenconstants.FlagContainerlabVCPU,
				Usage: "vm cpu count passed by containerlab 0.78 and earlier for some kinds; " +
					"used as QEMU_SMP when that is unset",
			},
			&urfavecli.StringFlag{
				Name: boxenconstants.FlagContainerlabRAM,
				Usage: "vm memory in MiB passed by containerlab 0.78 and earlier for some kinds; " +
					"used as QEMU_MEMORY when that is unset",
			},
		},
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			a := boxenagent.NewAgent(
				boxenlogging.LevelFromString(cmd.String(boxenconstants.FlagLogLevel)),
			)

			err := applyLegacyResourceFlags(
				cmd.String(boxenconstants.FlagContainerlabVCPU),
				cmd.String(boxenconstants.FlagContainerlabRAM),
			)
			if err != nil {
				return err
			}

			return a.Run(
				ctx,
				cmd.String(boxenconstants.FlagContainerlabUsername),
				cmd.String(boxenconstants.FlagContainerlabPassword),
				cmd.String(boxenconstants.FlagContainerlabHostname),
				cmd.String(boxenconstants.FlagContainerlabConnectionMode),
			)
		},
	}
}

// applyLegacyResourceFlags maps the --vcpu and --ram run flags, which older containerlab releases
// pass instead of setting QEMU_SMP and QEMU_MEMORY, onto those variables when they are unset.
func applyLegacyResourceFlags(vcpu, ram string) error {
	for env, value := range map[string]string{
		boxenconstants.EnvClabQemuSMP:    vcpu,
		boxenconstants.EnvClabQemuMemory: ram,
	} {
		if value == "" || os.Getenv(env) != "" {
			continue
		}

		err := os.Setenv(env, value)
		if err != nil {
			return err
		}
	}

	return nil
}

func healthCommand() *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "health",
		Usage: "container healthcheck; exits 0 when the node reports running",
		Action: func(_ context.Context, _ *urfavecli.Command) error {
			return boxenagent.Health()
		},
	}
}

func saveCommand() *urfavecli.Command {
	return &urfavecli.Command{
		Name: "save",
		Usage: "save the running configuration of a running node to its startup config file; " +
			"run inside the node container, e.g. with docker exec",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{
				Name:     boxenconstants.FlagLogLevel,
				Usage:    logLevelUsage,
				Required: false,
				Value:    "info",
				Sources:  urfavecli.EnvVars(boxenconstants.EnvLoggingLevel),
			},
			&urfavecli.StringFlag{
				Name:    boxenconstants.FlagContainerlabUsername,
				Usage:   "the node username, available to save process templates",
				Sources: urfavecli.EnvVars(boxenconstants.EnvClabUsername),
			},
			&urfavecli.StringFlag{
				Name:    boxenconstants.FlagContainerlabPassword,
				Usage:   "the node password, available to save process templates",
				Sources: urfavecli.EnvVars(boxenconstants.EnvClabPassword),
			},
			&urfavecli.StringFlag{
				Name: boxenconstants.FlagContainerlabHostname,
				Usage: "the node hostname, available to save process templates; defaults to the " +
					"container hostname",
			},
			&urfavecli.DurationFlag{
				Name:  boxenconstants.FlagTimeout,
				Usage: "the maximum duration of the save",
				Value: saveTimeout,
			},
		},
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			a := boxenagent.NewAgent(
				boxenlogging.LevelFromString(cmd.String(boxenconstants.FlagLogLevel)),
			)

			hostname := cmd.String(boxenconstants.FlagContainerlabHostname)
			if hostname == "" {
				hostname, _ = os.Hostname()
			}

			ctx, cancel := context.WithTimeout(ctx, cmd.Duration(boxenconstants.FlagTimeout))
			defer cancel()

			_, err := a.Save(
				ctx,
				cmd.String(boxenconstants.FlagContainerlabUsername),
				cmd.String(boxenconstants.FlagContainerlabPassword),
				hostname,
			)

			return err
		},
	}
}
