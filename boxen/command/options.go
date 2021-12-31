package command

import (
	"io"
	"time"
)

type ExecuteOption func(*args) error

func WithArgs(a []string) ExecuteOption {
	return func(o *args) error {
		o.args = a

		return nil
	}
}

func WithWorkDir(workDir string) ExecuteOption {
	return func(o *args) error {
		o.workDir = workDir

		return nil
	}
}

func WithSudo(sudo bool) ExecuteOption {
	return func(o *args) error {
		o.sudo = sudo

		return nil
	}
}

func WithStdOut(stdout io.Writer) ExecuteOption {
	return func(o *args) error {
		o.stdOut = stdout

		return nil
	}
}

func WithStdErr(stderr io.Writer) ExecuteOption {
	return func(o *args) error {
		o.stdErr = stderr

		return nil
	}
}

func WithWait(wait bool) ExecuteOption {
	return func(o *args) error {
		o.wait = wait

		return nil
	}
}

type CheckOption func(*checkArgs) error

func WithIgnore(i [][]byte) CheckOption {
	return func(o *checkArgs) error {
		o.ignore = i

		return nil
	}
}

func WithIsError(i [][]byte) CheckOption {
	return func(o *checkArgs) error {
		o.isError = i

		return nil
	}
}

func WithDuration(t time.Duration) CheckOption {
	return func(o *checkArgs) error {
		o.duration = t

		return nil
	}
}

func WithInterval(t time.Duration) CheckOption {
	return func(o *checkArgs) error {
		o.interval = t

		return nil
	}
}
