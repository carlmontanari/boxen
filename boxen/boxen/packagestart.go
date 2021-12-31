package boxen

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/carlmontanari/boxen/boxen/command"
	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/platforms"
	"github.com/carlmontanari/boxen/boxen/util"
)

func (b *Boxen) clabNicProvisionDelay() error {
	clabIntfs := util.GetEnvIntOrDefault("CLAB_INTFS", 0)

	if clabIntfs == 0 {
		return nil
	}

	b.Logger.Info("waiting until clab nics show up")

	intfGlob := "/sys/class/net/eth*"

	for {
		provisionedNics, err := filepath.Glob(intfGlob)
		if err != nil {
			return err
		}

		// clab intfs + 1 for mgmt
		if len(provisionedNics) >= clabIntfs+1 {
			b.Logger.Debug("clab nics available, moving on")

			return nil
		}

		time.Sleep(5 * time.Second) //nolint:gomnd
	}
}

// packageSocat launches the background socat tasks for the packaged instance.
func (b *Boxen) packageSocat(name string) error {
	b.Logger.Debug("launching socat processes")

	tcpPortPairs := b.Config.Instances[name].MgmtIntf.Nat.TCP
	udpPortPairs := b.Config.Instances[name].MgmtIntf.Nat.UDP

	for _, tcpPortPair := range tcpPortPairs {
		_, err := command.Execute("socat", command.WithArgs([]string{
			fmt.Sprintf("TCP-LISTEN:%d,fork", tcpPortPair.InstanceSide),
			fmt.Sprintf("TCP:127.0.0.1:%d", tcpPortPair.HostSide),
		}))
		if err != nil {
			b.Logger.Criticalf("error launching management tcp socat command: %s\n", err)

			return err
		}
	}

	for _, udpPortPair := range udpPortPairs {
		_, err := command.Execute("socat", command.WithArgs([]string{
			fmt.Sprintf("UDP-LISTEN:%d,fork", udpPortPair.InstanceSide),
			fmt.Sprintf("UDP:127.0.0.1:%d", udpPortPair.HostSide),
		}))
		if err != nil {
			b.Logger.Criticalf("error launching management udp socat command: %s\n", err)

			return err
		}
	}

	b.Logger.Debug("socat processes launched...")

	return nil
}

func (b *Boxen) clabStartDelay() {
	startDelay := util.GetEnvIntOrDefault(
		"BOOT_DELAY",
		0,
	)

	if startDelay != 0 {
		b.Logger.Infof("start delay set, sleeping for %d seconds...", startDelay)
		time.Sleep(time.Duration(startDelay) * time.Second)

		b.Logger.Debug("start delay complete, continuing")
	}
}

func (b *Boxen) packageStartConfig(name, username, password, hostname, config string) error {
	startupConfig := util.GetEnvStrOrDefault(
		"STARTUP_CONFIG",
		"",
	)

	saveRequired := false

	f := startupConfig
	if config != "" {
		f = config
	}

	if f != "" { //nolint:nestif
		err := b.Instances[name].InstallConfig(f, true)
		if err != nil {
			return err
		}

		saveRequired = true
	} else {
		if username != "" && password != "" {
			err := b.Instances[name].SetUserPass(username, password)
			if err != nil {
				return err
			}

			saveRequired = true
		}

		if hostname != "" {
			err := b.Instances[name].SetHostname(hostname)
			if err != nil {
				return err
			}

			saveRequired = true
		}
	}

	if saveRequired {
		err := b.Instances[name].SaveConfig()
		if err != nil {
			return err
		}
	}

	return nil
}

// PackageStart starts the qemu instance vm inside a packaged boxen container.
func (b *Boxen) PackageStart(username, password, hostname, config string) error {
	b.Logger.Info("package start requested")

	name := b.getPackagedInstanceName()

	instanceLoggers, err := instance.NewInstanceLoggersFOut(b.Logger, "/")
	if err != nil {
		return err
	}

	q, err := platforms.NewPlatformFromConfig(
		name,
		b.Config,
		instanceLoggers,
	)
	if err != nil {
		return err
	}

	err = b.clabNicProvisionDelay()
	if err != nil {
		return err
	}

	err = b.packageSocat(name)
	if err != nil {
		return err
	}

	b.clabStartDelay()

	b.Instances[name] = q

	err = b.Instances[name].Start(
		// with prepare console so that the console has paging disabled and all that good stuff
		// so, we can set the hostname/username/password if desired.
		platforms.WithPrepareConsole(true),
	)
	if err != nil {
		return err
	}

	err = b.packageStartConfig(name, username, password, hostname, config)
	if err != nil {
		return err
	}

	err = b.Instances[name].Detach()
	if err != nil {
		return err
	}

	b.Logger.Info("package start completed successfully, running until signal interrupt")

	b.Instances[name].RunUntilSigInt()

	return nil
}
