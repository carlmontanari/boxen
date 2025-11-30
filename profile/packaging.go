package profile

// Packaging holds info about how we package an instance.
type Packaging struct {
	// a list of things that if seen in a line of stderr we can ignore, otherwise we will error
	// when we see anything on stderr when launching a box
	StdErrIgnore []string `yaml:"stdErrIgnore"`
	Process      []Step   `yaml:"process"`
}
