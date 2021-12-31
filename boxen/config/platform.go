package config

type Profile struct {
	Hardware    *ProfileHardware `yaml:"hardware,omitempty"`
	Advanced    *Advanced        `yaml:"advanced,omitempty"`
	TPCNatPorts []int            `yaml:"tcp_nat_ports,omitempty"`
	UDPNatPorts []int            `yaml:"udp_nat_ports,omitempty"`
}

type Platform struct {
	SourceDisks []string            `yaml:"source_disks,omitempty"`
	Profiles    map[string]*Profile `yaml:"profiles,omitempty"`
}

func NewPlatform() *Platform {
	return &Platform{
		SourceDisks: make([]string, 0),
		Profiles:    make(map[string]*Profile),
	}
}
