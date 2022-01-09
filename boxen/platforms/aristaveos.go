package platforms

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"time"

	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/util"

	"github.com/scrapli/scrapligo/driver/base"
)

const (
	AristaVeosAbootFileName   = "Aboot-veos-serial-8.0.0.iso"
	AristaVeosScrapliPlatform = "arista_eos"
	AristaVeosDefaultUser     = "admin"
	AristaVeosDefaultPass     = "admin"
)

type AristaVeos struct {
	*instance.Qemu
	*ScrapliConsole
}

func (p *AristaVeos) Package(
	sourceDir, packageDir string,
) (packageFiles, runFiles []string, err error) {
	if !util.FileExists(fmt.Sprintf("%s/%s", sourceDir, AristaVeosAbootFileName)) {
		return nil, nil, fmt.Errorf(
			"%w: did not find Aboot iso in dir '%s'",
			util.ErrInspectionError,
			sourceDir,
		)
	}

	err = util.CopyFile(
		fmt.Sprintf("%s/%s", sourceDir, AristaVeosAbootFileName),
		fmt.Sprintf("%s/%s", packageDir, AristaVeosAbootFileName),
	)

	return []string{AristaVeosAbootFileName}, []string{}, err
}

func (p *AristaVeos) patchCmdMgmtNic(c *instance.QemuLaunchCmd) {
	// vEOS wants the mgmt port to be the first port on the bus, so make that happen...
	c.MgmtNic[1] += fmt.Sprintf(
		",bus=pci.1,addr=0x%x",
		2, //nolint:gomnd
	)
}

func (p *AristaVeos) patchCmdDataNic(c *instance.QemuLaunchCmd) {
	var nicCmd []string

	for nicID := 1; nicID < p.Hardware.NicCount+1; nicID++ {
		// need to offset pci bus addr by one to account for mgmt nic
		busID := int(math.Floor(float64(nicID+1)/float64(p.Hardware.NicPerBus))) + 1
		busAddr := (nicID + 1%p.Hardware.NicPerBus) + 1
		paddedNicID := fmt.Sprintf("%03d", nicID)

		nicCmd = append(
			nicCmd,
			p.BuildDataNic(nicID, busID, busAddr, paddedNicID)...)
	}

	c.DataNic = nicCmd
}

func (p *AristaVeos) patchCmdCdrom(c *instance.QemuLaunchCmd) {
	diskDir := filepath.Dir(p.Disk)
	c.Extra = append(
		c.Extra,
		[]string{"-cdrom", fmt.Sprintf("%s/%s", diskDir, AristaVeosAbootFileName)}...)
}

func (p *AristaVeos) modifyStartCmd(c *instance.QemuLaunchCmd) {
	p.patchCmdMgmtNic(c)
	p.patchCmdDataNic(c)
}

func (p *AristaVeos) modifyInstallCmd(c *instance.QemuLaunchCmd) {
	p.modifyStartCmd(c)
	p.patchCmdCdrom(c)
}

func (p *AristaVeos) startReady(install bool) error {
	err := p.openRetry()
	if err != nil {
		return err
	}

	if install {
		err = p.readUntil(
			// readUntil makes everything lower so we dont actually care about the case, but w/e
			[]byte("localhost ZeroTouch:"),
			getPlatformBootTimeout(PlatformTypeAristaVeos),
		)
		if err != nil {
			return err
		}

		err = p.login(
			&loginArgs{
				username:     AristaVeosDefaultUser,
				password:     AristaVeosDefaultPass,
				loginPattern: regexp.MustCompile(`(?i)localhost zerotouch:\s?`),
			},
		)
		if err != nil {
			return err
		}

		// device auto reloads after this
		p.Loggers.Base.Debug("disabling zerotouch, device will reload")

		_ = p.c.Channel.WriteAndReturn([]byte("zerotouch disable"), false)
	}

	err = p.readUntil(
		[]byte("login:"),
		getPlatformBootTimeout(PlatformTypeAristaVeos),
	)

	return err
}

