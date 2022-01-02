package cli

import (
	"strings"

	"github.com/carlmontanari/boxen/boxen/boxen"
)

// Start starts the provided instance(s) (provided as comma separated string).
func Start(config, instances string) error {
	l, li, err := spinLogger()
	if err != nil {
		return err
	}

	b, err := boxen.NewBoxen(boxen.WithLogger(li), boxen.WithConfig(config))
	if err != nil {
		return err
	}

	return spin(l, li, func() error {
		return instanceOp(b.Start, instances)
	})
}

// StartGroup starts all local instances in a group.
func StartGroup(config, group string) error {
	l, li, err := spinLogger()
	if err != nil {
		return err
	}

	b, err := boxen.NewBoxen(boxen.WithLogger(li), boxen.WithConfig(config))
	if err != nil {
		return err
	}

	instances, err := b.GetGroupInstances(group)
	if err != nil {
		return err
	}

	return spin(l, li, func() error {
		return instanceOp(b.Start, strings.Join(instances, ","))
	})
}
