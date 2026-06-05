package profile

import (
	"slices"
	"strings"
	"testing"

	boxenconstants "github.com/carlmontanari/boxen/constants"
)

func TestQemuMgmtNICLegacyNat(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "")

	args := qemuMgmtNIC(testQemuProfile(false), false)

	if !strings.Contains(args[3], "user,id=mgmt") {
		t.Fatalf("management netdev should use user networking, got %q", args[3])
	}

	if !strings.Contains(args[3], "hostfwd=tcp:0.0.0.0:22-10.0.0.15:22") {
		t.Fatalf("management netdev missing NAT hostfwd, got %q", args[3])
	}
}

func TestQemuMgmtNICProfilePassthrough(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "")
	t.Setenv(boxenconstants.EnvClabMgmtMAC, "02:00:00:00:00:01")

	args := qemuMgmtNIC(testQemuProfile(true), false)

	if !strings.Contains(args[1], "mac=02:00:00:00:00:01") {
		t.Fatalf("management device missing MAC, got %q", args[1])
	}

	if args[3] != "tap,id=mgmt,ifname=tap0,script=/etc/tc-tap-mgmt-ifup,downscript=no" {
		t.Fatalf("management netdev should use tap0, got %q", args[3])
	}
}

func TestQemuMgmtNICEnvPassthroughOverrideTrue(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "true")
	t.Setenv(boxenconstants.EnvClabMgmtMAC, "02:00:00:00:00:02")

	args := qemuMgmtNIC(testQemuProfile(false), false)

	if !strings.Contains(args[3], "tap,id=mgmt,ifname=tap0") {
		t.Fatalf("management netdev should use tap0, got %q", args[3])
	}
}

func TestQemuMgmtNICEnvPassthroughOverrideFalse(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "false")

	args := qemuMgmtNIC(testQemuProfile(true), false)

	if !strings.Contains(args[3], "user,id=mgmt") {
		t.Fatalf("management netdev should use user networking, got %q", args[3])
	}
}

func TestQemuMgmtNICPackagingFallback(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "true")

	args := qemuMgmtNIC(testQemuProfile(true), true)

	if !strings.Contains(args[3], "user,id=mgmt") {
		t.Fatalf("packaging netdev should use user networking, got %q", args[3])
	}
}

func testQemuProfile(managementPassthrough bool) *Profile {
	return &Profile{
		Name: "test",
		VirtualMachine: &VirtualMachine{
			NicType:               "virtio-net-pci",
			ManagementPassthrough: managementPassthrough,
			NatPorts: []NatPort{
				{
					Type:      NatTypeTCP,
					LocalPort: 22,
				},
			},
		},
	}
}

func TestQemuArgsOverridesAndExtras(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "")

	p := &Profile{
		Name: "test",
		VirtualMachine: &VirtualMachine{
			NicType:   "virtio-net-pci",
			NicCount:  1,
			NicPerBus: 26,
			Overrides: map[string][]QemuConfigField{
				disk: {
					{
						OnPackage: true,
						OnRun:     true,
						Val: []QemuConfigVal{
							{Content: "-drive"},
							{Content: "if=none,file=disk.qcow2,format=qcow2,id=drive0"},
						},
					},
				},
			},
			Extras: []QemuConfigField{
				{
					OnPackage: true,
					OnRun:     true,
					Val: []QemuConfigVal{
						{Content: "-bios"},
						{Content: "OVMF.fd"},
					},
				},
				{
					OnPackage: true,
					OnRun:     false,
					Val: []QemuConfigVal{
						{Content: "-cdrom"},
						{Content: "config.iso"},
					},
				},
			},
		},
	}

	runArgs, err := QemuArgsFromProfile(p, false)
	if err != nil {
		t.Fatalf("building run qemu args failed: %v", err)
	}

	// the override replaces the default disk section entirely
	if !slices.Contains(runArgs, "if=none,file=disk.qcow2,format=qcow2,id=drive0") {
		t.Fatalf("expected disk override content in run args, got %v", runArgs)
	}

	if slices.Contains(runArgs, "if=ide,file=disk.qcow2,format=qcow2") {
		t.Fatalf("default disk args should be replaced by the override, got %v", runArgs)
	}

	// an onRun extra is appended in run mode
	if !slices.Contains(runArgs, "-bios") || !slices.Contains(runArgs, "OVMF.fd") {
		t.Fatalf("expected onRun extra (-bios OVMF.fd) in run args, got %v", runArgs)
	}

	// an onPackage-only extra is gated out in run mode
	if slices.Contains(runArgs, "config.iso") {
		t.Fatalf("onPackage-only extra should be absent in run args, got %v", runArgs)
	}

	packageArgs, err := QemuArgsFromProfile(p, true)
	if err != nil {
		t.Fatalf("building packaging qemu args failed: %v", err)
	}

	// the onPackage-only extra is present in packaging mode
	if !slices.Contains(packageArgs, "-cdrom") || !slices.Contains(packageArgs, "config.iso") {
		t.Fatalf("expected onPackage extra in packaging args, got %v", packageArgs)
	}
}
