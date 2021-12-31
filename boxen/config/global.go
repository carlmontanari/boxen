package config

type GlobalOptions struct {
	Credentials *Credentials `yaml:"credentials,omitempty"`
	Qemu        *Qemu        `yaml:"qemu,omitempty"`
	Build       *Build       `yaml:"build,omitempty"`
}

type Credentials struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// NewDefaultCredentials returns a Credentials object with the boxen default creds set.
func NewDefaultCredentials() *Credentials {
	return &Credentials{
		Username: "boxen",
		Password: "b0x3N-b0x3N",
	}
}

type Qemu struct {
	Acceleration []string `yaml:"acceleration,omitempty"`
	Binary       string   `yaml:"binary,omitempty"`
	// UseThickDisks copies full disks instead of creating qemu disks with a backing chain
	UseThickDisks bool `yaml:"use_thick_disks,omitempty"`
}

type Build struct {
	InstancePath string `yaml:"instance_path,omitempty"`
	SourcePath   string `yaml:"source_path,omitempty"`
}
