package logging

import (
	"fmt"
	"strings"
)

type Option func(i *Instance) error

func WithLevel(l string) Option {
	return func(i *Instance) error {
		l = strings.ToLower(l)

		for _, v := range []string{debug, info, critical} {
			if l == v {
				i.level = levelMap[l]

				return nil
			}
		}

		return fmt.Errorf(
			"%w: invalid logging level '%s' provided, must be one of 'debug', 'info', 'critical'",
			ErrLogError, l,
		)
	}
}
