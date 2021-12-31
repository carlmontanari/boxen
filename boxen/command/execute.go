package command

import (
	"io"
	"os/exec"

	"github.com/carlmontanari/boxen/boxen/util"
)

type args struct {
	args    []string
	workDir string
	sudo    bool
	stdOut  io.Writer
	stdErr  io.Writer
	wait    bool
}

func Execute(cmd string, opts ...ExecuteOption) (*Result, error) {
	a := &args{}

	for _, o := range opts {
		err := o(a)

		if err != nil {
			return nil, err
		}
	}

	if a.sudo {
		cmd, a.args = newSudoer().updateCmd(cmd, a.args)
	}

	r := &Result{
		stdout:    util.NewLockingWriterReader(),
		stderr:    util.NewLockingWriterReader(),
		stderrInt: util.NewLockingWriterReader(),
	}

	r.Proc = exec.Command(cmd, a.args...) //nolint:gosec

	if a.workDir != "" {
		r.Proc.Dir = a.workDir
	}

	err := r.setIO(a)
	if err != nil {
		return r, err
	}

	err = r.Proc.Start()
	if err != nil {
		return r, err
	}

	if a.wait {
		err = r.Proc.Wait()
	}

	return r, err
}
