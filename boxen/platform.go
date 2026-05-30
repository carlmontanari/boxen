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

	p := &boxenprofile.Profile{}

	maybeProfileFilename := boxenutil.MustExpandPath(profileNameOrFile)

	_, err := os.Stat(maybeProfileFilename)
	if !errors.Is(err, os.ErrNotExist) {
		contents, err := os.ReadFile(maybeProfileFilename) //nolint: gosec
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal(contents, p)
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
		p = &boxenprofile.Profile{}

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

	return nil, fmt.Errorf(
		"%w: unable to resolve profile from given profile name %q or disk %q",
		boxenerrors.ErrBoxen,
		profileNameOrFile,
		b.disk,
	)
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

	if len(matches) > 1 {
		b.p.ResolvedVersion = matches[1]
	} else {
		b.p.ResolvedVersion = matches[0]
	}

	return nil
}
