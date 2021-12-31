package platforms

import "github.com/carlmontanari/boxen/boxen/util"

const (
	// DefaultInstallTime is the default value for "installation" timeout -- this means handling
	// any of the initial prompt stuff and saving the disk.
	DefaultInstallTime = 600
	// DefaultBootTime default value for "bootup" time -- as in time before the console is ready
	// for inputs.
	DefaultBootTime = 360
	// DefaultSaveTime default value for saving configurations.
	DefaultSaveTime = 120
)

func getPlatformBootTimeout(pT string) int {
	var t int

	switch pT {
	case PlatformTypeCiscoN9kv:
		t = ciscoN9kvDefaultBootTime
	case PlatformTypeCiscoXrv9k:
		t = ciscoXrv9kDefaultBootTime
	case PlatformTypePaloAltoPanos:
		t = paloAltoPanosDefaultBootTime
	default:
		t = DefaultBootTime
	}

	return util.ApplyTimeoutMultiplier(t)
}

func getPlatformSaveTimeout(pT string) int {
	_ = pT

	t := DefaultSaveTime

	return util.ApplyTimeoutMultiplier(t)
}
