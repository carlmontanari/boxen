package cli

import (
	"strings"
	"sync"

	"github.com/carlmontanari/boxen/boxen/boxen"

	"github.com/urfave/cli/v2"
)

func provisionCommands() []*cli.Command {
	config := boxenGlobalFlags()

	instances := &cli.StringFlag{
		Name:     "instances",
		Usage:    "instance or comma sep string of instances to provision",
		Required: false,
	}

	vendor := &cli.StringFlag{
		Name:     "vendor",
		Usage:    "name of the vendor (ex: 'arista') for the instance(s) to provision",
		Required: true,
	}

	platform := &cli.StringFlag{
		Name:     "platform",
		Usage:    "name of the platform (ex: 'veos') for the instance(s) to provision",
		Required: true,
	}

	source := &cli.StringFlag{
		Name:     "source-disk",
		Usage:    "installed source disk to use for provisioning the instance(s)",
		Required: false,
	}

	profile := &cli.StringFlag{
		Name:     "profile",
		Usage:    "hardware profile to apply to the instance(s)",
		Required: false,
	}

	return []*cli.Command{
		{
			Name:  "provision",
			Usage: "provision a local boxen instance",
			Flags: []cli.Flag{
				config,
				instances,
				vendor,
				platform,
				source,
				profile,
			},
			Action: func(c *cli.Context) error {
				return Provision(
					c.String("config"),
					c.String("instances"),
					c.String("vendor"),
					c.String("platform"),
					c.String("sourceDisk"),
					c.String("profile"),
				)
			},
		},
	}
}

func Provision(config, instances, vendor, platform, sourceDisk, profile string) error {
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
		func() error {
			wg := &sync.WaitGroup{}

			instanceSlice := strings.Split(instances, ",")

			var errs []error

			for _, instance := range instanceSlice {
				wg.Add(1)

				i := instance

				go func() {
					err = b.Provision(i, vendor, platform, sourceDisk, profile)

					if err != nil {
						errs = append(errs, err)
					}

					wg.Done()
				}()
			}

			wg.Wait()

			if len(errs) > 0 {
				return errs[0]
			}

			return nil
		},
	)
}
