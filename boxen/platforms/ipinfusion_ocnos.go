package platforms

import (
	"fmt"
	"time"

	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/scrapli/scrapligo/driver/base"
)

const (
	IPInfusionOcNOSScrapliPlatform = "ipinfusion_ocnos"
	IPInfusionOcNOSDefaultUser     = "root"
	IPInfusionOcNOSDefaultPass     = "root"
)

type IPInfusionOcNOS struct {
	*instance.Qemu
	*ScrapliConsole
}

func (p *IPInfusionOcNOS) Package(
	_, _ string,
) (packageFiles, runFiles []string, err error) {
	return nil, nil, err
}

func (p *IPInfusionOcNOS) Install(opts ...instance.Option) error {
	p.Loggers.Base.Info("install requested")

	a, opts, err := setInstallArgs(opts...)
	if err != nil {
		return err
	}

	c := make(chan error, 1)
	stop := make(chan bool, 1)

	go func() { //nolint:dupl
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

		p.Loggers.Base.Debug("start ready state acquired, logging in")

		err = p.login(
			&loginArgs{
				username: IPInfusionOcNOSDefaultUser,
				password: IPInfusionOcNOSDefaultPass,
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

func (p *IPInfusionOcNOS) Start(opts ...instance.Option) error {
	p.Loggers.Base.Info("start platform instance requested")

	a, opts, err := setStartArgs(opts...)
	if err != nil {
		return err
	}

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
			username: IPInfusionOcNOSDefaultUser,
			password: IPInfusionOcNOSDefaultPass,
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

func (p *IPInfusionOcNOS) startReady() error {
	// openRetry doesn't do auth and doesn't call onOpen as it is set to nil somewhere before this
	err := p.openRetry()
	if err != nil {
		return err
	}

	err = p.readUntil(
		[]byte("OcNOS login:"),
		getPlatformBootTimeout(PlatformTypeIPInfusionOcNOS),
	)

	return err
}

func (p *IPInfusionOcNOS) SaveConfig() error {
	p.Loggers.Base.Info("save config requested")

	_, err := p.c.SendCommand(
		"copy running-config startup-config",
		base.WithSendTimeoutOps(
			time.Duration(getPlatformSaveTimeout(PlatformTypeIPInfusionOcNOS))*time.Second,
		),
	)

	return err
}

func (p *IPInfusionOcNOS) SetUserPass(usr, pwd string) error {
	p.Loggers.Base.Infof("set user/password for user '%s' requested", usr)

	return p.Config([]string{fmt.Sprintf(
		"username %s role network-admin password %s",
		usr,
		pwd)})
}

func (p *IPInfusionOcNOS) SetHostname(h string) error {
	p.Loggers.Base.Infof("set hostname '%s' requested", h)

	return p.Config([]string{fmt.Sprintf(
		"hostname %s",
		h)})
}
