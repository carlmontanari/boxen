package util

// GetTimeoutMultiplier returns either 1, or the integer value of the environment variable
// BOXEN_TIMEOUT_MULTIPLIER.
func GetTimeoutMultiplier() int {
	return GetEnvIntOrDefault(
		"BOXEN_TIMEOUT_MULTIPLIER",
		1,
	)
}

// ApplyTimeoutMultiplier returns the timeout, as an integer, after being multiplied by the
// environment variable BOXEN_TIMEOUT_MULTIPLIER.
func ApplyTimeoutMultiplier(timeout int) int {
	return timeout * GetTimeoutMultiplier()
}
