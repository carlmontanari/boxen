package boxen

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/carlmontanari/boxen/boxen/command"
	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/platforms"
	"github.com/carlmontanari/boxen/boxen/util"
)

func (b *Boxen) copySourceDiskToInstanceDir(name, disk string) error {
	var err error

	if b.Config.Options.Qemu.UseThickDisks {
		err = util.CopyFile(
			fmt.Sprintf(
				"%s/%s/%s/disk.qcow2",
				b.Config.Options.Build.SourcePath,
				b.Config.Instances[name].PlatformType,
				b.Config.Instances[name].Disk,
			),
			disk,
		)
	} else {
		_, err = command.Execute(
			util.QemuImgCmd,
			command.WithArgs(
				[]string{"create", "-f", "qcow2", "-F", "qcow2", "-b", fmt.Sprintf(
					"%s/%s/%s/disk.qcow2",
					b.Config.Options.Build.SourcePath,
					b.Config.Instances[name].PlatformType,
					b.Config.Instances[name].Disk,
				), disk},
			),
			command.WithWait(true),
		)
	}

	if err != nil {
		b.Logger.Criticalf("error creating instance disk: %s\n", err)

		return err
	}

	return nil
}

func (b *Boxen) copySourceRunFilesToInstanceDir(name, disk string) error {
	runFiles, err := filepath.Glob(
		fmt.Sprintf("%s/%s/%s/[^disk.qcow2]*", b.Config.Options.Build.SourcePath,
			b.Config.Instances[name].PlatformType,
			b.Config.Instances[name].Disk),
	)

	if err != nil {
		b.Logger.Criticalf("error globbing run files associated with disk: %s\n", err)

		return err
	}

	for _, f := range runFiles {
		err = util.CopyFile(f, fmt.Sprintf("%s/%s", filepath.Dir(disk), filepath.Base(f)))

		if err != nil {
			b.Logger.Criticalf(
				"error copying run file '%s' to instance directory: %s\n",
				f,
				err,
			)

			return err
		}
	}

	return nil
}

func (b *Boxen) startCheckDisk(name, disk string) error {
	var err error

	diskExists := util.FileExists(disk)

	// in the future we will panic/err out if the disk exists and the persist mode (not implemented
	// yet) is false. for now, we will just copy the disk over if it doesn't exist.

	if !diskExists {
		err = b.copySourceDiskToInstanceDir(name, disk)
		if err != nil {
			return err
		}

		err = b.copySourceRunFilesToInstanceDir(name, disk)
		if err != nil {
			return err
		}
	}

	return nil
}

// Start starts a local boxen instance.
func (b *Boxen) Start(name string) error {
	b.Logger.Infof("start for instance '%s' requested", name)

	_, ok := b.Config.Instances[name]
	if !ok {
		return fmt.Errorf("%w: no instance name '%s' in the config", util.ErrInstanceError, name)
	}

	instanceDir := fmt.Sprintf("%s/%s", b.Config.Options.Build.InstancePath, name)
	instanceDirExists := util.DirectoryExists(instanceDir)

	if !instanceDirExists {
		err := os.Mkdir(instanceDir, os.ModePerm)
		if err != nil {
			b.Logger.Criticalf("error creating instance directory: %s", err)

			return err
		}
	}

	il, err := instance.NewInstanceLoggersFOut(b.Logger, instanceDir)
	if err != nil {
		b.Logger.Criticalf("error instantiating loggers for instance: %s", err)

		return err
	}

	// snag the disk version the instance should be using, then update the in memory config value of
	// that disk to the instanceDir + "disk.qcow2" since that's what we name all boot disks. We do
	// this just for spawning the instance, we'll set it back to the diskVer after so that the
	// config file always shows the version that the instance was provisioned with.
	diskVer := b.Config.Instances[name].Disk
	instanceDisk := fmt.Sprintf("%s/disk.qcow2", instanceDir)
	b.Config.Instances[name].Disk = instanceDisk

	q, err := platforms.NewPlatformFromConfig(
		name,
		b.Config,
		il,
	)
	if err != nil {
		b.Logger.Criticalf("error spawning instance from config: %s", err)

		return err
	}

	b.modifyInstanceMap(func() { b.Instances[name] = q })

	// set the disk name back to the version for the config
	b.Config.Instances[name].Disk = diskVer

	err = b.startCheckDisk(name, instanceDisk)
	if err != nil {
		return err
	}

	// now that we've sorted out the disk setup we can set the final/resolved disk on the in memory
	// instance setup
	b.Config.Instances[name].Disk = instanceDisk

	if b.Config.Instances[name].BootDelay > 0 {
		b.Logger.Infof("boot delay set, sleeping '%d' seconds", b.Config.Instances[name].BootDelay)

		time.Sleep(time.Duration(b.Config.Instances[name].BootDelay) * time.Second)
	}

	initialConfig := false

	if b.Config.Instances[name].StartupConfig != "" {
		initialConfig = true
	}

	err = q.Start(platforms.WithPrepareConsole(initialConfig))
	if err != nil {
		return err
	}

	if initialConfig {
		err = q.InstallConfig(b.Config.Instances[name].StartupConfig, true)
		if err != nil {
			return err
		}
	}

	b.Config.Instances[name].PID = q.GetPid()

	// reset the in memory config disk version back to the actual source disk version prior to
	// dumping the config to disk
	b.Config.Instances[name].Disk = diskVer

	err = b.Config.Dump(b.ConfigPath)
	if err != nil {
		b.Logger.Criticalf("error dumping updated boxen config to disk: %s", err)
		return err
	}

	b.Logger.Infof("start for instance '%s' completed successfully", name)

	return nil
}
