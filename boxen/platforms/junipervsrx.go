package platforms

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/util"

	"github.com/scrapli/scrapligo/channel"

	"github.com/scrapli/scrapligo/driver/base"
)

const (
	JuniperVsrxScrapliPlatform = "juniper_junos"
)

type JuniperVsrx struct {
	*instance.Qemu
	*ScrapliConsole
}

func (p *JuniperVsrx) Package(
	sourceDir, packageDir string,
) (packageFiles, runFiles []string, err error) {
	_, _ = sourceDir, packageDir
	return []string{}, []string{}, err
}

func (p *JuniperVsrx) modifyStartCmd(c *instance.QemuLaunchCmd) {
	_ = c
}

func (p *JuniperVsrx) modifyInstallCmd(c *instance.QemuLaunchCmd) {
	_ = c
}

func (p *JuniperVsrx) startReady(install bool) error {
	err := p.openRetry()
	if err != nil {
		return err
	}

	err = p.readUntil(
		[]byte("login:"),
		getPlatformBootTimeout(PlatformJuniperVsrx),
	)
	if err != nil || !install {
		return err
	}

	err = p.c.Channel.WriteAndReturn([]byte("root"), false)
	if err != nil {
		return err
	}

	err = p.readUntil(
		// this read takes a while! (sometimes ~180s)
		[]byte("root@%"),
		180, //nolint:gomnd
	)
	if err != nil {
		return err
	}

	err = p.c.Channel.WriteAndReturn([]byte("cli"), false)
	if err != nil {
		return err
	}

	return err
}

func (p *JuniperVsrx) Install(opts ...instance.Option) error {
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
				username: p.c.Channel.CommsReturnChar,
				password: p.c.Channel.CommsReturnChar,
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

			err = p.Config(p.configLinesEncryptPasswords(a.configLines))
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

func (p *JuniperVsrx) Start(opts ...instance.Option) error { //nolint:dupl
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

func (p *JuniperVsrx) configLinesEncryptPasswords(lines []string) []string {
	pattern := regexp.MustCompile(`(?i)(?:set system .* encrypted-password )(.*$)`)

	for i, line := range lines {
		matches := pattern.FindStringSubmatch(line)

		if len(matches) > 1 {
			newLine := strings.Replace(
				line,
				matches[1],
				string(util.Md5Crypt([]byte(matches[1]))),
				1,
			)

			lines[i] = newLine
		}
	}

	return lines
}

func (p *JuniperVsrx) SaveConfig() error {
	p.Loggers.Base.Info("save config requested")

	_, err := p.c.SendConfig(
		"commit",
		base.WithSendTimeoutOps(
			time.Duration(getPlatformSaveTimeout(PlatformJuniperVsrx))*time.Second,
		),
	)

	return err
}

func (p *JuniperVsrx) SetUserPass(usr, pwd string) error {
	p.Loggers.Base.Infof("set user/password for user '%s' requested", usr)

	_, err := p.c.Driver.SendInteractive(
		[]*channel.SendInteractiveEvent{
			{
				ChannelInput: fmt.Sprintf(
					"set system login user %s class super-user authentication plain-text-password",
					usr,
				),
				ChannelResponse: "New password:",
				HideInput:       false,
			},
			{
				ChannelInput:    pwd,
				ChannelResponse: "Retype new password:",
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

	return err
}

func (p *JuniperVsrx) SetHostname(h string) error {
	p.Loggers.Base.Infof("set hostname '%s' requested", h)

	return p.Config([]string{fmt.Sprintf(
		"set system host-name %s",
		h)})
}
