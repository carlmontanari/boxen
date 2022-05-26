package platforms

import (
	"fmt"
	"strings"
	"time"

	"github.com/carlmontanari/boxen/boxen/instance"

	"github.com/scrapli/scrapligo/driver/base"
)

const (
	CiscoXrv9kScrapliPlatform = "cisco_iosxr"

	ciscoXrv9kDefaultBootTime    = 720
	ciscoXrv9kDefaultPromptDelay = 180
	ciscoXrv9kDefaultPromptWait  = 60
)

type CiscoXrv9k struct {
	*instance.Qemu
	*ScrapliConsole
}

func (p *CiscoXrv9k) Package(
	sourceDir, packageDir string,
) (packageFiles, runFiles []string, err error) {
	_, _ = sourceDir, packageDir
	return []string{}, []string{}, err
}

func (p *CiscoXrv9k) patchCmdMgmtNic(c *instance.QemuLaunchCmd) {
	ctrlNic := []string{
		"-device",
		fmt.Sprintf("%s,netdev=ctrl", NicVirtio),
		"-netdev",
		"tap,id=ctrl,script=no,downscript=no",
	}
	devNic := []string{
		"-device",
		fmt.Sprintf("%s,netdev=dev", NicVirtio),
		"-netdev",
		"tap,id=dev,script=no,downscript=no",
	}

	c.MgmtNic = append(c.MgmtNic, ctrlNic...)
	c.MgmtNic = append(c.MgmtNic, devNic...)
}

func (p *CiscoXrv9k) modifyStartCmd(c *instance.QemuLaunchCmd) {
	p.patchCmdMgmtNic(c)
}

func (p *CiscoXrv9k) modifyInstallCmd(c *instance.QemuLaunchCmd) {
	p.modifyStartCmd(c)
}

func (p *CiscoXrv9k) startReady() error {
	err := p.openRetry()
	if err != nil {
		return err
	}

	err = p.readUntil(
		[]byte("Press RETURN to get started"),
		getPlatformBootTimeout(PlatformTypeCiscoXrv9k),
	)
	if err != nil {
		return err
	}

	return p.c.Channel.SendReturn()
}

func (p *CiscoXrv9k) initialConfigPrompt() error {
	rootUser, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte(p.Credentials.Username), false)
		},
		base.WithCallbackContains("enter root-system username"),
		base.WithCallbackNextTimeout(ciscoXrv9kDefaultPromptWait*time.Second),
	)

	enterAdminPass, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte(p.Credentials.Password), true)
		},
		// don't forget the colon at the end or this will match for the secret again step too!
		// could also have a "not contains" but this works well enough.
		base.WithCallbackContains("enter secret:"),
		base.WithCallbackNextTimeout(ciscoXrv9kDefaultPromptWait*time.Second),
	)

	confirmAdminPass, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte(p.Credentials.Password), true)
		},
		base.WithCallbackContains("enter secret again"),
		base.WithCallbackNextTimeout(ciscoXrv9kDefaultPromptWait*time.Second),
		base.WithCallbackComplete(true),
	)

	callbacks := []*base.ReadCallback{
		rootUser,
		enterAdminPass,
		confirmAdminPass,
	}

	return p.c.ReadWithCallbacks(
		callbacks,
		"",
		ciscoXrv9kDefaultPromptDelay*time.Second,
		1*time.Second,
	)
}

func (p *CiscoXrv9k) generateCryptoKey() error {
	confirmReplace, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte("yes"), false)
		},
		base.WithCallbackContains("really want to replace them"),
	)

	enterBits, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.SendReturn()
		},
		base.WithCallbackContains("many bits in the modulus"),
		base.WithCallbackComplete(true),
	)

	callbacks := []*base.ReadCallback{
		confirmReplace,
		enterBits,
	}

	return p.c.ReadWithCallbacks(
		callbacks,
		"crypto key generate rsa",
		ciscoXrv9kDefaultPromptWait*time.Second,
		1*time.Second,
	)
}

func (p *CiscoXrv9k) Install(opts ...instance.Option) error { //nolint:funlen
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

		err = p.startReady()
		if err != nil {
			p.Loggers.Base.Criticalf("error waiting for start ready state: %s\n", err)

			c <- err
		}

		p.Loggers.Base.Debug("start ready state acquired, handling initial config dialog")

		err = p.initialConfigPrompt()
		if err != nil {
			p.Loggers.Base.Criticalf("error running through initial config dialog: %s\n", err)

			c <- err
		}

		p.Loggers.Base.Debug("initial config dialog addressed, logging in")

		err = p.login(
			&loginArgs{
				username: p.Credentials.Username,
				password: p.Credentials.Password,
			},
		)
		if err != nil {
			c <- err
		}

		p.Loggers.Base.Debug("log in complete")

		err = p.generateCryptoKey()
		if err != nil {
			c <- err
		}

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

func (p *CiscoXrv9k) Start(opts ...instance.Option) error {
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

	err = p.startReady()
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

	p.Loggers.Base.Info(
		"on open complete, waiting until we see 'system configuration completed' message",
	)

	err = p.readUntil(
		[]byte("SYSTEM CONFIGURATION COMPLETED"),
		getPlatformBootTimeout(PlatformTypeCiscoXrv9k),
	)
	if err != nil {
		return err
	}

	p.Loggers.Base.Info("starting platform instance complete")

	return nil
}

func (p *CiscoXrv9k) SaveConfig() error {
	p.Loggers.Base.Info("save config requested")

	r, err := p.c.SendConfig(
		"commit",
		base.WithSendTimeoutOps(
			time.Duration(getPlatformSaveTimeout(PlatformTypeCiscoXrv9k))*time.Second,
		),
	)

	if strings.Contains(r.Result, "Failed to commit") {
		p.Loggers.Base.Info(
			"'failed to commit' seen in save config output, sleeping and trying again....",
		)

		time.Sleep(ciscoXrv9kDefaultPromptWait * time.Second)

		return p.SaveConfig()
	}

	return err
}

func (p *CiscoXrv9k) SetUserPass(usr, pwd string) error {
	p.Loggers.Base.Infof("set user/password for user '%s' requested", usr)

	return p.Config([]string{fmt.Sprintf(
		"username %s password 0 %s",
		usr,
		pwd)})
}

func (p *CiscoXrv9k) SetHostname(h string) error {
	p.Loggers.Base.Infof("set hostname '%s' requested", h)

	return p.Config([]string{fmt.Sprintf(
		"hostname %s",
		h)})
}
