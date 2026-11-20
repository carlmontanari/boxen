package profile

import boxenconstants "github.com/carlmontanari/boxen/constants"

// Run holds info about how to run a packaged instance.
type Run struct {
	Process []Step `yaml:"process"`
	// this is the process that is ran if/when a startup config is present (which will have been
	// mounted (or not) by clab). it uses the same `[]Step` as the package/run process does.
	ConfigProcess []Step `yaml:"configProcess"`
	// StartupConfigFiles lists the startup config paths to look for, in order; the first one
	// that exists is the node's startup config. When unset, only the containerlab default
	// /config/startup-config.cfg is used. `boxen save` writes to the existing startup config, or
	// to the first path when none exists yet.
	StartupConfigFiles []string `yaml:"startupConfigFiles"`
	// SaveProcess is ran by `boxen save` in a running node. It logs in if needed and uses
	// capture steps to record the running configuration, which is then written to the startup
	// config file.
	SaveProcess []Step `yaml:"saveProcess"`
}

// GetStartupConfigFiles returns the startup config paths to look for, in order.
func (r *Run) GetStartupConfigFiles() []string {
	if r == nil || len(r.StartupConfigFiles) == 0 {
		return []string{boxenconstants.StartupConfigFilePath}
	}

	return r.StartupConfigFiles
}
