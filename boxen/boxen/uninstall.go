package boxen

import (
	"fmt"
	"os"
	"strings"

	"github.com/carlmontanari/boxen/boxen/util"
)

// UnInstall removes an installed source disk from the local boxen config.
func (b *Boxen) UnInstall(pT, disk string) error {
	b.Logger.Infof("uninstall disk '%s' for platform type '%s' requested", disk, pT)

	_, ok := b.Config.Platforms[pT]
	if !ok {
		msg := fmt.Sprintf(
			"no disks for platform '%s' in config, cannot continue",
			pT,
		)

		b.Logger.Critical(msg)

		return fmt.Errorf(
			"%w: %s",
			util.ErrAllocationError,
			msg,
		)
	}

	err := os.RemoveAll(
		fmt.Sprintf(
			"%s/%s/%s",
			b.Config.Options.Build.SourcePath,
			pT,
			disk,
		),
	)
	if err != nil {
		b.Logger.Criticalf("error deleting installation files: %s", err)

		return err
	}

	updatedSourceDisks := make([]string, 0)

	for _, d := range b.Config.Platforms[pT].SourceDisks {
		if !strings.HasPrefix(d, disk) {
			updatedSourceDisks = append(updatedSourceDisks, d)
		}
	}

	b.Config.Platforms[pT].SourceDisks = updatedSourceDisks

	if len(b.Config.Platforms[pT].SourceDisks) == 0 {
		b.Logger.Debug("no disks remain for platform, deleting platform source directory")

		delete(b.Config.Platforms, pT)

		// also delete the platform dir in the source path if there are no more disks remaining
		// for the given platform type
		err = os.RemoveAll(
			fmt.Sprintf("%s/%s", b.Config.Options.Build.SourcePath, pT),
		)
		if err != nil {
			b.Logger.Criticalf("error deleting source disk directory: %s", err)

			return err
		}
	}

	err = b.Config.Dump(b.ConfigPath)
	if err != nil {
		b.Logger.Criticalf("error dumping updated boxen config to disk: %s", err)
		return err
	}

	b.Logger.Infof("uninstall disk '%s' for platform type '%s' completed successfully", disk, pT)

	return nil
}
