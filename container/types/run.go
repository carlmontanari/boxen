package types

// RunConfig is a config struct passed to the runtime interface's Run method.
type RunConfig struct {
	Name       string
	Image      string
	Env        []string
	Detached   bool
	Remove     bool
	Privileged bool
}
