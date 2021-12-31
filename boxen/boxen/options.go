package boxen

import "github.com/carlmontanari/boxen/boxen/logging"

type Option func(*args) error

// WithConfig allows for passing a config file path f to the NewBoxen function.
func WithConfig(f string) Option {
	return func(o *args) error {
		o.config = f

		return nil
	}
}

// WithLogger allows for passing a boxen logging.Instance l to the NewBoxen function.
func WithLogger(l *logging.Instance) Option {
	return func(o *args) error {
		o.logger = l

		return nil
	}
}
