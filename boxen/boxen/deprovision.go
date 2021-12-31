package boxen

import (
	"fmt"
	"os"
)

// DeProvision does what it says -- it "deprovisions" an instance from the Boxen configuration. This
// is only useful/relevant for "local" boxen operations, i.e. VMs not containerized/packaged as you
// would use with Containerlab.
func (b *Boxen) DeProvision(instance string) error {
	b.Logger.Infof("de-provision for instance '%s' requested", instance)

	err := os.RemoveAll(fmt.Sprintf("%s/%s", b.Config.Options.Build.InstancePath, instance))
	if err != nil {
		b.Logger.Criticalf("error deleting instance directory: %s", err)

		return err
	}

	b.Config.DeleteInstance(instance)

	err = b.Config.Dump(b.ConfigPath)
	if err != nil {
		b.Logger.Criticalf("error dumping updated boxen config to disk: %s", err)

		return err
	}

	b.Logger.Infof("de-provision for instance '%s' completed successfully", instance)

	return nil
}
