package constants

// Version is the boxen version, set by the release build's linker flags.
var Version = "0.0.0"

const (
	DefaultLogLevel = "debug"

	// PackageBootLogFilename records guest serial output during image packaging.
	PackageBootLogFilename = "package.boot.log"
	// RunBootLogFilename records guest serial output during runtime.
	RunBootLogFilename = "boot.log"

	DefaultListenHost = "[::]"

	// DefaultBoxenListenPort, so fun, 98 111 120 == box in decimal ascii, so 98+111+120 = 329.
	DefaultBoxenListenPort = 10329

	DockerLinuxX86Platform = "linux/amd64"

	// ConsolePort is the telnet port of the first guest serial console; additional serial
	// ports follow sequentially.
	ConsolePort = 5001
	// MonitorPort is the QEMU human monitor port, reachable from inside the container.
	MonitorPort = 4001

	// DiskFilename is the packaged VM disk. Packaging modifies it directly; runtime only reads it
	// as the backing file of RunDiskFilename.
	DiskFilename = "disk.qcow2"
	// RunDiskFilename is the per-container qcow2 overlay the VM writes to at runtime, so the
	// packaged disk in the image layer is never copied up.
	RunDiskFilename = "disk.overlay.qcow2"
	// InstanceUUIDFilename stores the VM UUID generated for a container, so that a restarted
	// container presents the same system UUID to the guest.
	InstanceUUIDFilename = "instance.uuid"
)
