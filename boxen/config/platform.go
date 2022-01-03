package config

// Profile is a struct containing information about the virtual machine hardware and port allocation
// as stored in the boxen configuration.
type Profile struct {
	Hardware    *ProfileHardware `yaml:"hardware,omitempty"`
	Advanced    *Advanced        `yaml:"advanced,omitempty"`
	TPCNatPorts []int            `yaml:"tcp_nat_ports,omitempty"`
	UDPNatPorts []int            `yaml:"udp_nat_ports,omitempty"`
}

// Platform contains information about a given platform -- i.e. Arista vEOS -- that is stored in the
// boxen configuration this includes the available hardware profiles and source disks that have
// been "installed".
type Platform struct {
	SourceDisks []string            `yaml:"source_disks,omitempty"`
	Profiles    map[string]*Profile `yaml:"profiles,omitempty"`
}

// NewPlatform returns an empty Platform object.
func NewPlatform() *Platform {
	return &Platform{
		SourceDisks: make([]string, 0),
		Profiles:    make(map[string]*Profile),
	}
}
