package profile

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	boxenerrors "github.com/carlmontanari/boxen/errors"
)

// Formatters holds all the valid "formatter" options for string interpolation in profile content.
type Formatters struct {
	disk           string
	version        string
	extraFiles     []string
	username       string
	password       string
	hostname       string
	connectionMode string
}

// NewFormatters returns a Formatters object based on the given inputs/profile.
func NewFormatters(
	username,
	password,
	hostname,
	connectionMode string,
	p *Profile,
) *Formatters {
	// disk is always disk.qcow2 in "run" mode, but we maybe have a disk that we resolved
	// during packaging, so override that if thats the case
	disk := "disk.qcow2"

	if p.ResolvedDisk != "" {
		// resolved disk we received from boxen builder (the main cli) will be fully qualified,
		// but that file will just be in . on the agent container; same applies to extra files
		disk = filepath.Base(p.ResolvedDisk)
	}

	extraFiles := make([]string, len(p.ExtraFiles))

	for idx := range p.ExtraFiles {
		extraFiles[idx] = filepath.Base(p.ExtraFiles[idx])
	}

	return &Formatters{
		disk:           disk,
		version:        p.ResolvedVersion,
		extraFiles:     extraFiles,
		username:       username,
		password:       password,
		hostname:       hostname,
		connectionMode: connectionMode,
	}
}

// UnpackFormatters accepts the users inputs, and returns a list of formatters to use with Sprintf.
func (f *Formatters) UnpackFormatters(inputs []string) ([]any, error) {
	var formatters []any

	for _, formatter := range inputs {
		switch {
		case formatter == "disk":
			if f.disk == "" {
				return nil, fmt.Errorf(
					"%w: disk unset but disk formatter requested", boxenerrors.ErrBoxen,
				)
			}

			formatters = append(formatters, f.disk)
		case formatter == "version":
			if f.version == "" {
				return nil, fmt.Errorf(
					"%w: version unset but version formatter requested", boxenerrors.ErrBoxen,
				)
			}

			formatters = append(formatters, f.version)
		case strings.HasPrefix(formatter, "extraFile"):
			idxStr := strings.TrimRight(strings.TrimLeft("extraFile[", formatter), "]")

			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return nil, err
			}

			formatters = append(formatters, f.extraFiles[idx])
		case formatter == "username":
			if f.username == "" {
				return nil, fmt.Errorf(
					"%w: username unset but username formatter requested", boxenerrors.ErrBoxen,
				)
			}

			formatters = append(formatters, f.username)
		case formatter == "password":
			if f.password == "" {
				return nil, fmt.Errorf(
					"%w: password unset but password formatter requested", boxenerrors.ErrBoxen,
				)
			}

			formatters = append(formatters, f.password)
		case formatter == "hostname":
			if f.hostname == "" {
				return nil, fmt.Errorf(
					"%w: hostname unset but hostname formatter requested", boxenerrors.ErrBoxen,
				)
			}

			formatters = append(formatters, f.hostname)
		default:
			return nil, fmt.Errorf("%w: invalid formatter %q", boxenerrors.ErrBoxen, formatter)
		}
	}

	return formatters, nil
}
