package boxen

import (
	"fmt"
	"os"

	"github.com/carlmontanari/boxen/boxen/config"
	"github.com/carlmontanari/boxen/boxen/util"
)

// Init initializes a boxen directory structure.
func (b *Boxen) Init(d string) error {
	b.Logger.Infof("init boxen directory requested for directory '%s'", d)

	d = util.ExpandPath(d)
	b.Logger.Debugf("requested directory resolved as '%s'", d)

	if util.DirectoryExists(d) {
		return fmt.Errorf(
			"%w: requested directory '%s' already exists, cannot continue",
			util.ErrAllocationError,
			d,
		)
	}

	instanceD := fmt.Sprintf("%s/instances", d)
	sourceD := fmt.Sprintf("%s/source", d)

	err := os.Mkdir(d, os.ModePerm)
	if err != nil {
		b.Logger.Criticalf("error creating requested directory: %s", err)

		return err
	}

	err = os.Mkdir(instanceD, os.ModePerm)
	if err != nil {
		b.Logger.Criticalf("error creating instance directory: %s", err)
		return err
	}

	err = os.Mkdir(sourceD, os.ModePerm)
	if err != nil {
		b.Logger.Criticalf("error creating source directory: %s", err)
		return err
	}

	b.Logger.Debug("boxen directories created successfully")

	b.Config = config.NewConfig()

	b.Config.Options.Build.InstancePath = instanceD
	b.Config.Options.Build.SourcePath = sourceD

	err = b.Config.Dump(fmt.Sprintf("%s/boxen.yaml", d))
	if err != nil {
		b.Logger.Criticalf("error dumping boxen initial config to disk: %s", err)
		return err
	}

	b.Logger.Info("init boxen completed successfully")

	return nil
}
