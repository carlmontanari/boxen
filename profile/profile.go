package profile

// Profile holds information about how to package/run a virtual machine in the containerlab
// compatible image.
type Profile struct {
	Name string `yaml:"name"`

	// DiskPatterns is an array of strings that will be compiled with ?im and used to check if this
	// profile should be used for a given disk image -- this is only checked if the user doesnt
	// explicitly tell us what profile to use when packaging a disk.
	DiskPatterns []string `yaml:"diskPatterns"`
	ResolvedDisk string   `yaml:"-"`

	// VersionPattern is used to snag the version from the resolved disk (assuming the disk name
	// includes the version of course... which... lets hope it does!); this is then set and is
	// made available as a `formatter`.
	VersionPattern  string `yaml:"versionPattern"`
	ResolvedVersion string `yaml:"-"`

	ExtraFiles     []string        `yaml:"extraFiles"`
	VirtualMachine *VirtualMachine `yaml:"virtualMachine"`

	ScrapliDefinitionNameOrFile string `yaml:"scrapliDefinitionNameOrFile"`

	// commands that are executed before the packaging process is kicked off -- this can be used
	// to create new files/disks/etc. (i.e. csr1000v genisoimage for the initial config). if you
	// want to do more elaborate shell script type things its probably nicer to put that script
	// in "extra files" and invoke that -- all "commands" are invoked like `/bin/bash -c XXX`.
	PrePackagingCommands  []string   `yaml:"prePackagingCommands"`
	Packaging             *Packaging `yaml:"packaging"`
	PostPackagingCommands []string   `yaml:"postPackagingCommands"`

	PreRunCommands []string `yaml:"preRunCommands"`
	Run            *Run     `yaml:"run"`
}
