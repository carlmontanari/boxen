package docker

import (
	"fmt"

	"github.com/carlmontanari/boxen/boxen/command"
)

// CopyFromContainer copies file/path 's' from the container ID provided in the options to the
// destination 'd' on the local filesystem.
func CopyFromContainer(s, d string, opts ...Option) error {
	a := &args{}

	for _, o := range opts {
		err := o(a)

		if err != nil {
			return err
		}
	}

	if a.container == "" {
		panic("container id not provided, can't copy")
	}

	cmdArgs := []string{"cp", fmt.Sprintf("%s:%s", a.container, s), d}

	executeArgs := setExecuteArgs(a)

	executeArgs = append(executeArgs, command.WithArgs(cmdArgs))

	r, err := command.Execute(dockerCmd, executeArgs...)
	if err != nil {
		return err
	}

	return r.Proc.Wait()
}
