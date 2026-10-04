package agent

import (
	"errors"
	"os"
	"strings"

	boxenconstants "github.com/carlmontanari/boxen/constants"
)

const healthFilePermissions = 0o644

var errUnhealthy = errors.New("unhealthy")

// healthFilePath is the health file consulted by the container healthcheck.
var healthFilePath = boxenconstants.HealthFilePath

// writeHealth writes the given status string to the health file consulted by the
// container healthcheck.
func (a *Agent) writeHealth(status string) error {
	return os.WriteFile(
		healthFilePath,
		[]byte(status),
		healthFilePermissions,
	)
}

// Health implements the container healthcheck. It returns nil (exit 0, healthy)
// only when the health file's status field is "0", otherwise an error (exit 1).
func Health() error {
	b, err := os.ReadFile(healthFilePath) //nolint: gosec // fixed path, a variable for tests
	if err != nil {
		return err
	}

	fields := strings.Fields(string(b))
	if len(fields) == 0 || fields[0] != "0" {
		return errUnhealthy
	}

	return nil
}
