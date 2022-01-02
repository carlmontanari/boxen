package cli

import (
	"github.com/carlmontanari/boxen/boxen/boxen"

	"github.com/urfave/cli/v2"
)

func installCommands() []*cli.Command {
	config := boxenGlobalFlags()

	disk := &cli.StringFlag{
		Name:     "disk",
		Usage:    "disk image to target, ex: 'vEOS-lab-4.22.1F.vmdk'",
		Required: true,
	}

	username, password, _ := customizationFlags()

	return []*cli.Command{{
		Name:  "install",
		Usage: "install a source disk for local boxen instances",
		Flags: []cli.Flag{
			config,
			disk,
			username,
			password,
		},
		Action: func(c *cli.Context) error {
			return Install(
				c.String("config"),
				c.String("disk"),
				c.String("username"),
				c.String("password"),
			)
		},
	}}
}

// Install is the cli entrypoint to install a disk as a local source disk.
func Install(config, disk, username, password string) error {
	l, li, err := spinLogger()
	if err != nil {
		return err
	}

	b, err := boxen.NewBoxen(boxen.WithLogger(li), boxen.WithConfig(config))
	if err != nil {
		return err
	}

	return spin(l, li, func() error { return b.Install(disk, username, password) })
}
