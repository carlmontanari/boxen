package boxen

import (
	"os"

	"github.com/carlmontanari/boxen/boxen/command"
	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/platforms"
	"github.com/carlmontanari/boxen/boxen/util"
)

func (b *Boxen) getPackagedInstanceName() string {
	var name string

	for n := range b.Config.Instances {
		// there will only ever be one instance in the "package" mode, so we'll just iterate, get
		// that one instance's name and peace out; probably a smarter way...
		name = n
		break
	}

	return name
}

func (b *Boxen) shrinkDisk(name string) error {
	err := os.Rename(b.Config.Instances[name].Disk, "fat.qcow2")
	if err != nil {
		return err
	}

	_, err = command.Execute(
		"virt-sparsify",
		command.WithArgs([]string{"fat.qcow2", "--compress", b.Config.Instances[name].Disk}),
		command.WithWait(true),
	)
	if err != nil {
		return err
	}

	err = os.Remove("fat.qcow2")
	if err != nil {
		return err
	}

	err = os.RemoveAll("./var/tmp/.guestfs-0")
	if err != nil {
		return err
	}

	return nil
}

// PackageInstall is the function that is run *in* the initially built container image. This
// function handles the initial provisioning of the instance.
func (b *Boxen) PackageInstall() error {
	b.Logger.Debug("package install starting")

	name := b.getPackagedInstanceName()

	// for things *not* packaging we will want to merge the instance config w/ the profile and/or
	// defaults, that is not necessary for packaging since we just load up the config all on the
	// instance though!
	q, err := platforms.NewPlatformFromConfig(
		name,
		b.Config,
		&instance.Loggers{
			Base:    b.Logger,
			Stdout:  os.Stdout,
			Stderr:  os.Stdout,
			Console: os.Stdout,
		},
	)
	if err != nil {
		return err
	}

	b.Instances[name] = q

	configLines, err := b.RenderInitialConfig(name)
	if err != nil {
		return err
	}

	b.Logger.Debug("begin instance install")

	err = b.Instances[name].Install(
		platforms.WithInstallConfig(configLines),
		instance.WithSudo(false),
	)
	if err != nil {
		b.Logger.Criticalf("package installation failed: %s\n", err)

		return err
	}

	if util.GetEnvIntOrDefault("BOXEN_SPARSIFY_DISK", 0) > 0 {
		err = b.shrinkDisk(name)
		if err != nil {
			b.Logger.Criticalf("shrinking installation disk failed: %s\n", err)

			return err
		}
	}

	return nil
}
