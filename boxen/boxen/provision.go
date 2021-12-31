package boxen

import (
	"fmt"

	"github.com/carlmontanari/boxen/boxen/config"
	"github.com/carlmontanari/boxen/boxen/util"
)

func (b *Boxen) provision(
	name, platformType, sourceDisk, profileName string,
	profileObj *config.Profile,
) error {
	instanceID, err := b.allocateInstanceID()
	if err != nil {
		b.Logger.Critical("failed allocating instance ID")

		return err
	}

	serialPorts, err := b.allocateSerialPorts(
		profileObj.Hardware.SerialPortCount,
		instanceID,
	)
	if err != nil {
		b.Logger.Critical("failed to allocate serial port ID(s)")

		return err
	}

	monitorPort := b.allocateMonitorPort(instanceID)

	socketListenPorts, err := b.allocateSocketListenPorts(profileObj.Hardware.NicCount)
	if err != nil {
		b.Logger.Critical("failed to allocate socket listen ports")

		return err
	}

	dataPlaneIntfMap := make(map[int]*config.SocketConnectPair)
	for i, val := range socketListenPorts {
		dataPlaneIntfMap[i+1] = &config.SocketConnectPair{
			Connect: -1,
			Listen:  val,
		}
	}

	tcpNats, err := b.allocateMgmtNatPorts(profileObj.TPCNatPorts, nil)
	if err != nil {
		b.Logger.Critical("failed to allocate management tcp nat ports")

		return err
	}

	updNats, err := b.allocateMgmtNatPorts(profileObj.UDPNatPorts, tcpNats)
	if err != nil {
		b.Logger.Critical("failed to allocate management udp nat ports")

		return err
	}

	hw := profileObj.Hardware.ToHardware()
	hw.SerialPorts = serialPorts
	hw.MonitorPort = monitorPort

	b.Config.AddInstance(name, &config.Instance{
		Name:         name,
		PlatformType: platformType,
		Disk:         sourceDisk,
		ID:           instanceID,
		PID:          0,
		Profile:      profileName,
		Credentials:  config.NewDefaultCredentials(),
		Hardware:     hw,
		MgmtIntf: &config.MgmtIntf{
			Nat: &config.Nat{
				TCP: tcpNats,
				UDP: updNats,
			},
			Bridge: nil,
		},
		DataPlaneIntf: &config.DataPlaneIntf{SocketConnectMap: dataPlaneIntfMap},
		Advanced:      profileObj.Advanced,
		BootDelay:     0,
	})

	return nil
}

// Provision creates all the required config objects/port allocation/etc. for a local boxen
// instance.
func (b *Boxen) Provision(instance, vendor, platform, sourceDisk, profile string) error {
	b.Logger.Infof(
		"provision instance '%s' of vendor '%s' platform '%s' requested",
		instance,
		vendor,
		platform,
	)

	_, ok := b.Config.Instances[instance]
	if ok {
		return fmt.Errorf(
			"%w: instance named '%s' already exists, cannot provision",
			util.ErrAllocationError,
			instance,
		)
	}

	platformType := fmt.Sprintf("%s_%s", vendor, platform)

	_, ok = b.Config.Platforms[platformType]
	if !ok {
		return fmt.Errorf(
			"%w: no platform type registered for type '%s', cannot proceed",
			util.ErrAllocationError,
			platformType,
		)
	}

	sourceDisks := b.Config.Platforms[platformType].SourceDisks
	if sourceDisk == "" {
		if len(sourceDisks) == 0 {
			sourceDisk = ""

			b.Logger.Debug(
				"no source disk provided, and no source disks available, this is gunna be a bad time",
			)
		} else {
			sourceDisk = sourceDisks[0]

			b.Logger.Debugf("no source disk provided, using '%s'", sourceDisk)
		}
	}

	if !util.StringSliceContains(sourceDisk, sourceDisks) {
		msg := fmt.Sprintf(
			"source disk '%s' does not exist for platform, cannot proceed",
			sourceDisk,
		)

		b.Logger.Criticalf(msg)

		return fmt.Errorf("%w: %s", util.ErrProvisionError, msg)
	}

	if profile == "" {
		profile = "default"

		b.Logger.Debugf("no profile name provided, using '%s'", profile)
	}

	profileObj, ok := b.Config.Platforms[platformType].Profiles[profile]
	if !ok {
		msg := fmt.Sprintf("profile '%s' does not exist for platform cannot proceed", profile)

		b.Logger.Criticalf(msg)

		return fmt.Errorf("%w: %s", util.ErrProvisionError, msg)
	}

	err := b.provision(instance, platformType, sourceDisk, profile, profileObj)
	if err != nil {
		return err
	}

	b.Logger.Info("provisioning instance(s) completed successfully")

	err = b.Config.Dump(b.ConfigPath)
	if err != nil {
		b.Logger.Criticalf("error dumping boxen initial config to disk: %s", err)
		return err
	}

	b.Logger.Info("provision instance(s) completed successfully")

	return nil
}
