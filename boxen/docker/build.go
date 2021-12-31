package docker

import (
	"fmt"

	"github.com/carlmontanari/boxen/boxen/command"
)

func Build(opts ...Option) error {
	a := &args{}
	a.repo = "boxen"
	a.tag = "latest"

	for _, o := range opts {
		err := o(a)

		if err != nil {
			return err
		}
	}

	cmdArgs := []string{"build"}

	if a.workDir != "" {
		cmdArgs = append(cmdArgs, a.workDir)
	}

	if a.dockerfile != "" {
		cmdArgs = append(cmdArgs, "-f", a.dockerfile)
	}

	if a.nocache {
		cmdArgs = append(cmdArgs, "--no-cache")
	}

	cmdArgs = append(cmdArgs, "-t", fmt.Sprintf("%s:%s", a.repo, a.tag))

	executeArgs := setExecuteArgs(a)

	executeArgs = append(executeArgs, command.WithArgs(cmdArgs))

	r, err := command.Execute(dockerCmd, executeArgs...)
	if err != nil {
		return err
	}

	return r.Proc.Wait()
}
