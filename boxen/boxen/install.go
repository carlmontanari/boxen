package boxen

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/carlmontanari/boxen/boxen/config"
	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/platforms"
	"github.com/carlmontanari/boxen/boxen/util"
)

type installInfo struct {
	inDisk       string
	srcDisk      *Disk
	newDisk      string
	username     string
	password     string
	config       []string
	installFiles []string
	name         string
	tmpDir       string
}

func (b *Boxen) installAllocateInstance(
	i *installInfo,
) error {
	platformDefaultProfile, err := GetDefaultProfile(i.srcDisk.PlatformType)
	if err != nil {
		b.Logger.Criticalf(
			"failed fetching default profile for platform type '%s'\n",
			i.srcDisk.PlatformType,
		)

		return err
	}

	_, ok := b.Config.Platforms[i.srcDisk.PlatformType].Profiles["default"]
	if !ok {
		b.Config.Platforms[i.srcDisk.PlatformType].Profiles["default"] = platformDefaultProfile
	}

	instanceID, err := b.allocateInstanceIDReverse()
	if err != nil {
		b.Logger.Critical("failed allocating instance ID for installation process")

		return err
	}

	serialPorts, err := b.allocateSerialPorts(
		platformDefaultProfile.Hardware.SerialPortCount,
		instanceID,
	)
	if err != nil {
		b.Logger.Critical("failed to allocate serial port ID(s) for installation process")

		return err
	}

	monitorPort := b.allocateMonitorPort(instanceID)

	// updating config but only in memory for the installation
	b.Config.Instances[i.name] = &config.Instance{
		Name:         i.name,
		PlatformType: i.srcDisk.PlatformType,
		Disk:         i.newDisk,
		ID:           instanceID,
		PID:          0,
		Profile:      "",
		Credentials:  config.NewDefaultCredentials(),
		Hardware:     platformDefaultProfile.Hardware.ToHardware(),
		MgmtIntf: &config.MgmtIntf{
			Nat: &config.Nat{
				TCP: zipDefaultTCPNats(platformDefaultProfile.TPCNatPorts),
				UDP: zipDefaultUDPNats(platformDefaultProfile.UDPNatPorts),
			},
			Bridge: nil,
		},
		DataPlaneIntf: nil,
		Advanced:      platformDefaultProfile.Advanced,
	}

	b.Config.Instances[i.name].Hardware.SerialPorts = serialPorts
	b.Config.Instances[i.name].Hardware.MonitorPort = monitorPort

	if len(i.username) > 0 {
		b.Config.Instances[i.name].Credentials.Username = i.username
	}

	if len(i.password) > 0 {
		b.Config.Instances[i.name].Credentials.Password = i.password
	}

	return nil
}

func (b *Boxen) installProvisionInstance(i *installInfo) error {
	var err error

	_, ok := b.Config.Platforms[i.srcDisk.PlatformType]
	if !ok {
		b.Config.Platforms[i.srcDisk.PlatformType] = config.NewPlatform()
	}

	err = b.installAllocateInstance(i)
	if err != nil {
		b.Logger.Critical("error allocating resource for installation")

		return err
	}

	il, err := instance.NewInstanceLoggersFOut(b.Logger, i.tmpDir)
	if err != nil {
		return err
	}

	b.Instances[i.name], err = platforms.NewPlatformFromConfig(
		i.name,
		b.Config,
		il,
	)
	if err != nil {
		b.Logger.Critical("error building qemu instance for installation")

		return err
	}

	i.config, err = b.RenderInitialConfig(i.name)
	if err != nil {
		return err
	}

	_, i.installFiles, err = b.Instances[i.name].Package(filepath.Dir(i.inDisk), i.tmpDir)
	if err != nil {
		b.Logger.Critical("error during package phase of installation")

		return err
	}

	return nil
}

