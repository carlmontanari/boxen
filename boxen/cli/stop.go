package cli

import (
	"strings"

	"github.com/carlmontanari/boxen/boxen/boxen"
)

func Stop(config, instances string) error {
	err := checkSudo()
	if err != nil {
		return err
	}

	l, li, err := spinLogger()
	if err != nil {
		return err
	}

	b, err := boxen.NewBoxen(boxen.WithLogger(li), boxen.WithConfig(config))
	if err != nil {
		return err
	}

	return spin(l, li, func() error {
		return instanceOp(b.Stop, instances)
	})
}

func StopGroup(config, group string) error {
	err := checkSudo()
	if err != nil {
		return err
	}

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
		return instanceOp(b.Stop, strings.Join(instances, ","))
	})
}
