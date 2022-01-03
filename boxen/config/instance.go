package config

// Instance is a struct that represents a qemu virtual machine instance in the boxen configuration.
type Instance struct {
	Name          string         `yaml:"name"`
	PlatformType  string         `yaml:"platform_type"`
	Disk          string         `yaml:"source_disk"`
	ID            int            `yaml:"id,omitempty"`
	PID           int            `yaml:"pid,omitempty"`
	Profile       string         `yaml:"profile,omitempty"`
	Credentials   *Credentials   `yaml:"credentials,omitempty"`
	Hardware      *Hardware      `yaml:"hardware,omitempty"`
	MgmtIntf      *MgmtIntf      `yaml:"mgmt_interface,omitempty"`
	DataPlaneIntf *DataPlaneIntf `yaml:"data_plane_interfaces,omitempty"`
	Advanced      *Advanced      `yaml:"advanced,omitempty"`
	BootDelay     int            `yaml:"boot-delay,omitempty"`
	StartupConfig string         `yaml:"startup-config,omitempty"`
}
