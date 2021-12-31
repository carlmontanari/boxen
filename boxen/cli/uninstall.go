package cli

import (
	"strings"
	"sync"

	"github.com/carlmontanari/boxen/boxen/boxen"

	"github.com/urfave/cli/v2"
)

func unInstallCommands() []*cli.Command {
	config := boxenGlobalFlags()

	platform := &cli.StringFlag{
		Name:     "platform",
		Usage:    "boxen platform type, i.e. 'arista_veos' or 'cisco_csr1000v'",
		Required: true,
	}

	disk := &cli.StringFlag{
		Name:     "disks",
		Usage:    "comma sep list of disk versions to uninstall, i.e. '4.22.1F,4.22.2F'",
		Required: true,
	}

	return []*cli.Command{{
		Name:  "uninstall",
		Usage: "uninstall source disk(s) for local boxen instances",
		Flags: []cli.Flag{
			config,
			platform,
			disk,
		},
		Action: func(c *cli.Context) error {
			return UnInstall(
				c.String("config"),
				c.String("platform"),
				c.String("disks"),
			)
		},
	}}
}

func UnInstall(config, pT, disks string) error {
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
		li, func() error {
			wg := &sync.WaitGroup{}

			diskSlice := strings.Split(disks, ",")

			var errs []error

			for _, disk := range diskSlice {
				wg.Add(1)

				d := disk

				go func() {
					err = b.UnInstall(pT, d)

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
