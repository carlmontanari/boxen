package cli

import (
	"github.com/carlmontanari/boxen/boxen/boxen"

	"github.com/urfave/cli/v2"
)

func vrnetlabFlags() ( //nolint: gocritic
	*cli.StringFlag, *cli.BoolFlag, *cli.StringFlag, *cli.StringFlag,
) {
	vrConnectionMode := &cli.StringFlag{
		Name:     "connection-mode",
		Usage:    "ignored",
		Required: false,
	}

	vrTrace := &cli.BoolFlag{
		Name:     "trace",
		Usage:    "ignored",
		Required: false,
	}

	vrVCPU := &cli.StringFlag{
		Name:     "vcpu",
		Usage:    "ignored",
		Required: false,
	}

	vrRAM := &cli.StringFlag{
		Name:     "ram",
		Usage:    "ignored",
		Required: false,
	}

	return vrConnectionMode, vrTrace, vrVCPU, vrRAM
}

func packageStartCommands() []*cli.Command {
	// vrnetlab compatibility flags are ignored, but exist so containerlab doesn't require any
	// changes to work with boxen!
	vrConnectionMode, vrTrace, vrVCPU, vrRAM := vrnetlabFlags()

	username, password, hostname := customizationFlags()

	startupConfig := &cli.StringFlag{
		Name:     "startup-config",
		Usage:    "path to startup-config file if desired",
		Required: false,
	}

	return []*cli.Command{{
		Name:   "package-start",
		Usage:  "start a packaged instance",
		Hidden: true,
		Flags: []cli.Flag{
			username,
			password,
			hostname,
			vrConnectionMode,
			vrTrace,
			vrVCPU,
			vrRAM,
			startupConfig,
		},
		Action: func(c *cli.Context) error {
			return packageStart(
				c.String("username"),
				c.String("password"),
				c.String("hostname"),
				c.String("startup-config"),
			)
		},
	}}
}

func packageStart(username, password, hostname, config string) error {
	l, li, err := spinLogger()
	if err != nil {
		return err
	}

	b, err := boxen.NewBoxen(boxen.WithLogger(li), boxen.WithConfig("boxen.yaml"))
	if err != nil {
		return err
	}

	return spin(l, li, func() error { return b.PackageStart(username, password, hostname, config) })
}
