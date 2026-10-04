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
	EnvClabQemuNicType        = "QEMU_NIC_TYPE"
	EnvClabQemuAdditionalArgs = "QEMU_ADDITIONAL_ARGS"

	// EnvClabUUID sets the VM system UUID, as in vrnetlab.
	EnvClabUUID = "UUID"

	// EnvClabUsername and EnvClabPassword are set by containerlab VM kinds alongside the run
	// flags; `boxen save` reads them because it is started with `docker exec`.
	EnvClabUsername = "USERNAME"
	EnvClabPassword = "PASSWORD"

	// StartupConfigFilePath is the default startup config path for containerlab VM kinds.
	StartupConfigFilePath = "/config/startup-config.cfg"

	// HealthFilePath is read by the container healthcheck; its first whitespace
	// separated field is the status code (0 == healthy), vrnetlab-compatible.
	HealthFilePath = "/health"

	HealthStatusBooting  = "1 booting"
	HealthStatusRunning  = "0 running"
	HealthStatusVMExited = "1 vm exited"
)
