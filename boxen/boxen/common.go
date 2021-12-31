package boxen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/carlmontanari/boxen/boxen"
	"github.com/carlmontanari/boxen/boxen/command"
	"github.com/carlmontanari/boxen/boxen/config"
	"github.com/carlmontanari/boxen/boxen/platforms"
	"github.com/carlmontanari/boxen/boxen/util"

	"gopkg.in/yaml.v2"
)

var Version = "v0.0.0" //nolint: gochecknoglobals

type Disk struct {
	Disk         string
	Vendor       string
	Platform     string
	PlatformType string
	Version      string
}

func (b *Boxen) sourceDiskExists(pT, diskVersion string) bool {
	v, ok := b.Config.Platforms[pT]
	if !ok {
		return false
	}

	if util.StringSliceContains(
		diskVersion,
		v.SourceDisks,
	) {
		return true
	}

	return false
}

func getDiskData(f string) (*Disk, error) {
	f, err := util.ResolveFile(f)
	if err != nil {
		return nil, err
	}

	d := &Disk{}
	d.Disk = f

	v, p, err := platforms.GetPlatformTypeFromDisk(filepath.Base(f))
	if err != nil {
		return nil, err
	}

	d.Vendor = v
	d.Platform = p
	d.PlatformType = platforms.GetPlatformType(d.Vendor, d.Platform)

	dV, err := platforms.GetDiskVersion(filepath.Base(f), d.PlatformType)
	d.Version = dV

	return d, err
}

// GetDefaultProfile fetches the default instance profile for a given platform type 'pt'.
func GetDefaultProfile(pt string) (*config.Profile, error) {
	f, err := boxen.Assets.ReadFile(fmt.Sprintf("assets/profiles/%s.yaml", pt))

	envProfilePath := os.Getenv(fmt.Sprintf("BOXEN_%s_PROFILE", strings.ToUpper(pt)))
	if envProfilePath != "" {
		f, err = os.ReadFile(envProfilePath)
	}

	if err != nil {
		return nil, err
	}

	profile := &config.Profile{}

	err = yaml.UnmarshalStrict(f, profile)
	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (b *Boxen) installAllocateDisks(i *installInfo) error {
	d, err := getDiskData(i.inDisk)
	if err != nil {
		msg := fmt.Sprintf("failed gleaning source data from disk '%s'", i.inDisk)

		b.Logger.Critical(msg)

		return fmt.Errorf(
			"%w: %s",
			util.ErrInspectionError,
			msg,
		)
	}

	i.srcDisk = d
	i.name = fmt.Sprintf("%s_%s", d.PlatformType, d.Version)
	i.newDisk = fmt.Sprintf("%s/%s.qcow2", i.tmpDir, i.name)

	_, err = command.Execute(
		util.QemuImgCmd,
		command.WithArgs(
			[]string{"convert", "-O", "qcow2", i.srcDisk.Disk, i.newDisk},
		),
		command.WithWait(true),
	)
	if err != nil {
		b.Logger.Criticalf("error copying source disk image: %s\n", err)

		return err
	}

	return nil
}
