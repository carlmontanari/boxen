package config

// AllocatedInstanceIDs returns a slice of integers of all currently allocated instance IDs in the
// local boxen config.
func (c *Config) AllocatedInstanceIDs() []int {
	allocatedIDs := make([]int, 0)

	for _, data := range c.Instances {
		allocatedIDs = append(allocatedIDs, data.ID)
	}

	return allocatedIDs
}

// AllocatedMonitorPorts returns a slice of integers of all currently allocated monitor port IDs in
// the local boxen config.
func (c *Config) AllocatedMonitorPorts() []int {
	allocatedMonitorPorts := make([]int, 0)

	for _, data := range c.Instances {
		allocatedMonitorPorts = append(allocatedMonitorPorts, data.Hardware.MonitorPort)
	}

	return allocatedMonitorPorts
}

// AllocatedSerialPorts returns a slice of integers of all currently allocated serial port IDs in
// the local boxen config.
func (c *Config) AllocatedSerialPorts() []int {
	allocatedSerialPorts := make([]int, 0)

	for _, data := range c.Instances {
		allocatedSerialPorts = append(allocatedSerialPorts, data.Hardware.SerialPorts...)
	}

	return allocatedSerialPorts
}

// AllocatedHostSideNatPorts returns a slice of integers of all currently allocated "host side" nat
// ports in the local boxen config. These ports are the ephemeral range ports that get applied to
// the qemu hostfwd nat directives for the local virtual machines.
func (c *Config) AllocatedHostSideNatPorts() []int {
	allocatedNatPorts := make([]int, 0)

	for _, data := range c.Instances {
		if data.MgmtIntf != nil && data.MgmtIntf.Nat != nil {
			for _, nat := range data.MgmtIntf.Nat.TCP {
				allocatedNatPorts = append(allocatedNatPorts, nat.HostSide)
			}

			for _, nat := range data.MgmtIntf.Nat.UDP {
				allocatedNatPorts = append(allocatedNatPorts, nat.HostSide)
			}
		}
	}

	return allocatedNatPorts
}

// AllocatedDataPlaneListenPorts returns a slice of integers of all currently allocated "listen"
// ports in the local boxen config. These ports are the ephemeral range ports that get applied to
// the qemu udp listen ports for the "dataplane" ports of the virtual machines.
func (c *Config) AllocatedDataPlaneListenPorts() []int {
	allocatedListenPorts := make([]int, 0)

	for _, data := range c.Instances {
		if data.DataPlaneIntf != nil && data.DataPlaneIntf.SocketConnectMap != nil {
			for _, pair := range data.DataPlaneIntf.SocketConnectMap {
				allocatedListenPorts = append(allocatedListenPorts, pair.Listen)
			}
		}
	}

	return allocatedListenPorts
}
