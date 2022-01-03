package instance

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/carlmontanari/boxen/boxen/command"
	"github.com/carlmontanari/boxen/boxen/config"
	"github.com/carlmontanari/boxen/boxen/util"
)

// Qemu represents a qemu virtual machine process and the associated configuration information.
type Qemu struct {
	Name string
	ID   int
	PID  int

	Qemu *config.Qemu

	Proc *exec.Cmd

	Credentials *config.Credentials

	Disk          string
	Hardware      *config.Hardware
	Advanced      *config.Advanced
	MgmtIntf      *config.MgmtIntf
	DataPlaneIntf *config.DataPlaneIntf

	LaunchCmd *QemuLaunchCmd

	Loggers *Loggers
}

// NewQemu returns a new "blank" qemu instance based on the provided instance name and boxen Config
// object.
func NewQemu(n string, c *config.Config, l *Loggers) (*Qemu, error) {
	i := &Qemu{
		Name:          n,
		ID:            c.Instances[n].ID,
		PID:           c.Instances[n].PID,
		Qemu:          c.Options.Qemu,
		Credentials:   c.Instances[n].Credentials,
		Disk:          c.Instances[n].Disk,
		Hardware:      c.Instances[n].Hardware,
		Advanced:      c.Instances[n].Advanced,
		MgmtIntf:      c.Instances[n].MgmtIntf,
		DataPlaneIntf: c.Instances[n].DataPlaneIntf,
		Loggers:       l,
	}

	if i.PID > 0 && !i.validatePid() {
		// failed to validate a stored pid; we'll assume it's not running
		i.PID = -1
	}

	if i.Credentials == nil && c.Options.Credentials != nil {
		i.Credentials = c.Options.Credentials
	}

	if c.Instances[n].Profile == "" {
		return i, nil
	}

	profile, ok := c.Platforms[c.Instances[n].PlatformType].Profiles[c.Instances[n].Profile]
	if !ok {
		panic("unknown profile for instance, cannot continue")
	}

	mergeProfileHardware(profile.Hardware, i)

	return i, nil
}

func mergeProfileHardware(
	hw *config.ProfileHardware,
	i *Qemu,
) {
	if i.Hardware.Memory == 0 && hw.Memory > 0 {
		i.Hardware.Memory = hw.Memory
	}

	if i.Hardware.Acceleration == nil && hw.Acceleration != nil {
		i.Hardware.Acceleration = hw.Acceleration
	}

	if i.Hardware.NicType == "" && hw.NicType != "" {
		i.Hardware.NicType = hw.NicType
	}

	if i.Hardware.NicCount == 0 && hw.NicCount > 0 {
		i.Hardware.NicCount = hw.NicCount
	}

	if i.Hardware.NicPerBus == 0 && hw.NicPerBus > 0 {
		i.Hardware.NicPerBus = hw.NicPerBus
	}
}

func (i *Qemu) buildLaunchCmd() {
	if i.LaunchCmd != nil {
		i.LaunchCmd = nil
	}

	i.LaunchCmd = &QemuLaunchCmd{
		Name:    []string{"-name", i.Name},
		UUID:    i.launchCmdUUID(),
		Accel:   i.launchCmdAccel(),
		Display: i.launchCmdDisplay(),
		Machine: i.launchCmdMachine(),
		Memory:  i.launchCmdMemory(),
		CPU:     i.launchCmdCPU(),
		Monitor: i.launchCmdMonitor(),
		Serial:  i.launchCmdSerial(),
		Disk:    i.launchCmdDisk(),
		Pci:     i.launchCmdPci(),
		MgmtNic: i.launchCmdMgmtNic(),
		DataNic: i.launchCmdDataNic(),
	}
}

// Install "installs" a qemu disk -- meaning it runs through any "initial config" type processes for
// a virtual machine. For the most part the Qemu struct doesn't do much here -- it simply runs
// Start, however the objects that embed this struct likely add additional tasks and actions in
// order to properly "install" the disk.
func (i *Qemu) Install(opts ...Option) error {
	return i.Start(opts...)
}

