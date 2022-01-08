package platforms

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/util"

	"github.com/scrapli/scrapligo/driver/base"
)

const (
	CiscoN9kvBiosName        = "OVMF.fd"
	CiscoN9kvScrapliPlatform = "cisco_nxos"
	CiscoN9kvDefaultUser     = "admin"

	ciscoN9kvDefaultBootTime    = 720
	ciscoN9kvDefaultPromptDelay = 300
	ciscoN9kvDefaultPromptWait  = 60
)

type CiscoN9kv struct {
	*instance.Qemu
	*ScrapliConsole
}

func (p *CiscoN9kv) Package(
	sourceDir, packageDir string,
) (packageFiles, runFiles []string, err error) {
	if !util.FileExists(fmt.Sprintf("%s/%s", sourceDir, CiscoN9kvBiosName)) {
		return nil, nil, fmt.Errorf(
			"%w: did not find bios file in dir '%s'",
			util.ErrInspectionError,
			sourceDir,
		)
	}

	err = util.CopyFile(
		fmt.Sprintf("%s/%s", sourceDir, CiscoN9kvBiosName),
		fmt.Sprintf("%s/%s", packageDir, CiscoN9kvBiosName),
	)

	return []string{CiscoN9kvBiosName}, []string{CiscoN9kvBiosName}, err
}

func (p *CiscoN9kv) patchCmdDisk(c *instance.QemuLaunchCmd) {
	c.Disk = []string{
		"-drive",
		fmt.Sprintf("if=none,file=%s,format=qcow2,id=drive-sata-disk0", p.Disk),
		"-device",
		"ahci,id=ahci0,bus=pci.0",
		"-device",
		"ide-hd,drive=drive-sata-disk0,bus=ahci0.0,id=drive-sata-disk0,bootindex=1",
	}
}

func (p *CiscoN9kv) patchCmdExtraBios(c *instance.QemuLaunchCmd) {
	diskDir := filepath.Dir(p.Disk)

	c.Extra = append(
		c.Extra,
		[]string{
			"-bios",
			fmt.Sprintf("%s/%s", diskDir, CiscoN9kvBiosName),
		}...,
	)
}

func (p *CiscoN9kv) patchCmdExtraBoot(c *instance.QemuLaunchCmd) {
	c.Extra = append(
		c.Extra,
		[]string{
			"-boot",
			"c",
		}...,
	)
}

func (p *CiscoN9kv) modifyStartCmd(c *instance.QemuLaunchCmd) {
	p.patchCmdDisk(c)
	p.patchCmdExtraBios(c)
	p.patchCmdExtraBoot(c)
}

func (p *CiscoN9kv) modifyInstallCmd(c *instance.QemuLaunchCmd) {
	p.modifyStartCmd(c)
}

func (p *CiscoN9kv) startReady(install bool) error {
	err := p.openRetry()
	if err != nil {
		return err
	}

	if install {
		err = p.readUntil(
			[]byte("starting auto provisioning ..."),
			getPlatformBootTimeout(PlatformTypeCiscoN9kv),
		)
	} else {
		err = p.readUntil(
			[]byte("login:"),
			getPlatformBootTimeout(PlatformTypeCiscoN9kv),
		)
	}

	return err
}

func (p *CiscoN9kv) initialConfigPrompt() error {
	disablePOAP, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte("yes"), false)
		},
		base.WithCallbackContains("(yes/skip/no)[no]:"),
		base.WithCallbackNextTimeout(ciscoN9kvDefaultPromptWait*time.Second),
	)

	disableSecurePassword, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte("no"), false)
		},
		base.WithCallbackContains("do you want to enforce secure password standard"),
		base.WithCallbackNextTimeout(ciscoN9kvDefaultPromptWait*time.Second),
	)

	enterAdminPass, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte(p.Credentials.Password), true)
		},
		base.WithCallbackContains("enter the password for \"admin\""),
		base.WithCallbackNextTimeout(ciscoN9kvDefaultPromptWait*time.Second),
	)

	confirmAdminPass, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte(p.Credentials.Password), true)
		},
		base.WithCallbackContains("confirm the password for \"admin\""),
		base.WithCallbackNextTimeout(ciscoN9kvDefaultPromptWait*time.Second),
	)

	initialConfigDialog, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte("no"), false)
		},
		base.WithCallbackContains(
			"would you like to enter the basic configuration dialog (yes/no):",
		),
		base.WithCallbackComplete(true),
	)

	callbacks := []*base.ReadCallback{
		disablePOAP,
		disableSecurePassword,
		enterAdminPass,
		confirmAdminPass,
		initialConfigDialog,
	}

	return p.c.ReadWithCallbacks(
		callbacks,
		"",
		ciscoN9kvDefaultPromptDelay*time.Second,
		1*time.Second,
	)
}

