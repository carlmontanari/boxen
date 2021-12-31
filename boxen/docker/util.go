package docker

import (
	"os"
	"strings"
)

// ReadCidFile loads provided cidfile f and returns trimmed string contents.
func ReadCidFile(f string) string {
	o, _ := os.ReadFile(f)
	return strings.TrimSpace(string(o))
}
