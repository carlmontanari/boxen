package profile

import (
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
