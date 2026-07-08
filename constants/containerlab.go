package constants

const (
	EnvClabMgmtPassthrough = "CLAB_MGMT_PASSTHROUGH" //nolint: gosec
	EnvClabMgmtDHCP        = "CLAB_MGMT_DHCP"
	EnvClabMgmtMAC         = "CLAB_MGMT_MAC"
	EnvClabMgmtIntf        = "CLAB_MGMT_INTF"
	EnvClabIntfPrefix      = "CLAB_INTF_PREFIX"
	EnvClabIntfs           = "CLAB_INTFS"
	EnvClabBootDelay       = "BOOT_DELAY"

	// clab also has some env vars to set qemu values like memory/cpu/etc.

	EnvClabQemuMemory         = "QEMU_MEMORY"
	EnvClabQemuCPU            = "QEMU_CPU"
	EnvClabQemuSMP            = "QEMU_SMP"
	EnvClabQemuAdditionalArgs = "QEMU_ADDITIONAL_ARGS"

	StartupConfigFilePath = "/config/startup-config.cfg"

	// HealthFilePath is read by the container healthcheck; its first whitespace
	// separated field is the status code (0 == healthy), vrnetlab-compatible.
	HealthFilePath = "/health"

	HealthStatusBooting = "1 booting"
	HealthStatusRunning = "0 running"
)
