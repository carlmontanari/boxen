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
	// ConsoleHost is the loopback address the console listens on; it is spelled out rather than
	// using localhost, which can resolve to IPv6 where nothing listens.
	ConsoleHost = "127.0.0.1"
)