func (p *AristaVeos) Install(opts ...instance.Option) error {
	p.Loggers.Base.Info("install requested")

	a, opts, err := setInstallArgs(opts...)
	if err != nil {
		return err
	}

	opts = append(opts, instance.WithLaunchModifier(p.modifyInstallCmd))

	c := make(chan error, 1)
	stop := make(chan bool, 1)

	go func() {
		err = p.Qemu.Start(opts...)
		if err != nil {
			c <- err
		}

		p.Loggers.Base.Debug("instance started, waiting for start ready state")

		err = p.startReady(true)
		if err != nil {
			p.Loggers.Base.Criticalf("error waiting for start ready state: %s\n", err)

			c <- err
		}

		p.Loggers.Base.Debug("start ready state acquired, logging in")

		err = p.login(
			&loginArgs{
				username: AristaVeosDefaultUser,
				password: AristaVeosDefaultPass,
			},
		)
		if err != nil {
			c <- err
		}

		p.Loggers.Base.Debug("log in complete")

		if a.configLines != nil {
			p.Loggers.Base.Debug("install config lines provided, executing scrapligo on open")

			err = p.defOnOpen(p.c)
			if err != nil {
				p.Loggers.Base.Criticalf("error running scrapligo on open: %s\n", err)

				c <- err
			}

			err = p.Config(a.configLines)
			if err != nil {
				p.Loggers.Base.Criticalf("error sending install config lines: %s\n", err)

				c <- err
			}
		}

		p.Loggers.Base.Debug("initial installation complete")

		err = p.SaveConfig()
		if err != nil {
			p.Loggers.Base.Criticalf("error saving config: %s\n", err)

			c <- err
		}

		// small delay ensuring config is saved nicely, without this extra sleep things just seem to
		// not actually "save" despite the "save complete" or whatever output.
		time.Sleep(5 * time.Second) // nolint:gomnd

		c <- nil
		stop <- true
	}()

	go p.WatchMainProc(c, stop)

	err = <-c
	if err != nil {
		return err
	}

	p.Loggers.Base.Info("install complete, stopping instance")

	return p.Stop(opts...)
}

func (p *AristaVeos) Start(opts ...instance.Option) error { //nolint:dupl
	p.Loggers.Base.Info("start platform instance requested")

	a, opts, err := setStartArgs(opts...)
	if err != nil {
		return err
	}

	opts = append(opts, instance.WithLaunchModifier(p.modifyStartCmd))

	err = p.Qemu.Start(opts...)
	if err != nil {
		return err
	}

	err = p.startReady(false)
	if err != nil {
		p.Loggers.Base.Criticalf("error waiting for start ready state: %s\n", err)

		return err
	}

	if !a.prepareConsole {
		p.Loggers.Base.Info("prepare console not requested, starting instance complete")

		return nil
	}

	err = p.login(
		&loginArgs{
			username: p.Credentials.Username,
			password: p.Credentials.Password,
		},
	)
	if err != nil {
		return err
	}

	err = p.defOnOpen(p.c)
	if err != nil {
		return err
	}

	p.Loggers.Base.Info("starting platform instance complete")

	return nil
}

func (p *AristaVeos) SaveConfig() error {
	p.Loggers.Base.Info("save config requested")

	_, err := p.c.SendCommand(
		"copy running-config startup-config",
		base.WithSendTimeoutOps(
			time.Duration(getPlatformSaveTimeout(PlatformTypeAristaVeos))*time.Second,
		),
	)

	return err
}

func (p *AristaVeos) SetUserPass(usr, pwd string) error {
	p.Loggers.Base.Infof("set user/password for user '%s' requested", usr)

	return p.Config([]string{fmt.Sprintf(
		"username %s secret 0 %s role network-admin",
		usr,
		pwd)})
}

func (p *AristaVeos) SetHostname(h string) error {
	p.Loggers.Base.Infof("set hostname '%s' requested", h)

	return p.Config([]string{fmt.Sprintf(
		"hostname %s",
		h)})
}
