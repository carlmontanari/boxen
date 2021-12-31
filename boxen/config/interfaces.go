package config

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

type Bridge struct{}

type DataPlaneIntf struct {
	SocketConnectMap map[int]*SocketConnectPair `yaml:"socket_connect_map,omitempty"`
}

type SocketConnectPair struct {
	Connect int `yaml:"connect,omitempty"`
	Listen  int `yaml:"listen,omitempty"`
}
