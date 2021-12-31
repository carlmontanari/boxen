package docker

import (
	"fmt"

	"github.com/carlmontanari/boxen/boxen/command"
)

func Commit(opts ...Option) error {
	a := &args{}

	for _, o := range opts {
		err := o(a)

		if err != nil {
			return err
		}
	}

	if a.container == "" {
		panic("container id not provided, can't commit container")
	}

	if a.repo == "" || a.tag == "" {
		panic("repo and tag not provided, can't commit container")
	}

	cmdArgs := []string{"commit"}

	if a.commitChange != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--change=%s", a.commitChange))
	}

	cmdArgs = append(cmdArgs, a.container, fmt.Sprintf("%s:%s", a.repo, a.tag))

	executeArgs := setExecuteArgs(a)

	executeArgs = append(executeArgs, command.WithArgs(cmdArgs))

	r, err := command.Execute(dockerCmd, executeArgs...)
	if err != nil {
		return err
	}

	err = r.CheckStdErr()
	if err != nil {
		return err
	}

	return r.Proc.Wait()
}
