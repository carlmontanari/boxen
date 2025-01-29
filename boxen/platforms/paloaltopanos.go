package platforms

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/scrapli/scrapligo/driver/generic"
	sopoptions "github.com/scrapli/scrapligo/driver/opoptions"

	"github.com/carlmontanari/boxen/boxen/util"

	"github.com/carlmontanari/boxen/boxen/instance"
)

const (
	PaloAltoPanosScrapliPlatform = "paloalto_panos"

	paloAltoPanosDefaultBootTime   = 720
	paloAltoPanosDefaultPromptWait = 300
	paloAltoPanosDefaultLoginWait  = 300
)

type PaloAltoPanos struct {
	*instance.Qemu
	*ScrapliConsole
}

func (p *PaloAltoPanos) Package(
	sourceDir, packageDir string,
) (packageFiles, runFiles []string, err error) {
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

		return p.c.Channel.WriteReturn()
	}

	err = p.readUntil(
		[]byte("PA-VM login:"),
		getPlatformBootTimeout(PlatformTypePaloAltoPanos),
	)

	return err
}

func (p *PaloAltoPanos) initialConfigPrompt() error {
	vmLoginPrompt, _ := generic.NewCallback(
		func(d *generic.Driver, output string) error {
			return d.Channel.WriteReturn()
		},
		// needs to be re so it doesnt also match "pa-vm login" or obv could use "not contains"
		sopoptions.WithCallbackContainsRe(regexp.MustCompile(`^vm login:`)),
		sopoptions.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
	)

	hdfLoginPrompt, _ := generic.NewCallback(
		func(d *generic.Driver, output string) error {
			return d.Channel.WriteReturn()
		},
		sopoptions.WithCallbackContains("pa-hdf login"),
		sopoptions.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
	)

	loginPrompt, _ := generic.NewCallback(
		func(d *generic.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte("admin"), false)
		},
		sopoptions.WithCallbackContainsRe(regexp.MustCompile(`^pa-vm login:`)),
		sopoptions.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
	)

	passwordPrompt, _ := generic.NewCallback(
		func(d *generic.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte("admin"), false)
		},
		sopoptions.WithCallbackContains("assword:"),
		sopoptions.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
	)

	oldPasswordPrompt, _ := generic.NewCallback(
		func(d *generic.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte("admin"), false)
		},
		sopoptions.WithCallbackContains("enter old password :"),
		sopoptions.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
	)

	newPasswordPrompt, _ := generic.NewCallback(
		func(d *generic.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte(p.Credentials.Password), true)
		},
		sopoptions.WithCallbackContains("enter new password :"),
		sopoptions.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
	)

	confirmNewPasswordPrompt, _ := generic.NewCallback(
		func(d *generic.Driver, output string) error {
			return d.Channel.WriteAndReturn([]byte(p.Credentials.Password), true)
		},
		sopoptions.WithCallbackContains("confirm password   :"),
		sopoptions.WithCallbackNextTimeout(paloAltoPanosDefaultPromptWait*time.Second),
		sopoptions.WithCallbackComplete(),
	)

	callbacks := []*generic.Callback{
		vmLoginPrompt,
		hdfLoginPrompt,
		loginPrompt,
		passwordPrompt,
		oldPasswordPrompt,
		newPasswordPrompt,
		confirmNewPasswordPrompt,
	}

	_, err := p.c.SendWithCallbacks(
		"",
		callbacks,
		60*time.Second, // nolint:gomnd
	)

	return err
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

			err = p.defOnOpen(p.c)
			if err != nil {
				p.Loggers.Base.Criticalf("error running scrapligo on open: %s\n", err)

				c <- err
			}

			// this should move to scrapligo/scrapli-community, but to be quicker here it is...
			_, err = p.ScrapliConsole.c.SendCommand("set cli terminal width 500")
			if err != nil {
				p.Loggers.Base.Criticalf("error disabling terminal width on open: %s\n", err)

				c <- err
			}

			err = p.waitAutoCommit()
			if err != nil {
				p.Loggers.Base.Criticalf("error waiting for autocommit to complete: %s\n", err)

				c <- err
			}

			err = p.Config(
				util.ConfigLinesMd5Password(
					a.configLines,
					regexp.MustCompile(`(?i)(?:set mgt-config users .* phash )(.*$)`),
				),
			)
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

func (p *PaloAltoPanos) Start(opts ...instance.Option) error {
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
		p.Loggers.Base.Criticalf("error waiting for start ready state: %s", err)

		return err
	}

	if !a.prepareConsole {
		p.Loggers.Base.Info("prepare console not requested, starting instance complete")

		return nil
	}

	loginWait := util.ApplyTimeoutMultiplier(paloAltoPanosDefaultLoginWait)

	p.Loggers.Base.Debugf(
		"start ready prompt found, but sleeping %d seconds to give auth time to get ready",
		loginWait,
	)

	time.Sleep(time.Duration(loginWait) * time.Second)

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

	err := p.waitAutoCommit()
	if err != nil {
		p.Loggers.Base.Criticalf("error waiting for autocommit to complete: %s\n", err)

		return err
	}

	_, err = p.c.SendConfig(
		"commit",
		sopoptions.WithTimeoutOps(
			time.Duration(getPlatformSaveTimeout(PlatformTypePaloAltoPanos))*time.Second,
		),
	)

	return err
}

func (p *PaloAltoPanos) SetUserPass(usr, pwd string) error {
	p.Loggers.Base.Infof("set user/password for user '%s' requested", usr)

	lines := []string{
		fmt.Sprintf("set mgt-config users %s permissions role-based superuser yes", usr),
		fmt.Sprintf("set mgt-config users %s phash %s", usr, pwd),
	}

	lines = util.ConfigLinesMd5Password(
		lines,
		regexp.MustCompile(`(?i)(?:set mgt-config users .* phash )(.*$)`),
	)

	return p.Config(lines)
}

func (p *PaloAltoPanos) SetHostname(h string) error {
	p.Loggers.Base.Infof("set hostname '%s' requested", h)

	return p.Config([]string{fmt.Sprintf(
		"set deviceconfig system hostname %s",
		h)})
}
