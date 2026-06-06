package util

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	boxenconstants "github.com/carlmontanari/boxen/constants"
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

// ClabIntfPrefix returns the containerlab interface name prefix (e.g. "eth"),
// honoring the CLAB_INTF_PREFIX environment variable when set.
func ClabIntfPrefix() string {
	return GetEnvStrOrDefault(boxenconstants.EnvClabIntfPrefix, "eth")
}

// ClabIntfName returns the containerlab interface name for the given index
// (e.g. "eth0"), honoring the CLAB_INTF_PREFIX environment variable.
func ClabIntfName(idx int) string {
	return fmt.Sprintf("%s%d", ClabIntfPrefix(), idx)
}

// ClabMgmtIntfName returns the containerlab management interface name. It
// honors the CLAB_MGMT_INTF environment variable when set, otherwise defaulting
// to the zero-indexed interface (e.g. "eth0").
func ClabMgmtIntfName() string {
	return GetEnvStrOrDefault(boxenconstants.EnvClabMgmtIntf, ClabIntfName(0))
}
