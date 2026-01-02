package profile

// VirtualMachine defines the qemu settings/profile for an endpoint, this will generally come from
// yaml "profile" manifests that we load or users provide to tell us how to configure the vm.
type VirtualMachine struct {
	Emulation string `yaml:"emulation"`
	Machine   string `yaml:"machine"`

	Memory uint `yaml:"memory"`

	CPUEmulation string `yaml:"cpuEmulation"`
	CPUCores     uint8  `yaml:"cpuCores"`
	CPUThreads   uint8  `yaml:"cpuThreads"`
	CPUSockets   uint8  `yaml:"cpuSockets"`

	SerialPortCount uint8 `yaml:"serialPortCount"`

	Display string `yaml:"display"`

	NicType   string `yaml:"nicType"`
	NicCount  uint16 `yaml:"nicCount"`
	NicPerBus uint8  `yaml:"nicPerBus"`

	Management ManagementNIC `yaml:"management"`

	// Overrides allows for completely overriding any of the individual qemu settings that would
	// otherwise be generated from this struct, the options are:
	// - cpu
	// - memory
	// - accel
	// - machine
	// - disk
	// - serial
	// - monitor
	// - display
	// - pci
	// - mgmtNIC
	// - dataNICs
	Overrides map[string][]string `yaml:"overrides"`

	// Mutators is the same mapping as overrides, but instead of directly overriding a value the
	// value in this map is a valid starlark (python-like) script that will accept the []string of
	// the field to mutate -- i.e. it will pass in the list of strings for the "pci" command if the
	// key is "pci" -- the value returned will replace the value of that section of the qemu
	// command.
	Mutators map[string]string `yaml:"mutators"`

	Extras []Extra `yaml:"extras"`
}

// Extra represents an extra qemu arg -- it holds the actual arg(s) to pass and some flags to
// define if it should be at packaging, runtime, or both.
type Extra struct {
	OnPackage bool       `yaml:"onPackage"`
	OnRun     bool       `yaml:"onRun"`
	Val       []ExtraVal `yaml:"val"`
}

// ExtraVal represents an extra string and any formatters that should be applied to it.
type ExtraVal struct {
	Content    string   `yaml:"content"`
	Formatters []string `yaml:"formatters"`
}

// ManagementNIC defines the management nic config for the vm.
type ManagementNIC struct {
	Passthrough bool      `yaml:"passthrough"`
	NatPorts    []NatPort `yaml:"natPorts"`
}

// NatType is an enum-ish value for the type of NAT port -- tcp or udp.
type NatType string

// enumerations of NatType.
const (
	NatTypeTCP NatType = "tcp"
	NatTypeUDP NatType = "udp"
)

// NatPort represents a single tcp or udp nat port.
type NatPort struct {
	Type         NatType `yaml:"type"`
	LocalPort    uint32  `yaml:"localPort"`
	ExternalPort uint32  `yaml:"externalPort"`
}
