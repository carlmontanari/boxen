package constants

const (
	// Version is the boxen version, set in ci.
	Version = "0.0.0"
)

const (
	DefaultLogLevel = "debug"

	DefaultListenHost = "[::]"

	// DefaultBoxenListenPort, so fun, 98 111 120 == box in decimal ascii, so 98+111+120 = 329.
	DefaultBoxenListenPort = 10329

	DockerLinuxX86Platform = "linux/amd64"
)
