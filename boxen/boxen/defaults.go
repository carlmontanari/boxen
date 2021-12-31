package boxen

import (
	"github.com/carlmontanari/boxen/boxen/config"
)

func defaultTCPNatMap() map[int]int {
	return map[int]int{
		22:  21022,
		23:  21023,
		80:  21080,
		443: 21443,
		830: 21830,
	}
}

func zipDefaultTCPNats(platformDefaultNats []int) []*config.NatPortPair {
	nats := make([]*config.NatPortPair, 0)

	defaultNATMap := defaultTCPNatMap()

	for _, instancePort := range platformDefaultNats {
		hostPort, ok := defaultNATMap[instancePort]
		if !ok {
			continue
		}

		nats = append(nats, &config.NatPortPair{
			InstanceSide: instancePort,
			HostSide:     hostPort,
		})
	}

	return nats
}

func defaultUDPNatMap() map[int]int {
	return map[int]int{
		161: 31161,
	}
}

func zipDefaultUDPNats(platformDefaultNats []int) []*config.NatPortPair {
	nats := make([]*config.NatPortPair, 0)

	defaultNATMap := defaultUDPNatMap()

	for _, instancePort := range platformDefaultNats {
		hostPort, ok := defaultNATMap[instancePort]
		if !ok {
			continue
		}

		nats = append(nats, &config.NatPortPair{
			InstanceSide: instancePort,
			HostSide:     hostPort,
		})
	}

	return nats
}
