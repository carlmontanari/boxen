package docker

import "io"

type Option func(*args) error

func WithWorkDir(workDir string) Option {
	return func(o *args) error {
		o.workDir = workDir

		return nil
	}
}

func WithDockerfile(f string) Option {
	return func(o *args) error {
		o.dockerfile = f

		return nil
	}
}

func WithCidFile(f string) Option {
	return func(o *args) error {
		o.cidFile = f

		return nil
	}
}

func WithPrivileged(b bool) Option {
	return func(o *args) error {
		o.privileged = b

		return nil
	}
}

func WithRepo(s string) Option {
	return func(o *args) error {
		o.repo = s

		return nil
	}
}

func WithTag(s string) Option {
	return func(o *args) error {
		o.tag = s

		return nil
	}
}

func WithContainer(s string) Option {
	return func(o *args) error {
		o.container = s

		return nil
	}
}

func WithCommitChange(s string) Option {
	return func(o *args) error {
		o.commitChange = s

		return nil
	}
}

func WithStdOut(stdout io.Writer) Option {
	return func(o *args) error {
		o.stdOut = stdout

		return nil
	}
}

func WithStdErr(stderr io.Writer) Option {
	return func(o *args) error {
		o.stdErr = stderr

		return nil
	}
}

func WithNoCache(c bool) Option {
	return func(o *args) error {
		o.nocache = c

		return nil
	}
}
