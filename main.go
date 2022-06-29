package main

import (
	"fmt"
	"os"

	"github.com/carlmontanari/boxen/boxen/logging"

	"github.com/carlmontanari/boxen/boxen/cli"
)

func main() {
	err := cli.NewCLI().Run(os.Args)

	logging.Manager.Terminate()

	if err != nil {
		fmt.Println(err)

		os.Exit(1)
	}
}
