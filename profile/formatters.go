package profile

import (
	"fmt"
	"strconv"
	"strings"

	boxenerrors "github.com/carlmontanari/boxen/errors"
)

// Formatters holds all the valid "formatter" options for string interpolation in profile content.
type Formatters struct {
	disk           string
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
		disk = p.ResolvedDisk
	}

	return &Formatters{
		disk:           disk,
		extraFiles:     p.ExtraFiles,
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
			formatters = append(formatters, f.disk)
		case strings.HasPrefix(formatter, "extraFile"):
			idxStr := strings.TrimRight(strings.TrimLeft("extraFile[", formatter), "]")

			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return nil, err
			}

			formatters = append(formatters, f.extraFiles[idx])
		case formatter == "username":
			formatters = append(formatters, f.username)
		case formatter == "password":
			formatters = append(formatters, f.password)
		case formatter == "hostname":
			formatters = append(formatters, f.hostname)
		default:
			return nil, fmt.Errorf("%w: invalid formatter %q", boxenerrors.ErrBoxen, formatter)
		}
	}

	return formatters, nil
}
