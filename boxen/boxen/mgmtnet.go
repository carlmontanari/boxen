package boxen

import "github.com/carlmontanari/boxen/boxen/config"

const (
	// host port base number, instance ports are incremented from this value
	hostPortBase = 48000
)

// zipPlatformProfileNats receives the lists of TCP and UDP ports that are defined in a platform profile yaml.
// For each received tcp/udp instance port a host port is allocated sequentially from the hostPortBase port number.
// These port mappings are used in packaging workflow (e.g. when building for containerlab)
func zipPlatformProfileNats(platformTCPPorts, platformUDPPorts []int) (tcpNats, udpNats []*config.NatPortPair) {
	tcpPortsLen := len(platformTCPPorts)
	udpPortsLen := len(platformUDPPorts)

	tcpNats = make([]*config.NatPortPair, 0, tcpPortsLen)
	udpNats = make([]*config.NatPortPair, 0, udpPortsLen)

	for i, tcp := range platformTCPPorts {
		tcpNats = append(tcpNats, &config.NatPortPair{
			InstanceSide: tcp,
			HostSide:     hostPortBase + i,
		})
	}

	for i, udp := range platformUDPPorts {
		udpNats = append(udpNats, &config.NatPortPair{
			InstanceSide: udp,
			HostSide:     hostPortBase + tcpPortsLen + i,
		})
	}

	return
}
