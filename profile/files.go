package profile

import (
	"errors"
	"fmt"
	"os"

	boxenerrors "github.com/carlmontanari/boxen/errors"
)

// readFile returns raw file contents, optionally using a default for a missing file.
func readFile(path string, defaults ...string) (string, error) {
	if len(defaults) > 1 {
		return "", fmt.Errorf("%w: readFile accepts at most one default", boxenerrors.ErrBoxen)
	}
	//nolint:gosec // Profile-controlled paths, just like contentFromFile and fileBase64.
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && len(defaults) == 1 {
		return defaults[0], nil
	}
	if err != nil {
		return "", err
	}

	return string(data), nil
}
