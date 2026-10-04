package profile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"

	boxenconstants "github.com/carlmontanari/boxen/constants"
)

// Run holds info about how to run a packaged instance.
type Run struct {
	Process []Step `yaml:"process"`
	// this is the process that is ran if/when a startup config is present (which will have been
	// mounted (or not) by clab). it uses the same `[]Step` as the package/run process does.
	ConfigProcess []Step `yaml:"configProcess"`
	// StartupConfigFiles lists the startup config paths to look for, in order; the first one
	// that exists is the node's startup config. When unset, only the containerlab default
	// /config/startup-config.cfg is used. `boxen save` writes to the existing startup config, or
	// to the first path when none exists yet.
	StartupConfigFiles []string `yaml:"startupConfigFiles"`
	// SaveProcess is ran by `boxen save` in a running node. It logs in if needed and uses
	// capture steps to record the running configuration, which is then written to the startup
	// config file.
	SaveProcess []Step `yaml:"saveProcess"`
	// UUIDFrom sets the VM system UUID from a file when the file exists and matches, for guests
	// whose license is bound to the system UUID. The UUID environment variable wins over it.
	UUIDFrom *FileMatch `yaml:"uuidFrom"`
}

// FileMatch selects a value from a file: the first capture group of Pattern in the content of
// File.
type FileMatch struct {
	File    string `yaml:"file"`
	Pattern string `yaml:"pattern"`
}

// Match returns the first capture group of the pattern in the file. It returns false when the file
// does not exist or the pattern does not match.
func (m *FileMatch) Match() (value string, ok bool, err error) {
	pattern, err := regexp.Compile(m.Pattern)
	if err != nil {
		return "", false, err
	}

	b, err := os.ReadFile(m.File)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}

	if err != nil {
		return "", false, err
	}

	match := pattern.FindSubmatch(b)
	if len(match) < 2 { //nolint: mnd // the whole match and the first group
		return "", false, nil
	}

	return string(match[1]), true, nil
}

func (m *FileMatch) validate() []error {
	if m == nil {
		return nil
	}

	var errs []error

	if m.File == "" {
		errs = append(errs, fmt.Errorf("run.uuidFrom.file %w", errRequired))
	}

	pattern, err := regexp.Compile(m.Pattern)

	switch {
	case m.Pattern == "":
		errs = append(errs, fmt.Errorf("run.uuidFrom.pattern %w", errRequired))
	case err != nil:
		errs = append(errs, fmt.Errorf("run.uuidFrom.pattern: %w", err))
	case pattern.NumSubexp() == 0:
		errs = append(errs, fmt.Errorf(
			"run.uuidFrom.pattern %w: it needs a capture group for the uuid",
			errInvalid,
		))
	}

	return errs
}

// GetStartupConfigFiles returns the startup config paths to look for, in order.
func (r *Run) GetStartupConfigFiles() []string {
	if r == nil || len(r.StartupConfigFiles) == 0 {
		return []string{boxenconstants.StartupConfigFilePath}
	}

	return r.StartupConfigFiles
}
