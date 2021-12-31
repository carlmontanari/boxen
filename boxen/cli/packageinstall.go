package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/carlmontanari/boxen/boxen/boxen"
	"github.com/carlmontanari/boxen/boxen/logging"
	"github.com/carlmontanari/boxen/boxen/util"

	"github.com/urfave/cli/v2"
)

func packageInstallCommands() []*cli.Command {
	return []*cli.Command{{
		Name:   "package-install",
		Usage:  "install/finalize a vm instance packaging in a container",
		Hidden: true,
		Action: func(c *cli.Context) error {
			return packageInstall()
		},
	}}
}

func packageInstall() error {
	v, ok := os.LookupEnv("BOXEN_LOG_TARGET")
	if !ok {
		// for this block and the rest, we actually do want to panic since that will kill the
		// container, dump the panic output to container logs, and ultimately bubble the error back
		// up to the cli process that is managing the installation.
		panic("no boxen log target set, this shouldn't happen!")
	}

	parts := strings.Split(v, ":")
	a := parts[0]
	p, _ := strconv.Atoi(parts[1])

	sl, err := logging.NewSocketSender(a, p)
	if err != nil {
		panic("could not setup socket sender")
	}

	logLevel := util.GetEnvStrOrDefault("BOXEN_LOG_LEVEL", "info")

	li, err := logging.NewInstance(sl.Emit, logging.WithLevel(logLevel))
	if err != nil {
		return err
	}

	b, err := boxen.NewBoxen(boxen.WithLogger(li), boxen.WithConfig("boxen.yaml"))
	if err != nil {
		panic(fmt.Sprintf("error spawning boxen instance: %s\n", err))
	}

	err = b.PackageInstall()

	return err
}