func (p *CiscoN9kv) setBootVar() error {
	dirBootflashResult, err := p.c.SendCommand("dir bootflash: | i nxos")
	if err != nil {
		return err
	}

	bootImagePattern := regexp.MustCompile(`nxos[.0-9]+bin`)
	bootVarImage := bootImagePattern.FindString(dirBootflashResult.Result)

	bootVarConfig := "boot nxos bootflash:///"

	if bootVarImage != "" {
		bootVarConfig = fmt.Sprintf("boot nxos bootflash:%s", bootVarImage)
	}

	// setting boot config does a verification thing too; it takes an eternity!
	_, err = p.c.SendConfig(
		bootVarConfig,
		base.WithSendTimeoutOps(
			time.Duration(getPlatformSaveTimeout(PlatformTypeCiscoN9kv))*2*time.Second,
		),
	)

	return err
}

func (p *CiscoN9kv) Install(opts ...instance.Option) error { //nolint: funlen
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

			return
		}

		p.Loggers.Base.Debug("start ready state acquired, handling initial config dialog")

		err = p.initialConfigPrompt()
		if err != nil {
			p.Loggers.Base.Criticalf("error running through initial config dialog: %s\n", err)

			c <- err

			return
		}

		p.Loggers.Base.Debug("initial config dialog addressed, logging in")

		err = p.login(
			&loginArgs{
				username: CiscoN9kvDefaultUser,
				// user has not been created yet, just set the admin password to the one associated
				// with the instance, so login as admin w/ that password for now!
				password: p.Credentials.Password,
			},
		)
		if err != nil {
			c <- err

			return
		}

		p.Loggers.Base.Debug("log in complete")

		if a.configLines != nil {
			p.Loggers.Base.Debug("install config lines provided, executing scrapligo on open")

			err = p.defOnOpen(p.c)
			if err != nil {
				p.Loggers.Base.Criticalf("error running scrapligo on open: %s\n", err)

				c <- err

				return
			}

			err = p.Config(a.configLines)
			if err != nil {
				p.Loggers.Base.Criticalf("error sending install config lines: %s\n", err)

				c <- err

				return
			}
		}

		p.Loggers.Base.Debug("initial installation complete, setting boot var before saving")

		err = p.setBootVar()
		if err != nil {
			p.Loggers.Base.Criticalf("error setting boot var: %s\n", err)

			c <- err

			return
		}

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

func (p *CiscoN9kv) Start(opts ...instance.Option) error { //nolint:dupl
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

func (p *CiscoN9kv) SaveConfig() error {
	p.Loggers.Base.Info("save config requested")

	_, err := p.c.SendCommand(
		"copy running-config startup-config",
		base.WithSendTimeoutOps(
			time.Duration(getPlatformSaveTimeout(PlatformTypeCiscoN9kv))*time.Second,
		),
	)

	return err
}

func (p *CiscoN9kv) SetUserPass(usr, pwd string) error {
	p.Loggers.Base.Infof("set user/password for user '%s' requested", usr)

	return p.Config([]string{fmt.Sprintf(
		"username %s password 0 %s role network-admin",
		usr,
		pwd)})
}

func (p *CiscoN9kv) SetHostname(h string) error {
	p.Loggers.Base.Infof("set hostname '%s' requested", h)

	return p.Config([]string{fmt.Sprintf(
		"hostname %s",
		h)})
}
