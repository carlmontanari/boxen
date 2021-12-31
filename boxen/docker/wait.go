package docker

import (
	"github.com/carlmontanari/boxen/boxen/command"
)

func Wait(opts ...Option) error {
	a := &args{}

	for _, o := range opts {
		err := o(a)

		if err != nil {
			return err
		}
	}

	if a.container == "" {
		panic("container id not provided, can't wait")
	}

	cmdArgs := []string{"wait", a.container}

	executeArgs := setExecuteArgs(a)

	executeArgs = append(executeArgs, command.WithArgs(cmdArgs))

	r, err := command.Execute(dockerCmd, executeArgs...)
	if err != nil {
		return err
	}

	return r.Proc.Wait()
}
