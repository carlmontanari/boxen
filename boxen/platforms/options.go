package platforms

import (
	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/util"
)

type installArgs struct {
	configLines []string
}

// WithInstallConfig sets an option to push configs during platform installation.
func WithInstallConfig(configLines []string) instance.Option {
	return func(o interface{}) error {
		a, ok := o.(*installArgs)

		if ok {
			a.configLines = configLines
			return nil
		}

		return util.ErrIgnoredOption
	}
}

type startArgs struct {
	prepareConsole bool
	runUntilSigint bool
}

// WithPrepareConsole sets an option to notify the platform implementation to prepare the console
// connection to receive configurations/commands. This typically will mean handling any initial
// login after a "start ready" state, and disabling paging and the like.
func WithPrepareConsole(b bool) instance.Option {
	return func(o interface{}) error {
		a, ok := o.(*startArgs)

		if ok {
			a.prepareConsole = b
			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithRunUntilSigint sets an option to launch health check and run until signal interrupt.
func WithRunUntilSigint(b bool) instance.Option {
	return func(o interface{}) error {
		a, ok := o.(*startArgs)

		if ok {
			a.runUntilSigint = b
			return nil
		}

		return util.ErrIgnoredOption
	}
}