func (b *Boxen) installTransferFinalFiles(
	i *installInfo,
) error {
	pTSourceDir := fmt.Sprintf("%s/%s", b.Config.Options.Build.SourcePath, i.srcDisk.PlatformType)
	pTVersionSourceDir := fmt.Sprintf("%s/%s", pTSourceDir, i.srcDisk.Version)

	// platform type source dir may exist already, so check that and if it doesn't make it,
	// otherwise, all good to move onto making the disk version dir
	ptSourceDirExists := util.DirectoryExists(pTSourceDir)

	if !ptSourceDirExists {
		err := os.Mkdir(pTSourceDir, os.ModePerm)
		if err != nil {
			b.Logger.Criticalf("error creating platform type source disk directory: %s\n", err)

			return err
		}
	}

	err := os.Mkdir(pTVersionSourceDir, os.ModePerm)
	if err != nil {
		b.Logger.Criticalf("error creating source disk version directory: %s\n", err)

		return err
	}

	err = util.CopyFile(
		i.newDisk,
		fmt.Sprintf(
			"%s/disk.qcow2",
			pTVersionSourceDir,
		),
	)
	if err != nil {
		b.Logger.Criticalf("error copying installed disk to source disk directory: %s\n", err)

		return err
	}

	for _, f := range i.installFiles {
		err = util.CopyFile(
			fmt.Sprintf("%s/%s", i.tmpDir, f),
			fmt.Sprintf("%s/%s", pTVersionSourceDir, f),
		)
		if err != nil {
			b.Logger.Criticalf(
				"error copying required installation files to source disk directory: %s\n", err,
			)

			return err
		}
	}

	return nil
}

func (b *Boxen) installUpdateConfig(i *installInfo) error {
	b.Config.Platforms[i.srcDisk.PlatformType].SourceDisks = append(
		b.Config.Platforms[i.srcDisk.PlatformType].SourceDisks,
		i.srcDisk.Version,
	)

	// delete the instance out of the config since we don't need it anymore
	delete(b.Config.Instances, i.name)

	return b.Config.Dump(b.ConfigPath)
}

// Install "installs" a disk as a source disk which local instances can be provisioned to boot from.
// This is only useful/relevant in the local VM (not Containerlab) mode of operation. Install
// handles disabling initial config dialogs/prompts, setting base credentials, and then storing the
// image for use for later provisioned instances.
func (b *Boxen) Install(
	disk, username, password string,
) error {
	b.Logger.Infof("install requested for disk '%s'", disk)

	i := &installInfo{inDisk: disk, username: username, password: password}

	var err error

	i.tmpDir, err = ioutil.TempDir(os.TempDir(), "boxen")
	if err != nil {
		b.Logger.Criticalf("error creating temporary working directory: %s\n", err)

		return err
	}

	b.Logger.Debugf("temporary directory '%s' created successfully", i.tmpDir)

	if util.GetEnvIntOrDefault("BOXEN_DEV_MODE", 0) > 0 {
		b.Logger.Info(
			"boxen dev mode enabled, not deleting temporary directory after installation",
		)
	} else {
		defer os.RemoveAll(i.tmpDir)
	}

	err = b.installAllocateDisks(i)
	if err != nil {
		return err
	}

	b.Logger.Debug("disks allocated for installation")

	if b.sourceDiskExists(i.srcDisk.PlatformType, i.srcDisk.Version) {
		msg := fmt.Sprintf(
			"source disk for platform type '%s' of version '%s' already exists, cannot continue",
			i.srcDisk.PlatformType,
			i.srcDisk.Version,
		)

		b.Logger.Critical(msg)

		return fmt.Errorf(
			"%w: %s",
			util.ErrAllocationError,
			msg,
		)
	}

	b.Logger.Debug("disk allocation complete")

	err = b.installProvisionInstance(i)
	if err != nil {
		return err
	}

	b.Logger.Info("initial provisioning complete")

	err = b.Instances[i.name].Install(
		platforms.WithInstallConfig(i.config),
		instance.WithSudo(true),
	)
	if err != nil {
		return err
	}

	b.Logger.Info("initial installation complete")

	err = b.installTransferFinalFiles(i)
	if err != nil {
		return err
	}

	b.Logger.Debug("post installation file transfers complete")

	err = b.installUpdateConfig(i)
	if err != nil {
		return err
	}

	b.Logger.Debug("post installation config update complete")

	b.Logger.Infof("install completed for disk '%s'", disk)

	return nil
}
