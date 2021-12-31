package boxen

import (
	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/platforms"
)

// Stop stops a local boxen instance.
func (b *Boxen) Stop(name string) error {
	b.Logger.Infof("stop for instance '%s' requested", name)

	q, err := platforms.NewPlatformFromConfig(
		name,
		b.Config,
		&instance.Loggers{
			Base:    b.Logger,
			Stdout:  nil,
			Stderr:  nil,
			Console: nil,
		},
	)
	if err != nil {
		b.Logger.Criticalf("error spawning instance from config: %s", err)

		return err
	}

	b.modifyInstanceMap(func() { b.Instances[name] = q })

	err = q.Stop(instance.WithSudo(true))
	if err != nil {
		return err
	}

	b.Logger.Infof("stop for instance '%s' completed successfully", name)

	return nil
}
