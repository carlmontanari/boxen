package cli

import (
	"github.com/carlmontanari/boxen/boxen/boxen"

	"github.com/urfave/cli/v2"
)

func initCommands() []*cli.Command {
	directory := &cli.StringFlag{
		Name:     "directory",
		Usage:    "directory to initialize boxen in",
		Required: false,
		Value:    "~/boxen",
	}

	return []*cli.Command{
		{
			Name:  "init",
			Usage: "initialize a boxen config/directory structure",
			Flags: []cli.Flag{
				directory,
			},
			Action: func(c *cli.Context) error {
				return Init(
					c.String("directory"),
				)
			},
		},
	}
}

func Init(directory string) error {
	b, err := boxen.NewBoxen()
	if err != nil {
		return err
	}

	err = b.Init(directory)

	return err
}
