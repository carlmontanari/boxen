package util

import (
	"os"
	"path/filepath"
	"strings"
)

// MustExpandPath expands the provided path if prefixed with "~" -- it cannot fail, it can only
// return the original path or the successfully expanded path.
func MustExpandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}

		path = filepath.Join(home, path[1:])
	}

	return path
}
