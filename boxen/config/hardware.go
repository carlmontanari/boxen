package config

type ProfileHardware struct {
	Memory          int      `yaml:"memory,omitempty"`
	Acceleration    []string `yaml:"acceleration,omitempty"`
	SerialPortCount int      `yaml:"serial_port_count,omitempty"`
	NicType         string   `yaml:"nic_type,omitempty"`
	NicCount        int      `yaml:"nic_count,omitempty"`
	NicPerBus       int      `yaml:"nic_per_bus,omitempty"`
}

func (p *ProfileHardware) ToHardware() *Hardware {
	// especially for packaging we basically just roll w/ the default profile for bootstrapping, but
	// because hardware profile has *count* of serial instead of actual serial ports we can't use
	// `ProfileHardware` as `Hardware` -- so this method just converts and drops the serial ports
	return &Hardware{
		Memory:       p.Memory,
		Acceleration: p.Acceleration,
		NicType:      p.NicType,
		NicCount:     p.NicCount,
		NicPerBus:    p.NicPerBus,
	}
}

type Hardware struct {
	Memory       int      `yaml:"memory,omitempty"`
	Acceleration []string `yaml:"acceleration,omitempty"`
	MonitorPort  int      `yaml:"monitor_port,omitempty"`
	SerialPorts  []int    `yaml:"serial_ports,omitempty"`
	NicType      string   `yaml:"nic_type,omitempty"`
	NicCount     int      `yaml:"nic_count,omitempty"`
	NicPerBus    int      `yaml:"nic_per_bus,omitempty"`
}

type Advanced struct {
	Display string       `yaml:"display,omitempty"`
	Machine string       `yaml:"machine,omitempty"`
	CPU     *AdvancedCPU `yaml:"cpu,omitempty"`
}

type AdvancedCPU struct {
	Emulation string `yaml:"emulation,omitempty"`
	Cores     int    `yaml:"cores,omitempty"`
	Threads   int    `yaml:"threads,omitempty"`
	Sockets   int    `yaml:"sockets,omitempty"`
}
