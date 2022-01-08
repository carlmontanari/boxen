package cli

import (
	"github.com/carlmontanari/boxen/boxen/boxen"

	"github.com/urfave/cli/v2"
)

func packageBuildCommands() []*cli.Command {
	username, password, _ := customizationFlags()

	disk := &cli.StringFlag{
		Name:     "disk",
		Usage:    "disk image to target, ex: 'vEOS-lab-4.22.1F.vmdk'",
		Required: true,
	}

	repo := &cli.StringFlag{
		Name:     "repo",
		Usage:    "name of repository to tag packaged instance to",
		Required: false,
	}

	tag := &cli.StringFlag{
		Name:     "tag",
		Usage:    "version tag to tag packaged instance to",
		Required: false,
	}

	vendor, platform, version := platformTargetFlags()

	return []*cli.Command{{
		Name:  "package",
		Usage: "package a vm instance as a container",
		Flags: []cli.Flag{
			disk,
			username,
			password,
			repo,
			tag,
			vendor,
			platform,
			version,
		},
		Action: func(c *cli.Context) error {
			return packageBuild(
				c.String("disk"),
				c.String("username"),
				c.String("password"),
				c.String("repo"),
				c.String("tag"),
				c.String("vendor"),
				c.String("platform"),
				c.String("version"),
			)
		},
	}}
}

func packageBuild(disk, username, password, repo, tag, vendor, platform, version string) error {
	err := checkSudo()
	if err != nil {
		return err
	}

	l, li, err := spinLogger()
	if err != nil {
		return err
	}

	b, err := boxen.NewBoxen(boxen.WithLogger(li))
	if err != nil {
		return err
	}

	return spin(
		l,
		li,
		func() error {
			return b.PackageBuild(disk, username, password, repo, tag, vendor, platform, version)
		},
	)
}
