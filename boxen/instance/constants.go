package instance

const (
	// AccelKVM represents KVM acceleration.
	AccelKVM = "kvm"
	// AccelHVF represents HVF (Darwin) acceleration.
	AccelHVF = "hvf"
	// AccelHAX represents HAX (Intel Haxm) acceleration.
	AccelHAX = "hax"
	// AccelNone represents no acceleration.
	AccelNone = "none"
	// OptNone represents a literal string "none" to be used for some qemu command options.
	OptNone = "none"
	// OptMachinePc represents the Qemu "machine" type of "pc".
	OptMachinePc = "pc"
	// NicE1000 represents an E1000 virtual nic.
	NicE1000 = "e1000"
	// NicVirtio represents an virtio-net-pci virtual nic.
	NicVirtio = "virtio-net-pci"
)
