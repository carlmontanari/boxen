package config

import (
	"fmt"
	"os"
	"sync"

	"github.com/carlmontanari/boxen/boxen/util"

	"gopkg.in/yaml.v2"
)

const (
	// MAXINSTANCES is the maximum number of instances boxen can allocate.
	MAXINSTANCES = 255
	// MONITORPORTBASE is the starting port ID for qemu monitor ports.
	MONITORPORTBASE = 4000
	// SERIALPORTBASE is the starting port ID for serial ports.
	SERIALPORTBASE = 5000
	// SERIALPORTLOW is the first possible serial port ID.
	SERIALPORTLOW = 5001
	// SERIALPORTHI is the last possible serial port ID.
	SERIALPORTHI = 5999
	// MGMTNATLOW is the first possible "management NAT" port ID.
	MGMTNATLOW = 30001
	// MGMTNATHI is the last possible "management NAT" port ID.
	MGMTNATHI = 39999
	// SOCKETLISTENPORTLOW is the starting port ID for "data plane" listening ports.
	SOCKETLISTENPORTLOW = 40000
	// SOCKETLISTENPORTHI is the last possible port ID for "data plane" listening ports.
	SOCKETLISTENPORTHI = 65535
)

// Config is a struct representing boxen configuration data.
type Config struct {
	Options        *GlobalOptions       `yaml:"options,omitempty"`
	Instances      map[string]*Instance `yaml:"instances,omitempty"`
	InstanceGroups map[string][]string  `yaml:"groups,omitempty"`
	Platforms      map[string]*Platform `yaml:"platforms,omitempty"`
	lock           *sync.Mutex
}

// NewConfig returns a new instantiated config object.
func NewConfig() *Config {
	return &Config{
		&GlobalOptions{
			Credentials: NewDefaultCredentials(),
			Qemu: &Qemu{
				Acceleration:  util.AvailableAccel(),
				Binary:        util.GetQemuPath(),
				UseThickDisks: false,
			},
			Build: &Build{
				InstancePath: "",
				SourcePath:   "",
			},
		},
		make(map[string]*Instance),
		make(map[string][]string),
		make(map[string]*Platform),
		&sync.Mutex{},
	}
}

// NewPackageConfig returns a new instantiated config object specifically for use during "packaging"
// this is mostly the same as normal, but omits the global config options (which include qemu path
// and available acceleration).
func NewPackageConfig() *Config {
	return &Config{
		nil,
		make(map[string]*Instance),
		make(map[string][]string),
		make(map[string]*Platform),
		&sync.Mutex{},
	}
}

func expandConfig(cfg *Config) {
	// ensure we don't have nil sections of the config
	if cfg.Options == nil {
		cfg.Options = &GlobalOptions{}
		cfg.Options.Credentials = NewDefaultCredentials()
		cfg.Options.Qemu = &Qemu{
			Acceleration:  util.AvailableAccel(),
			Binary:        util.GetQemuPath(),
			UseThickDisks: false,
		}
		cfg.Options.Build = &Build{}
	}

	if cfg.Instances == nil {
		cfg.Instances = make(map[string]*Instance)
	}

	if cfg.InstanceGroups == nil {
		cfg.InstanceGroups = make(map[string][]string)
	}

	if cfg.Platforms == nil {
		cfg.Platforms = make(map[string]*Platform)
	}

	if cfg.lock == nil {
		cfg.lock = &sync.Mutex{}
	}
}

// NewConfigFromFile returns an instantiated Config object loaded from a YAML file.
func NewConfigFromFile(f string) (*Config, error) {
	yamlFile, err := os.ReadFile(f)

	if err != nil {
		return nil, err
	}

	cfg := &Config{}

	err = yaml.UnmarshalStrict(yamlFile, cfg)
	if err != nil {
		return nil, err
	}

	expandConfig(cfg)

	err = cfg.Validate()

	return cfg, err
}

func (c *Config) validateIDs() error {
	allocatedIDs := c.AllocatedInstanceIDs()

	uniqueIDs := util.IntSliceUniqify(allocatedIDs)

	if len(allocatedIDs) != len(uniqueIDs) {
		return fmt.Errorf("%w: one or more overlapping instance ids", util.ErrValidationError)
	}

	return nil
}

func (c *Config) validateMonitorPorts() error {
	allocatedMonitorPorts := c.AllocatedMonitorPorts()

	uniqueMonitorPorts := util.IntSliceUniqify(allocatedMonitorPorts)

	if len(allocatedMonitorPorts) != len(uniqueMonitorPorts) {
		return fmt.Errorf("%w: one or more overlapping monitor ports", util.ErrValidationError)
	}

	return nil
}

func (c *Config) validateSerialPorts() error {
	allocatedSerialPorts := c.AllocatedSerialPorts()

	uniqueSerialPorts := util.IntSliceUniqify(allocatedSerialPorts)

	if len(allocatedSerialPorts) != len(uniqueSerialPorts) {
		return fmt.Errorf("%w: one or more overlapping serial ports", util.ErrValidationError)
	}

	return nil
}

func (c *Config) validateNATPorts() error {
	allocatedNatPorts := c.AllocatedHostSideNatPorts()

	uniqueNatPorts := util.IntSliceUniqify(allocatedNatPorts)

	if len(allocatedNatPorts) != len(uniqueNatPorts) {
		return fmt.Errorf(
			"%w: one or more overlapping host side nat ports",
			util.ErrValidationError,
		)
	}

	return nil
}

func (c *Config) validateListenPorts() error {
	allocatedListenPorts := c.AllocatedDataPlaneListenPorts()

	uniqueListenPorts := util.IntSliceUniqify(allocatedListenPorts)

	if len(allocatedListenPorts) != len(uniqueListenPorts) {
		return fmt.Errorf("%w: one or more overlapping host listen ports", util.ErrValidationError)
	}

	return nil
}

// Validate provides basic configuration validation -- checking for things like duplicate device IDs
// and duplicate allocated ports.
func (c *Config) Validate() error {
	for _, f := range []func() error{
		c.validateIDs,
		c.validateMonitorPorts,
		c.validateSerialPorts,
		c.validateNATPorts,
		c.validateListenPorts} {
		err := f()
		if err != nil {
			return err
		}
	}

	return nil
}

// Dump the config to disk at path 'f'.
func (c *Config) Dump(f string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	y, err := yaml.Marshal(c)

	if err != nil {
		return err
	}

	err = os.WriteFile(f, y, util.FilePerms)

	return err
}

// AddInstance safely (with a lock) adds an instance to the config object Instances map.
func (c *Config) AddInstance(name string, instance *Instance) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.Instances[name] = instance
}

// DeleteInstance safely (with a lock) deletes an instance from the config object Instances map.
func (c *Config) DeleteInstance(name string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	delete(c.Instances, name)
}
