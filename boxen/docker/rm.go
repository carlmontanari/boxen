package docker

import (
	"fmt"

	"github.com/carlmontanari/boxen/boxen/command"
)

func RmImage(repo, tag string, opts ...Option) error {
	a := &args{}

	for _, o := range opts {
		err := o(a)

		if err != nil {
			return err
		}
	}

	cmdArgs := []string{"image", "rm", fmt.Sprintf("%s:%s", repo, tag)}

	executeArgs := setExecuteArgs(a)

	executeArgs = append(executeArgs, command.WithArgs(cmdArgs))

	r, err := command.Execute(dockerCmd, executeArgs...)
	if err != nil {
		return err
	}

	return r.Proc.Wait()
}

func RmContainer(container string, opts ...Option) error {
	a := &args{}

	for _, o := range opts {
		err := o(a)

		if err != nil {
			return err
		}
	}

	cmdArgs := []string{"rm", container}

	executeArgs := setExecuteArgs(a)

	executeArgs = append(executeArgs, command.WithArgs(cmdArgs))

	r, err := command.Execute(dockerCmd, executeArgs...)
	if err != nil {
		return err
	}

	return r.Proc.Wait()
}
