package boxen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	boxenassets "github.com/carlmontanari/boxen/assets"
	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenprofile "github.com/carlmontanari/boxen/profile"
	boxenutil "github.com/carlmontanari/boxen/util"
	"go.yaml.in/yaml/v4"
)

func (b *Boxen) resolveProfile(
	profileNameOrFile string,
) (*boxenprofile.Profile, error) {
	b.l.Debug(
		"attempting to resolve given profile",
		"profile",
		profileNameOrFile,
		"disk",
		b.disk,
	)

	maybeProfileFilename := boxenutil.MustExpandPath(profileNameOrFile)

	_, err := os.Stat(maybeProfileFilename)
	if !errors.Is(err, os.ErrNotExist) {
		p, err := loadProfileFile(maybeProfileFilename)
		if err != nil {
			return nil, err
		}

		b.l.Info(
			"resolved profile from path",
			"profile",
			maybeProfileFilename,
		)

		return p, nil
	}

	baseDiskImageName := filepath.Base(b.disk)

	assetFiles, err := boxenassets.Assets.ReadDir("profiles")
	if err != nil {
		return nil, err
	}

	for _, assetFile := range assetFiles {
		p := &boxenprofile.Profile{}

		if assetFile.IsDir() {
			continue
		}

		maybeAssetProfileName := strings.TrimSuffix(assetFile.Name(), ".yaml")

		b.l.Debug(
			"checking asset profile",
			"profile",
			maybeAssetProfileName,
		)

		contents, err := boxenassets.Assets.ReadFile(fmt.Sprintf("profiles/%s", assetFile.Name()))
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal(contents, p)
		if err != nil {
			return nil, err
		}

		if maybeAssetProfileName == profileNameOrFile {
			return p, nil
		}
		if profileNameOrFile != "" {
			continue
		}

		for _, diskPattern := range p.DiskPatterns {
			diskRe, err := regexp.Compile(diskPattern)
			if err != nil {
				b.l.Warn("pattern failed to compile, skipping", "pattern", diskPattern)

				continue
			}

			if diskRe.MatchString(baseDiskImageName) {
				return p, nil
			}
		}
	}

	if profileNameOrFile != "" {
		return nil, fmt.Errorf("%w: unknown profile %q", boxenerrors.ErrBoxen, profileNameOrFile)
	}

	return nil, fmt.Errorf(
		"%w: unable to resolve profile from disk %q",
		boxenerrors.ErrBoxen,
		b.disk,
	)
}

func loadProfileFile(filename string) (*boxenprofile.Profile, error) {
	contents, err := os.ReadFile(filename) //nolint: gosec
	if err != nil {
		return nil, err
	}

	p := &boxenprofile.Profile{}
	if err := yaml.Unmarshal(contents, p); err != nil {
		return nil, err
	}
	profileDirectory, err := filepath.Abs(filepath.Dir(filename))
	if err != nil {
		return nil, err
	}
	for i, filename := range p.ExtraFiles {
		if !filepath.IsAbs(filename) {
			p.ExtraFiles[i] = filepath.Join(profileDirectory, filename)
		}
	}

	return p, nil
}

func (b *Boxen) resolveVersion() error {
	if b.p.VersionPattern == "" {
		// its valid for users to not care about this
		return nil
	}

	versionRe, err := regexp.Compile(b.p.VersionPattern)
	if err != nil {
		b.l.Warn("pattern failed to compile, skipping", "pattern", versionRe)

		return err
	}

	matches := versionRe.FindStringSubmatch(filepath.Base(b.disk))
	if len(matches) == 0 {
		b.l.Warn(
			"version pattern did not match the disk string, skipping version resolution",
			"pattern",
			b.p.VersionPattern,
			"disk",
			filepath.Base(b.disk),
		)

		return nil
	}

	if len(matches) > 1 {
		b.p.ResolvedVersion = matches[1]
	} else {
		b.p.ResolvedVersion = matches[0]
	}

	return nil
}
