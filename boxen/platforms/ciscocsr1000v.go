package platforms

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/carlmontanari/boxen/boxen/command"
	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/util"

	"github.com/scrapli/scrapligo/driver/base"
)

const (
	CiscoCsr1000vInstallCdromName = "config.iso"
	CiscoCsr1000vScrapliPlatform  = "cisco_iosxe"
	CiscoCsr1000vDefaultUser      = "admin"
	CiscoCsr1000vDefaultPass      = "admin"
)

type CiscoCsr1000v struct {
	*instance.Qemu
	*ScrapliConsole
}

func ciscoCsr1000vInstallConfig() []byte {
	return []byte(
		"platform console serial\r\n\r\n" +
			"do clear platform software vnic-if nvtable\r\n\r\n" +
			"do wr\r\n\n" +
			"do reload\r\n",
	)
}

func (p *CiscoCsr1000v) Package(
	sourceDir, packageDir string,
) (packageFiles, runFiles []string, err error) {
	_ = sourceDir

	err = os.WriteFile(
		fmt.Sprintf("%s/%s", packageDir, "iosxe_config.txt"),
		ciscoCsr1000vInstallConfig(),
		util.FilePerms,
	)
	if err != nil {
		return nil, nil, err
	}

	// binary to create iso files
	var genisoBinary string

	switch {
	case util.CommandExists(util.ISOBinary):
		genisoBinary = util.ISOBinary
	case util.CommandExists(util.DarwinISOBinary):
		genisoBinary = util.DarwinISOBinary
	}

	switch {
	// if genisoBinary was not detected - use docker
	case genisoBinary == "":
		_, err = command.Execute(
			util.DockerCmd,
			command.WithArgs(
				[]string{
					"run",
					"-v",
					packageDir + ":/work",
					"-w",
					packageDir,
					util.ISOBinaryContainer,
					"mkisofs",
					"-l",
					"-o",
					CiscoCsr1000vInstallCdromName,
					"iosxe_config.txt",
				},
			),
			command.WithWait(true),
		)
	default:
		_, err = command.Execute(
			genisoBinary,
			command.WithArgs(
				[]string{"-l", "-o", CiscoCsr1000vInstallCdromName, "iosxe_config.txt"},
			),
			command.WithWorkDir(packageDir),
			command.WithWait(true),
		)
	}

	if err != nil {
		return nil, nil, err
	}

	return []string{CiscoCsr1000vInstallCdromName}, []string{}, err
}

func (p *CiscoCsr1000v) patchCmdCdrom(c *instance.QemuLaunchCmd) {
	diskDir := filepath.Dir(p.Disk)
	c.Extra = append(
		c.Extra,
		[]string{"-cdrom", fmt.Sprintf("%s/%s", diskDir, CiscoCsr1000vInstallCdromName)}...)
}

func (p *CiscoCsr1000v) modifyStartCmd(c *instance.QemuLaunchCmd) {
	_ = c
}

func (p *CiscoCsr1000v) modifyInstallCmd(c *instance.QemuLaunchCmd) {
	p.modifyStartCmd(c)
	p.patchCmdCdrom(c)
}

func (p *CiscoCsr1000v) startReady() error {
	err := p.openRetry()
	if err != nil {
		return err
	}

	err = p.readUntil(
		[]byte("Press RETURN to get started"),
		getPlatformBootTimeout(PlatformTypeCiscoCsr1000v),
	)

	return err
}

func (p *CiscoCsr1000v) Install(opts ...instance.Option) error {
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

		p.Loggers.Base.Debug("start ready state acquired, logging in")

		err = p.login(
			&loginArgs{
				username: CiscoCsr1000vDefaultUser,
				password: CiscoCsr1000vDefaultPass,
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

func (p *CiscoCsr1000v) Start(opts ...instance.Option) error {
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

	p.Loggers.Base.Info("starting platform instance complete")

	return nil
}

func (p *CiscoCsr1000v) SaveConfig() error {
	p.Loggers.Base.Info("save config requested")

	_, err := p.c.SendCommand(
		"copy running-config startup-config",
		base.WithSendTimeoutOps(
			time.Duration(getPlatformSaveTimeout(PlatformTypeCiscoCsr1000v))*time.Second,
		),
	)

	return err
}

func (p *CiscoCsr1000v) SetUserPass(usr, pwd string) error {
	p.Loggers.Base.Infof("set user/password for user '%s' requested", usr)

	return p.Config([]string{fmt.Sprintf(
		"username %s privilege 15 password %s",
		usr,
		pwd)})
}

func (p *CiscoCsr1000v) SetHostname(h string) error {
	p.Loggers.Base.Infof("set hostname '%s' requested", h)

	return p.Config([]string{fmt.Sprintf(
		"hostname %s",
		h)})
}
