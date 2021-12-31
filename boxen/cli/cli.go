package cli

import (
	"fmt"

	"github.com/carlmontanari/boxen/boxen/boxen"

	"github.com/urfave/cli/v2"
)

func boxenGlobalFlags() *cli.StringFlag {
	config := &cli.StringFlag{
		Name:     "config",
		Usage:    "config file to use/create",
		Required: false,
		Value:    "~/boxen/boxen.yaml",
	}

	return config
}

func customizationFlags() (username, password, hostname *cli.StringFlag) {
	username = &cli.StringFlag{
		Name:     "username",
		Usage:    "username to set on the instance",
		Required: false,
	}

	password = &cli.StringFlag{
		Name:     "password",
		Usage:    "password to set on the instance",
		Required: false,
	}

	hostname = &cli.StringFlag{
		Name:     "hostname",
		Usage:    "hostname to set on the device",
		Required: false,
	}

	return username, password, hostname
}

func operationCommands() []*cli.Command {
	config := boxenGlobalFlags()

	instances := &cli.StringFlag{
		Name:     "instances",
		Usage:    "instance or comma sep string of instances to start/stop",
		Required: false,
	}
	group := &cli.StringFlag{
		Name:     "group",
		Usage:    "name of instance group to start/stop",
		Required: false,
	}

	return []*cli.Command{
		{ //nolint:dupl
			Name:  "start",
			Usage: "start boxen instance(s)/group(s)",
			Subcommands: []*cli.Command{
				{
					Name:  "instance",
					Usage: "start boxen instance(s)",
					Flags: []cli.Flag{
						config,
						instances,
					},
					Action: func(c *cli.Context) error {
						return Start(c.String("config"), c.String("instances"))
					},
				},
				{
					Name:  "group",
					Usage: "start boxen group",
					Flags: []cli.Flag{
						config,
						group,
					},
					Action: func(c *cli.Context) error {
						return StartGroup(c.String("config"), c.String("group"))
					},
				},
			},
		},
		{ //nolint:dupl
			Name:  "stop",
			Usage: "stop boxen instance(s)/group(s)",
			Subcommands: []*cli.Command{
				{
					Name:  "instance",
					Usage: "stop boxen instance(s)",
					Flags: []cli.Flag{
						config,
						instances,
					},
					Action: func(c *cli.Context) error {
						return Stop(c.String("config"), c.String("instances"))
					},
				},
				{
					Name:  "group",
					Usage: "stop boxen group",
					Flags: []cli.Flag{
						config,
						group,
					},
					Action: func(c *cli.Context) error {
						return StopGroup(c.String("config"), c.String("group"))
					},
				},
			},
		},
	}
}

func NewCLI() *cli.App {
	cli.VersionPrinter = showVersion

	var commands []*cli.Command

	commands = append(commands, initCommands()...)
	commands = append(commands, packageBuildCommands()...)
	commands = append(commands, packageInstallCommands()...)
	commands = append(commands, packageStartCommands()...)
	commands = append(commands, installCommands()...)
	commands = append(commands, unInstallCommands()...)
	commands = append(commands, provisionCommands()...)
	commands = append(commands, deProvisionCommands()...)
	commands = append(commands, operationCommands()...)

	app := &cli.App{
		Name:     "boxen",
		Version:  "dev",
		Usage:    "package or run network operating system vm instances",
		Commands: commands,
	}

	return app
}

func showVersion(c *cli.Context) {
	fmt.Printf("\tversion: %s\n", boxen.Version)
	fmt.Printf("\tsource: %s\n", "https://github.com/carlmontanari/boxen")
}
