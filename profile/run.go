package profile

// Run holds info about how to run a packaged instance.
type Run struct {
	Process []Step `yaml:"process"`
	// this is the process that is ran if/when a startup config is present (which will have been
	// mounted (or not) by clab). it uses the same `[]Step` as the package/run process does.
	ConfigProcess []Step `yaml:"configProcess"`
}
