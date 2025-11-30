package profile

// Profile holds information about how to package/run a virtual machine in the containerlab
// compatible image.
type Profile struct {
	Name string `yaml:"name"`

	// DiskPatterns is an array of strings that will be compiled with ?im and used to check if this
	// profile should be used for a given disk image -- this is only checked if the user doesnt
	// explicitly tell us what profile to use when packaging a disk.
	DiskPatterns   []string        `yaml:"diskPatterns"`
	ExtraFiles     []string        `yaml:"extraFiles"`
	VirtualMachine *VirtualMachine `yaml:"virtualMachine"`

	ScrapliDefinitionNameOrFile string `yaml:"scrapliDefinitionNameOrFile"`

	Packaging *Packaging `yaml:"packaging"`
}
