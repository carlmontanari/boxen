package boxen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/carlmontanari/boxen/boxen/platforms"

	"github.com/carlmontanari/boxen/boxen"
	"github.com/carlmontanari/boxen/boxen/command"
	"github.com/carlmontanari/boxen/boxen/config"
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

func getDiskData(i *installInfo) error {
	f, err := util.ResolveFile(i.inDisk)
	if err != nil {
		return err
	}

	// if the disk object has already been set (because user provided the values) we can just
	// set the resolved disk path then bail out of here.
	if i.srcDisk != nil {
		i.srcDisk.Disk = f

		return nil
	}

	i.srcDisk = &Disk{}
	i.srcDisk.Disk = f

	v, p, err := platforms.GetPlatformTypeFromDisk(filepath.Base(f))
	if err != nil {
		return err
	}

	i.srcDisk.Vendor = v
	i.srcDisk.Platform = p
	i.srcDisk.PlatformType = platforms.GetPlatformType(i.srcDisk.Vendor, i.srcDisk.Platform)

	dV, err := platforms.GetDiskVersion(filepath.Base(f), i.srcDisk.PlatformType)
	i.srcDisk.Version = dV

	return err
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
	err := getDiskData(i)
	if err != nil {
		msg := fmt.Sprintf("failed gleaning source data from disk '%s'", i.inDisk)

		b.Logger.Critical(msg)

		return fmt.Errorf(
			"%w: %s",
			util.ErrInspectionError,
			msg,
		)
	}

	i.name = fmt.Sprintf("%s_%s", i.srcDisk.PlatformType, i.srcDisk.Version)
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

func (b *Boxen) handleProvidedPlatformInfo(i *installInfo, vendor, platform, version string) error {
	if !util.AllStringVal("", vendor, platform, version) {
		if util.AnyStringVal("", vendor, platform, version) {
			return fmt.Errorf("%w: one or more of vendor, platform, version set, "+
				"but not all values provided, if explicitly targeting a specific "+
				" vendor/platform/version you must provide all values",
				util.ErrValidationError)
		}

		pT := platforms.GetPlatformType(vendor, platform)

		if pT == "" {
			return fmt.Errorf("%w: provided vendor/platform '%s'/'%s' not supported",
				util.ErrValidationError, vendor, platform)
		}

		i.srcDisk = &Disk{
			Vendor:       vendor,
			Platform:     platform,
			PlatformType: pT,
			Version:      version,
		}
	}

	return nil
}
