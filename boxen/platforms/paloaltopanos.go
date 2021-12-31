package platforms

import (
	"fmt"
	"strings"
	"time"

	"github.com/carlmontanari/boxen/boxen/instance"

	"github.com/scrapli/scrapligo/channel"

	"github.com/scrapli/scrapligo/driver/base"
)

const (
	PaloAltoPanosScrapliPlatform = "paloalto_panos"

	paloAltoPanosDefaultBootTime   = 720
	paloAltoPanosDefaultPromptWait = 30
)

type PaloAltoPanos struct {
	*instance.Qemu
	*ScrapliConsole
}

func (p *PaloAltoPanos) Package(
	sourceDir, packageDir string,
) (packageFiles, installFiles []string, err error) {
	_, _ = sourceDir, packageDir
	return []string{}, []string{}, err
}

func (p *PaloAltoPanos) patchCPUEmulation(c *instance.QemuLaunchCmd) {
	if len(c.Accel) > 1 && c.Accel[1] != AccelKVM {
		// if the accel is *not* KVM, we'll use emulated CPU, otherwise we'll leave it
		// to the default of "max"
		c.CPU[1] = "qemu64,+ssse3,+sse4.1,+sse4.2"
	}
}

func (p *PaloAltoPanos) modifyStartCmd(c *instance.QemuLaunchCmd) {
	p.patchCPUEmulation(c)
}

func (p *PaloAltoPanos) modifyInstallCmd(c *instance.QemuLaunchCmd) {
	p.modifyStartCmd(c)
}

func (p *PaloAltoPanos) startReady(install bool) error {
	err := p.openRetry()
	if err != nil {
		return err
	}

	if install {
		err = p.readUntil(
			[]byte("vm login:"),
			getPlatformBootTimeout(PlatformPaloAltoPanos),
		)

		if err != nil {
			return err
		}

		return p.c.Channel.SendReturn()
	}

	err = p.readUntil(
		[]byte("PA-VM login:"),
		getPlatformBootTimeout(PlatformTypePaloAltoPanos),
	)

	return err
}

func (p *PaloAltoPanos) initialConfigPrompt() error {
	vmLoginPrompt, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.SendReturn()
		},
		// needs to be re so it doesnt also match "pa-vm login" or obv could use "not contains"
		base.WithCallbackContainsRe(`^vm login:`),
		base.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
	)

	hdfLoginPrompt, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.SendReturn()
		},
		base.WithCallbackContains("pa-hdf login"),
		base.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
	)

	loginPrompt, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte("admin"), false)
		},
		base.WithCallbackContainsRe(`^pa-vm login:`),
		base.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
	)

	passwordPrompt, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte("admin"), false)
		},
		base.WithCallbackContains("assword:"),
		base.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
	)

	oldPasswordPrompt, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte("admin"), false)
		},
		base.WithCallbackContains("enter old password :"),
		base.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
	)

	newPasswordPrompt, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte(p.Credentials.Password), true)
		},
		base.WithCallbackContains("enter new password :"),
		base.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
	)

	confirmNewPasswordPrompt, _ := base.NewReadCallback(
		func(d *base.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte(p.Credentials.Password), true)
		},
		base.WithCallbackContains("confirm password   :"),
		base.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
		base.WithCallbackComplete(true),
	)

	callbacks := []*base.ReadCallback{
		vmLoginPrompt,
		hdfLoginPrompt,
		loginPrompt,
		passwordPrompt,
		oldPasswordPrompt,
		newPasswordPrompt,
		confirmNewPasswordPrompt,
	}

	return p.c.ReadWithCallbacks(
		callbacks,
		"",
		60*time.Second, // nolint:gomnd
		1*time.Second,
	)
}

func (p *PaloAltoPanos) Install(opts ...instance.Option) error { //nolint:funlen
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

		if a.configLines != nil {
			p.Loggers.Base.Debug("install config lines provided, executing scrapligo on open")

			// seems like the pan vm wants to flake out a bit and paint the prompt a few times in
			// weird timing immediately after boot... fetching the prompt seems to
			// kinda "clear" things out
			_, _ = p.c.GetPrompt()

			err = p.defOnOpen(p.c)
			if err != nil {
				p.Loggers.Base.Criticalf("error running scrapligo on open: %s\n", err)

				c <- err
			}

			err = p.waitAutoCommit()
			if err != nil {
				p.Loggers.Base.Criticalf("error waiting for autocommit to complete: %s\n", err)

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

func (p *PaloAltoPanos) Start(opts ...instance.Option) error { //nolint:dupl
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

func (p *PaloAltoPanos) waitAutoCommit() error {
	for {
		time.Sleep(paloAltoPanosDefaultPromptWait * time.Second)

		r, err := p.c.SendCommand("show jobs processed")
		if err != nil {
			return err
		}

		if strings.Contains(r.Result, "FIN") {
			p.Loggers.Base.Debug("no commits pending")

			return nil
		}

		p.Loggers.Base.Debug("commit still pending, sleeping...")
	}
}

func (p *PaloAltoPanos) SaveConfig() error {
	p.Loggers.Base.Info("save config requested")

	_, err := p.c.SendConfig(
		"commit",
		base.WithSendTimeoutOps(
			time.Duration(getPlatformSaveTimeout(PlatformTypePaloAltoPanos))*time.Second,
		),
	)

	return err
}

func (p *PaloAltoPanos) SetUserPass(usr, pwd string) error {
	p.Loggers.Base.Infof("set user/password for user '%s' requested", usr)

	_, err := p.c.Driver.SendInteractive(
		[]*channel.SendInteractiveEvent{
			{
				ChannelInput: fmt.Sprintf(
					"set mgt-config users %s password",
					usr,
				),
				ChannelResponse: "Enter password   :",
				HideInput:       false,
			},
			{
				ChannelInput:    pwd,
				ChannelResponse: "Confirm password :",
				HideInput:       true,
			},
			{
				ChannelInput:    pwd,
				ChannelResponse: "#",
				HideInput:       true,
			},
		},
		base.WithDesiredPrivilegeLevel("configuration"),
	)
	if err != nil {
		return err
	}

	return p.Config([]string{
		fmt.Sprintf("set mgmt-config users %s permissions role-based superuser yes", usr)})
}

func (p *PaloAltoPanos) SetHostname(h string) error {
	p.Loggers.Base.Infof("set hostname '%s' requested", h)

	return p.Config([]string{fmt.Sprintf(
		"set deviceconfig system hostname %s",
		h)})
}
