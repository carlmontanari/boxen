package util

import (
	"os"
	"strconv"
)

func GetEnvIntOrDefault(k string, d int) int {
	if v, ok := os.LookupEnv(k); ok {
		ev, err := strconv.Atoi(v)

		if err != nil {
			return d
		}

		return ev
	}

	return d
}

func GetEnvStrOrDefault(k, d string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}

	return d
}
