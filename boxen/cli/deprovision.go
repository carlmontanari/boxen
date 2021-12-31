package cli

import (
	"github.com/carlmontanari/boxen/boxen/boxen"

	"github.com/urfave/cli/v2"
)

func deProvisionCommands() []*cli.Command {
	config := boxenGlobalFlags()

	instances := &cli.StringFlag{
		Name:     "instances",
		Usage:    "instance or comma sep string of instances to provision",
		Required: false,
	}

	return []*cli.Command{
		{
			Name:  "deprovision",
			Usage: "deprovision a local boxen instance",
			Flags: []cli.Flag{
				config,
				instances,
			},
			Action: func(c *cli.Context) error {
				return DeProvision(
					c.String("config"),
					c.String("instances"),
				)
			},
		},
	}
}

func DeProvision(config, instances string) error {
	l, li, err := spinLogger()
	if err != nil {
		return err
	}

	b, err := boxen.NewBoxen(boxen.WithLogger(li), boxen.WithConfig(config))
	if err != nil {
		return err
	}

	return spin(
		l,
		li,
		func() error { return instanceOp(b.DeProvision, instances) },
	)
}