// Start starts the qemu instance.
func (i *Qemu) Start(opts ...Option) error {
	i.Loggers.Base.Info("qemu instance start requested")

	if i.PID > 0 {
		msg := "cannot start, stored pid > 0, assuming instance is already running"

		i.Loggers.Base.Critical(msg)

		return fmt.Errorf("%w: %s", util.ErrInstanceError, msg)
	}

	diskExists := util.FileExists(i.Disk)

	if !diskExists {
		i.Loggers.Base.Critical("cannot start, source disk does not exist")

		panic(
			"disk doesnt exist, in the future, depending on operation mode we will " +
				"copy from the base disks here",
		)
	}

	i.buildLaunchCmd()

	qOpts := &qemuOpts{}

	for _, option := range opts {
		err := option(qOpts)

		if err != nil {
			if errors.Is(err, util.ErrIgnoredOption) {
				continue
			} else {
				return err
			}
		}
	}

	if qOpts.launchModifier != nil {
		qOpts.launchModifier(i.LaunchCmd)
	}

	launchCmd, err := i.LaunchCmd.Render()
	if err != nil {
		i.Loggers.Base.Critical("failure rendering launch command, cannot continue")

		return err
	}

	i.Loggers.Base.Debugf("launching instance with command: %s", launchCmd)

	executeArgs := []command.ExecuteOption{command.WithArgs(launchCmd), command.WithSudo(true)}

	if i.Loggers.Stdout != nil {
		i.Loggers.Base.Debug("stdout logger provided, setting execute argument")

		executeArgs = append(executeArgs, command.WithStdOut(i.Loggers.Stdout))
	}

	if i.Loggers.Stderr != nil {
		i.Loggers.Base.Debug("stderr logger provided, setting execute argument")

		executeArgs = append(executeArgs, command.WithStdOut(i.Loggers.Stderr))
	}

	r, err := command.Execute(
		i.Qemu.Binary,
		executeArgs...,
	)
	if err != nil {
		i.Loggers.Base.Criticalf("error executing launch command: %s\n", err)

		return err
	}

	err = r.CheckStdErr(command.WithIgnore([][]byte{[]byte("CPUID")}))
	if err != nil {
		i.Loggers.Base.Critical("unknown stderr on instance launch, cannot continue")
		return err
	}

	i.Proc = r.Proc
	i.PID = i.Proc.Process.Pid

	i.Loggers.Base.Info("qemu instance start complete")

	return nil
}

func (i *Qemu) validatePid() bool {
	r, err := command.Execute(
		"ps",
		command.WithArgs([]string{"-axf", strconv.Itoa(i.PID)}),
		command.WithWait(true),
	)
	if err != nil {
		return false
	}

	stdoutOutput, _ := r.ReadStdout()

	return bytes.Contains(stdoutOutput, []byte(i.Name))
}

// Stop stops the qemu virtual machine.
func (i *Qemu) Stop(opts ...Option) error {
	i.Loggers.Base.Info("qemu instance stop requested")

	if i.PID < 1 {
		msg := "cannot stop, stored pid < 1, assuming instance is already stopped"
		i.Loggers.Base.Critical(msg)

		return fmt.Errorf("%w: %s", util.ErrInstanceError, msg)
	}

	// if the boxen process has stayed running (like during installation) in a perfect world we
	// could just simply call Kill() on the process, *but* because sudo we launch child procs and
	// Kill() does *not* handle killing those. So... we'll just always kill things in the way we
	// would if we were working with a stored pid.

	if !i.validatePid() {
		msg := "cannot stop, failed validating stored pid"

		i.Loggers.Base.Critical(msg)

		return fmt.Errorf("%w: %s", util.ErrInstanceError, msg)
	}

	qOpts := &qemuOpts{}

	for _, option := range opts {
		err := option(qOpts)

		if err != nil {
			if errors.Is(err, util.ErrIgnoredOption) {
				continue
			} else {
				return err
			}
		}
	}

	_, err := command.Execute(
		"kill",
		command.WithArgs([]string{strconv.Itoa(i.PID)}),
		command.WithWait(true),
		command.WithSudo(qOpts.sudo),
	)
	if err != nil {
		i.Loggers.Base.Criticalf("error executing kill command: %s\n", err)

		return err
	}

	i.PID = 0
	i.Proc = nil

	// eventually delete instance dir if instance mode is not persist
	i.Loggers.Base.Info("qemu instance stop complete")

	return nil
}

// GetPid returns the process ID of the virtual machine.
func (i *Qemu) GetPid() int {
	return i.PID
}
