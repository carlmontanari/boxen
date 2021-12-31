package docker

import (
	"fmt"

	"github.com/carlmontanari/boxen/boxen/command"
)

func Run(opts ...Option) (*command.Result, error) {
	a := &args{}

	for _, o := range opts {
		err := o(a)

		if err != nil {
			return nil, err
		}
	}

	if a.repo == "" || a.tag == "" {
		panic("repo and tag not provided, can't run container")
	}

	cmdArgs := []string{"run"}

	if a.cidFile != "" {
		cmdArgs = append(cmdArgs, "--cidfile", a.cidFile)
	}

	if a.privileged {
		cmdArgs = append(cmdArgs, "--privileged")
	}

	cmdArgs = append(cmdArgs, fmt.Sprintf("%s:%s", a.repo, a.tag))

	executeArgs := setExecuteArgs(a)

	executeArgs = append(executeArgs, command.WithArgs(cmdArgs))

	r, err := command.Execute(dockerCmd, executeArgs...)
	if err != nil {
		return r, err
	}

	err = r.CheckStdErr()

	return r, err
}
