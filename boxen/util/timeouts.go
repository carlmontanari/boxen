package util

func GetTimeoutMultiplier() int {
	return GetEnvIntOrDefault(
		"BOXEN_TIMEOUT_MULTIPLIER",
		1,
	)
}

func ApplyTimeoutMultiplier(timeout int) int {
	return timeout * GetTimeoutMultiplier()
}
