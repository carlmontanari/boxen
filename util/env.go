package util

import "os"

// GetEnvStrOrDefault returns the value of the environment variable k as a string *or* the default d
// if casting fails or the environment variable is not set.
func GetEnvStrOrDefault(k, d string) string {
	v, ok := os.LookupEnv(k)
	if ok && v != "" {
		return v
	}

	return d
}
