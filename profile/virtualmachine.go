package profile

import "fmt"

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

	NatPorts              []NatPort `yaml:"natPorts"`
	ManagementPassthrough bool      `yaml:"managementPassthrough"`

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
	Overrides map[string][]QemuConfigField `yaml:"overrides"`

	// Mutators is the same mapping as overrides, but instead of directly overriding a value the
	// value in this map is a valid starlark (python-like) script that will accept the []string of
	// the field to mutate -- i.e. it will pass in the list of strings for the "pci" command if the
	// key is "pci" -- the value returned will replace the value of that section of the qemu
	// command.
	Mutators map[string]string `yaml:"mutators"`

	Extras []QemuConfigField `yaml:"extras"`
}

// QemuConfigField represents an extra qemu arg -- it holds the actual arg(s) to pass and some
// flags to define if it should be at packaging, runtime, or both.
type QemuConfigField struct {
	OnPackage bool            `yaml:"onPackage"`
	OnRun     bool            `yaml:"onRun"`
	Val       []QemuConfigVal `yaml:"val"`
}

// Apply applies this ConfigField to the list of qemu commands in `o`.
func (c QemuConfigField) Apply(f *Formatters, isPackaging bool, o []string) ([]string, error) {
	if (isPackaging && c.OnPackage) || (!isPackaging && c.OnRun) {
		for _, v := range c.Val {
			fs, err := f.UnpackFormatters(v.Formatters)
			if err != nil {
				return nil, err
			}

			o = append(o, fmt.Sprintf(v.Content, fs...))
		}
	}

	return o, nil
}

// QemuConfigVal represents an extra string and any formatters that should be applied to it.
type QemuConfigVal struct {
	Content    string   `yaml:"content"`
	Formatters []string `yaml:"formatters"`
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
