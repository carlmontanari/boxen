package docker

import (
	"io"

	"github.com/carlmontanari/boxen/boxen/command"
)

const (
	dockerCmd = "docker"
)

type args struct {
	workDir      string
	dockerfile   string
	repo         string
	tag          string
	container    string
	cidFile      string
	privileged   bool
	commitChange string
	nocache      bool
	stdOut       io.Writer
	stdErr       io.Writer
}

func setExecuteArgs(a *args) []command.ExecuteOption {
	var executeArgs []command.ExecuteOption

	if a.workDir != "" {
		executeArgs = append(executeArgs, command.WithWorkDir(a.workDir))
	}

	if a.stdOut != nil {
		executeArgs = append(executeArgs, command.WithStdOut(a.stdOut))
	}

	if a.stdErr != nil {
		executeArgs = append(executeArgs, command.WithStdErr(a.stdErr))
	}

	if a.privileged {
		executeArgs = append(executeArgs, command.WithSudo(true))
	}

	return executeArgs
}
