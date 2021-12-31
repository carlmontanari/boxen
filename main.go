package main

import (
	"fmt"
	"os"

	"github.com/carlmontanari/boxen/boxen/cli"
	"github.com/carlmontanari/boxen/boxen/logging"
)

func main() {
	defer logging.Manager.Terminate()

	err := cli.NewCLI().Run(os.Args)

	if err != nil {
		fmt.Println(err)
	}
}
