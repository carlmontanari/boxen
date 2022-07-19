package config

import "fmt"

type MgmtIntf struct {
	Nat    *Nat
	Bridge *Bridge
}

type Nat struct {
	TCP []*NatPortPair `yaml:"tcp,omitempty"`
	UDP []*NatPortPair `yaml:"udp,omitempty"`
}

type NatPortPair struct {
	InstanceSide int `yaml:"instance_side,omitempty"`
	HostSide     int `yaml:"host_side,omitempty"`
}

func (n *NatPortPair) String() string {
	return fmt.Sprintf("(host)%d<->%d(instance)", n.HostSide, n.InstanceSide)
}

type Bridge struct{}

type DataPlaneIntf struct {
	SocketConnectMap map[int]*SocketConnectPair `yaml:"socket_connect_map,omitempty"`
}

type SocketConnectPair struct {
	Connect int `yaml:"connect,omitempty"`
	Listen  int `yaml:"listen,omitempty"`
}
