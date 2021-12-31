package config

func (c *Config) AllocatedInstanceIDs() []int {
	allocatedIDs := make([]int, 0)

	for _, data := range c.Instances {
		allocatedIDs = append(allocatedIDs, data.ID)
	}

	return allocatedIDs
}

func (c *Config) AllocatedMonitorPorts() []int {
	allocatedMonitorPorts := make([]int, 0)

	for _, data := range c.Instances {
		allocatedMonitorPorts = append(allocatedMonitorPorts, data.Hardware.MonitorPort)
	}

	return allocatedMonitorPorts
}

func (c *Config) AllocatedSerialPorts() []int {
	allocatedSerialPorts := make([]int, 0)

	for _, data := range c.Instances {
		allocatedSerialPorts = append(allocatedSerialPorts, data.Hardware.SerialPorts...)
	}

	return allocatedSerialPorts
}

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
