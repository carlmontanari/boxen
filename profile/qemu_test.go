package profile

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	"go.yaml.in/yaml/v4"
)

func TestQemuSerialBootLogs(t *testing.T) {
	p := testQemuProfile(false)
	p.VirtualMachine.SerialPortCount = 2
	for _, test := range []struct {
		packaging bool
		filename  string
	}{
		{packaging: true, filename: boxenconstants.PackageBootLogFilename},
		{packaging: false, filename: boxenconstants.RunBootLogFilename},
	} {
		args, err := QemuArgsFromProfile(p, test.packaging)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			"socket,id=serial0,host=0.0.0.0,port=5001,server=on,wait=off,telnet=on," +
				"logfile=" + test.filename + ",logappend=off",
			"socket,id=serial1,host=0.0.0.0,port=5002,server=on,wait=off,telnet=on," +
				"logfile=" + test.filename + ".2,logappend=off",
			"chardev:serial0", "chardev:serial1",
		} {
			if !slices.Contains(args, want) {
				t.Fatalf("packaging=%v: missing %q in %v", test.packaging, want, args)
			}
		}
	}
}

func TestQemuDiskInterface(t *testing.T) {
	for _, test := range []struct {
		name, setting, expected string
	}{
		{name: "default", expected: "ide"},
		{name: "ide", setting: "  diskInterface: ide\n", expected: "ide"},
		{name: "virtio", setting: "  diskInterface: virtio\n", expected: "virtio"},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := &Profile{}
			data := "virtualMachine:\n  nicType: virtio-net-pci\n  nicPerBus: 26\n" + test.setting
			if err := yaml.Unmarshal([]byte(data), p); err != nil {
				t.Fatal(err)
			}
			for _, packaging := range []bool{true, false} {
				args, err := QemuArgsFromProfile(p, packaging)
				if err != nil {
					t.Fatal(err)
				}
				idx := slices.Index(args, "-drive")
				disk := boxenconstants.RunDiskFilename
				if packaging {
					disk = boxenconstants.DiskFilename
				}
				expected := "if=" + test.expected + ",file=" + disk + ",format=qcow2"
				if idx < 0 || idx+1 >= len(args) || args[idx+1] != expected {
					t.Fatalf("packaging=%v: expected -drive %q, got %v", packaging, expected, args)
				}
			}
		})
	}
}

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

	if args[3] != "tap,id=mgmt,ifname=tap0,script=no,downscript=no" {
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

func TestQemuDataNICsAlwaysTap(t *testing.T) {
	p := &Profile{
		Name: "test",
		VirtualMachine: &VirtualMachine{
			NicType:   "virtio-net-pci",
			NicCount:  1,
			NicPerBus: 26,
		},
	}

	args := qemuDataNICs(p)

	if len(args) != 4 {
		t.Fatalf("expected 4 args for a single data nic, got %d: %v", len(args), args)
	}

	if args[0] != device {
		t.Fatalf("expected %q, got %q", device, args[0])
	}

	if !strings.HasPrefix(args[1], "virtio-net-pci,netdev=p001,bus=pci.1,addr=0x2,mac=") {
		t.Fatalf("unexpected data device args, got %q", args[1])
	}

	if args[2] != "-netdev" {
		t.Fatalf("expected -netdev, got %q", args[2])
	}

	// the tap is always created with script=no regardless of whether the container
	// interface exists yet; the boxen tc service wires it up later (incl. hotplug)
	if args[3] != "tap,id=p001,ifname=tap1,script=no,downscript=no" {
		t.Fatalf("data netdev should always be a script=no tap, got %q", args[3])
	}
}

func TestQemuArgsOverridesAndExtras(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "")

	p := &Profile{
		Name: "test",
		VirtualMachine: &VirtualMachine{
			DiskInterface: "virtio",
			NicType:       "virtio-net-pci",
			NicCount:      1,
			NicPerBus:     26,
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

	// the override replaces the default disk section entirely, and at runtime its disk reference
	// points at the overlay
	if !slices.Contains(runArgs, "if=none,file=disk.overlay.qcow2,format=qcow2,id=drive0") {
		t.Fatalf("expected disk override content in run args, got %v", runArgs)
	}

	if slices.ContainsFunc(runArgs, func(arg string) bool {
		return strings.HasPrefix(arg, "if=virtio,")
	}) {
		t.Fatalf("generated disk args should be replaced by the override, got %v", runArgs)
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

	// packaging writes the packaged disk itself
	if !slices.Contains(packageArgs, "if=none,file=disk.qcow2,format=qcow2,id=drive0") {
		t.Fatalf("expected packaged disk in packaging args, got %v", packageArgs)
	}
}

func TestUseRunDisk(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "bare argument",
			args: []string{"-drive", "if=none,file=disk.qcow2,format=qcow2", "-hda", "disk.qcow2"},
			want: []string{
				"-drive", "if=none,file=disk.overlay.qcow2,format=qcow2",
				"-hda", "disk.overlay.qcow2",
			},
		},
		{
			name: "path qualified",
			args: []string{
				"-drive", "if=none,file=/boxen/disk.qcow2,format=qcow2,id=drive0",
				"-drive", "file=./disk.qcow2",
			},
			want: []string{
				"-drive", "if=none,file=disk.overlay.qcow2,format=qcow2,id=drive0",
				"-drive", "file=disk.overlay.qcow2",
			},
		},
		{
			name: "other files untouched",
			args: []string{"-drive", "file=disk.qcow2.bak,format=qcow2", "-cdrom", "config.iso"},
			want: []string{"-drive", "file=disk.qcow2.bak,format=qcow2", "-cdrom", "config.iso"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := useRunDisk(test.args); !slices.Equal(got, test.want) {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestQemuAdditionalArgsRunDisk(t *testing.T) {
	t.Setenv(
		boxenconstants.EnvClabQemuAdditionalArgs,
		"-drive if=none,file=disk.qcow2,format=qcow2",
	)

	p := testQemuProfile(false)

	args, err := QemuArgsFromProfile(p, false)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Contains(args, "if=none,file=disk.overlay.qcow2,format=qcow2") {
		t.Fatalf("expected additional args disk reference rewritten to overlay, got %v", args)
	}
}

func TestQemuInstanceUUID(t *testing.T) {
	p := testQemuProfile(false)
	p.InstanceUUID = "123e4567-e89b-12d3-a456-426614174000"

	runArgs, err := QemuArgsFromProfile(p, false)
	if err != nil {
		t.Fatal(err)
	}

	if runArgs[3] != p.InstanceUUID {
		t.Fatalf("expected run uuid %q, got %v", p.InstanceUUID, runArgs[:4])
	}

	packageArgs, err := QemuArgsFromProfile(p, true)
	if err != nil {
		t.Fatal(err)
	}

	if packageArgs[3] == p.InstanceUUID || packageArgs[3] == "" {
		t.Fatalf("expected a random packaging uuid, got %v", packageArgs[:4])
	}
}

func TestQemuNicTypeEnv(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "")
	t.Setenv(boxenconstants.EnvClabQemuNicType, "vmxnet3")

	p := testQemuProfile(false)
	p.VirtualMachine.NicCount = 1
	p.VirtualMachine.NicPerBus = 26

	if got := qemuMgmtNIC(p, false)[1]; !strings.HasPrefix(got, "vmxnet3,") {
		t.Fatalf("management nic should use QEMU_NIC_TYPE, got %q", got)
	}

	if got := qemuDataNICs(p)[1]; !strings.HasPrefix(got, "vmxnet3,") {
		t.Fatalf("data nic should use QEMU_NIC_TYPE, got %q", got)
	}
}

func TestQemuAdditionalArgsWhitespace(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabQemuAdditionalArgs, "  -machine  pc\t-no-reboot ")

	args, err := QemuArgsFromProfile(testQemuProfile(false), false)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(args[len(args)-3:], []string{"-machine", "pc", "-no-reboot"}) {
		t.Fatalf("expected whitespace separated additional args, got %v", args[len(args)-3:])
	}

	if slices.Contains(args, "") {
		t.Fatalf("additional args produced an empty argument: %v", args)
	}
}

func TestQemuMgmtNICLegacyDefaultNatPorts(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "")

	p := testQemuProfile(false)
	p.VirtualMachine.NatPorts = nil

	args := qemuMgmtNIC(p, false)

	for _, want := range []string{
		"hostfwd=tcp:0.0.0.0:22-10.0.0.15:22",
		"hostfwd=udp:0.0.0.0:161-10.0.0.15:161",
		"hostfwd=tcp:0.0.0.0:830-10.0.0.15:830",
		"hostfwd=tcp:0.0.0.0:57400-10.0.0.15:57400",
	} {
		if !strings.Contains(args[3], want) {
			t.Fatalf("legacy management netdev missing default %q, got %q", want, args[3])
		}
	}
}

func TestQemuBridgeBoundaries(t *testing.T) {
	for _, count := range []uint16{25, 26, 27, 52, 70} {
		p := testQemuProfile(false)
		p.VirtualMachine.NicCount = count
		p.VirtualMachine.NicPerBus = 26
		args, err := QemuArgsFromProfile(p, false)
		if err != nil {
			t.Fatal(err)
		}
		for index := 1; index <= int(count); index++ {
			bridge := fmt.Sprintf("pci-bridge,chassis_nr=%d,id=pci.%d",
				index/26+1, index/26+1)
			if !slices.Contains(args, bridge) {
				t.Fatalf("NIC %d of %d references a missing bridge: %s", index, count, bridge)
			}
			if !slices.Contains(args,
				fmt.Sprintf("tap,id=p%03d,ifname=tap%d,script=no,downscript=no", index, index)) {
				t.Fatalf("missing tap%d", index)
			}
		}
	}
}
