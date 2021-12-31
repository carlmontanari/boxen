package instance

import "github.com/carlmontanari/boxen/boxen/util"

type qemuOpts struct {
	launchModifier func(c *QemuLaunchCmd)
	sudo           bool
}

// WithLaunchModifier sets an option to modify qemu launch command.
func WithLaunchModifier(f func(c *QemuLaunchCmd)) Option {
	return func(o interface{}) error {
		q, ok := o.(*qemuOpts)

		if ok {
			q.launchModifier = f
			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithSudo tells boxen to start/stop instances with sudo or not.
func WithSudo(b bool) Option {
	return func(o interface{}) error {
		q, ok := o.(*qemuOpts)

		if ok {
			q.sudo = b
			return nil
		}

		return util.ErrIgnoredOption
	}
}
