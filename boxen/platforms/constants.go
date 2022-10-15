package platforms

const (
	VendorCisco      = "cisco"
	VendorArista     = "arista"
	VendorJuniper    = "juniper"
	VendorPaloAlto   = "paloalto"
	VendorIPInfusion = "ipinfusion"
	VendorCheckpoint = "checkpoint"

	PlatformAristaVeos           = "veos"
	PlatformCiscoCsr1000v        = "csr1000v"
	PlatformCiscoXrv9k           = "xrv9k"
	PlatformCiscoN9kv            = "n9kv"
	PlatformJuniperVsrx          = "vsrx"
	PlatformPaloAltoPanos        = "panos"
	PlatformIPInfusionOcNOS      = "ocnos"
	PlatformCheckpointCloudguard = "cloudguard"

	PlatformTypeAristaVeos           = "arista_veos"
	PlatformTypeCiscoCsr1000v        = "cisco_csr1000v"
	PlatformTypeCiscoXrv9k           = "cisco_xrv9k"
	PlatformTypeCiscoN9kv            = "cisco_n9kv"
	PlatformTypeJuniperVsrx          = "juniper_vsrx"
	PlatformTypePaloAltoPanos        = "paloalto_panos"
	PlatformTypeIPInfusionOcNOS      = "ipinfusion_ocnos"
	PlatformTypeCheckpointCloudguard = "checkpoint_cloudguard"

	NicE1000  = "e1000"
	NicVirtio = "virtio-net-pci"

	AccelKVM  = "kvm"
	AccelHVF  = "hvf"
	AccelHAX  = "hax"
	AccelNone = "none"
)
