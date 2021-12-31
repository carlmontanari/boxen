package platforms

import (
	"errors"

	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/util"
)

func setStartArgs(opts ...instance.Option) (*startArgs, []instance.Option, error) {
	a := &startArgs{}

	var instanceOpts []instance.Option

	for _, option := range opts {
		err := option(a)

		if err != nil {
			if errors.Is(err, util.ErrIgnoredOption) {
				instanceOpts = append(instanceOpts, option)
				continue
			} else {
				return nil, nil, err
			}
		}
	}

	return a, instanceOpts, nil
}

func setInstallArgs(opts ...instance.Option) (*installArgs, []instance.Option, error) {
	a := &installArgs{}

	var instanceOpts []instance.Option

	for _, option := range opts {
		err := option(a)

		if err != nil {
			if errors.Is(err, util.ErrIgnoredOption) {
				instanceOpts = append(instanceOpts, option)
				continue
			} else {
				return nil, nil, err
			}
		}
	}

	return a, instanceOpts, nil
}
