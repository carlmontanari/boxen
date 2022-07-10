package platforms

import (
	_ "embed" // embed FS
	"fmt"
	"time"

	sopoptions "github.com/scrapli/scrapligo/driver/opoptions"

	"github.com/carlmontanari/boxen/boxen/instance"
)

const (
	CheckpointCloudguardDefaultUser = "admin"
	CheckpointCloudguardDefaultPass = "admin"

	CheckpointCloudguardDefaultScrapliPlatformDefinitionFile = "https://gist.githubusercontent.com/hellt/1eee1024bc1cb3121aaeac199d48663a/raw/07caf0b024802da2dbb6fe17dbabcb26231b8cb6/checkpoint_cloudguard.yaml" // nolint:lll

	checkpointCloudGuardDefaultBootTime = 720
)

type CheckpointCloudguard struct {
	*instance.Qemu
	*ScrapliConsole
}

func (p *CheckpointCloudguard) Package(
	_, _ string,
) (packageFiles, runFiles []string, err error) {
	return nil, nil, err
}

func (p *CheckpointCloudguard) Install(opts ...instance.Option) error { // nolint:dupl
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
				username: CheckpointCloudguardDefaultUser,
				password: CheckpointCloudguardDefaultPass,
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

func (p *CheckpointCloudguard) Start(opts ...instance.Option) error {
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
			username: CheckpointCloudguardDefaultUser,
			password: CheckpointCloudguardDefaultPass,
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

func (p *CheckpointCloudguard) startReady() error {
	// openRetry doesn't do auth and doesn't call onOpen as it is set to nil somewhere before this
	err := p.openRetry()
	if err != nil {
		return err
	}

	err = p.readUntil(
		[]byte("This system is for authorized use only"),
		getPlatformBootTimeout(PlatformTypeCheckpointCloudguard),
	)

	return err
}

func (p *CheckpointCloudguard) SaveConfig() error {
	p.Loggers.Base.Info("save config requested")

	_, err := p.c.SendCommand(
		"save config",
		sopoptions.WithTimeoutOps(
			time.Duration(getPlatformSaveTimeout(PlatformTypeCheckpointCloudguard))*time.Second,
		),
	)

	return err
}

func (p *CheckpointCloudguard) SetUserPass(usr, pwd string) error {
	if usr == CheckpointCloudguardDefaultPass && pwd == CheckpointCloudguardDefaultPass {
		p.Loggers.Base.Info("skipping user creation, since credentials match defaults for platform")
		return nil
	}

	p.Loggers.Base.Infof("set user/password for user '%s' requested", usr)

	return p.Config([]string{
		fmt.Sprintf(
			"add user %s uid 0 homedir /home/%s",
			usr,
			usr),
		fmt.Sprintf(
			"add rba user %s roles adminRole",
			usr),
		fmt.Sprintf(
			"set user %s newpass %s",
			usr,
			pwd),
	})
}

func (p *CheckpointCloudguard) SetHostname(h string) error {
	p.Loggers.Base.Infof("set hostname '%s' requested", h)

	return p.Config([]string{fmt.Sprintf("set hostname %s", h)})
}
