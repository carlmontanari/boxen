package util

import (
	"os"
	"strconv"
	"strings"
)

// GetEnvStrOrDefault returns the value of the environment variable k or the default d if the value
// is unset/empty.
func GetEnvStrOrDefault(k, d string) string {
	v, ok := os.LookupEnv(k)
	if ok && v != "" {
		return v
	}

	return d
}

// GetEnvIntOrDefault returns the value of the environment variable k as an int *or* the default d
// if casting fails or the env var is not set.
func GetEnvIntOrDefault(k string, d int) int {
	v, ok := os.LookupEnv(k)
	if ok && v != "" {
		tv, err := strconv.Atoi(v)
		if err != nil {
			return d
		}

		return tv
	}

	return d
}

// EnvBoolTrue returns true if the environment variable k is set to "true", false otherwise.
func EnvBoolTrue(k string) bool {
	v := GetEnvStrOrDefault(k, "")

	return strings.ToLower(v) == "true"
}
