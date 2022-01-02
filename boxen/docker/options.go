package docker

import "io"

// Option sets a docker args option.
type Option func(*args) error

// WithWorkDir sets the working directory argument for docker operations.
func WithWorkDir(workDir string) Option {
	return func(o *args) error {
		o.workDir = workDir

		return nil
	}
}

// WithDockerfile sets '-f' dockerfile argument to provided name f for docker operations.
func WithDockerfile(f string) Option {
	return func(o *args) error {
		o.dockerfile = f

		return nil
	}
}

// WithCidFile sets the cidfile argument to the provided name f for docker operations.
func WithCidFile(f string) Option {
	return func(o *args) error {
		o.cidFile = f

		return nil
	}
}

// WithPrivileged allows for running containers with the privileged flag set.
func WithPrivileged(b bool) Option {
	return func(o *args) error {
		o.privileged = b

		return nil
	}
}

// WithRepo sets the container repository value for docker operations.
func WithRepo(s string) Option {
	return func(o *args) error {
		o.repo = s

		return nil
	}
}

// WithTag sets the container tag value for docker operations.
func WithTag(s string) Option {
	return func(o *args) error {
		o.tag = s

		return nil
	}
}

// WithContainer sets the container id value for docker operations.
func WithContainer(s string) Option {
	return func(o *args) error {
		o.container = s

		return nil
	}
}

// WithCommitChange sets the docker args commit changes value to the provided string s.
func WithCommitChange(s string) Option {
	return func(o *args) error {
		o.commitChange = s

		return nil
	}
}

// WithStdOut sets the docker args stdout flag to the provided io.Writer.
func WithStdOut(stdout io.Writer) Option {
	return func(o *args) error {
		o.stdOut = stdout

		return nil
	}
}

// WithStdErr sets the docker args stderr flag to the provided io.Writer.
func WithStdErr(stderr io.Writer) Option {
	return func(o *args) error {
		o.stdErr = stderr

		return nil
	}
}

// WithNoCache sets the docker args flag to disable cache layers for building container images.
func WithNoCache(c bool) Option {
	return func(o *args) error {
		o.nocache = c

		return nil
	}
}
