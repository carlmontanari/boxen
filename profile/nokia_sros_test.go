package profile

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	boxenconstants "github.com/carlmontanari/boxen/constants"
)

const srosInstanceMAC = "02:aa:bb:cc:dd:00"

// srosQemuArgs returns the qemu args of the SR OS profile for the given variant and management
// values.
func srosQemuArgs(
	t *testing.T,
	variant string,
	management *managementFormatters,
	isPackaging bool,
) []string {
	t.Helper()
	t.Setenv(boxenconstants.EnvClabMgmtMAC, "52:54:00:00:00:01")

	p := loadEmbeddedProfile(t, "nokia_sros")

	if err := p.ApplyVariant(variant); err != nil {
		t.Fatal(err)
	}

	p.InstanceMAC = srosInstanceMAC

	f := NewFormatters("", "", "sr1", "", p, isPackaging)
	f.management = management

	args, err := QemuArgsFromProfile(p, isPackaging, f)
	if err != nil {
		t.Fatal(err)
	}

	return args
}

func srosSMBIOS(t *testing.T, args []string) string {
	t.Helper()

	idx := slices.Index(args, "-smbios")
	if idx < 0 || idx+1 >= len(args) {
		t.Fatalf("no -smbios in %v", args)
	}

	return args[idx+1]
}

func TestSrosSMBIOS(t *testing.T) {
	ipv4 := &managementFormatters{ipv4: "192.0.2.2/24", ipv4Gateway: "192.0.2.1"}
	ipv6 := &managementFormatters{ipv6: "2001:db8::2/64", ipv6Gateway: "2001:db8::1"}

	const tail = " system-base-mac=" + srosInstanceMAC +
		" slot=A chassis=sr-1 card=cpm-1 mda/1=me12-100gb-qsfp28"

	for _, test := range []struct {
		name       string
		management *managementFormatters
		want       string
	}{
		{
			name:       "ipv4",
			management: ipv4,
			want: "type=1,product=TIMOS:license-file=cf1:/license.txt " +
				"primary-config=cf3:/config.cfg address=192.0.2.2/24@active " +
				"static-route=0.0.0.0/0@192.0.2.1" + tail,
		},
		{
			name:       "ipv6",
			management: ipv6,
			want: "type=1,product=TIMOS:license-file=cf1:/license.txt " +
				"primary-config=cf3:/config.cfg address=2001:db8::2/64@active " +
				"static-route=::/0@2001:db8::1" + tail,
		},
		{
			name:       "dual-stack",
			management: defaultManagementFormatters(),
			want: "type=1,product=TIMOS:license-file=cf1:/license.txt " +
				"primary-config=cf3:/config.cfg address=10.0.0.15/24@active " +
				"address=2001:db8::2/64@active static-route=0.0.0.0/0@10.0.0.2 " +
				"static-route=::/0@2001:db8::1" + tail,
		},
		{
			name:       "dhcp",
			management: dhcpManagementFormatters(),
			want: "type=1,product=TIMOS:license-file=cf1:/license.txt " +
				"primary-config=cf3:/config.cfg" + tail,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := srosQemuArgs(t, "", test.management, false)

			if got := srosSMBIOS(t, args); got != test.want {
				t.Fatalf("got  %q\nwant %q", got, test.want)
			}

			// the license and startup config disk
			idx := slices.Index(args, "if=virtio,index=1,format=raw,readonly=on,file=fat:/tftpboot")
			if idx < 1 || args[idx-1] != "-drive" {
				t.Fatalf("no license disk in %v", args)
			}
		})
	}
}

func TestSrosPackagingArgs(t *testing.T) {
	args := srosQemuArgs(t, "", defaultManagementFormatters(), true)

	want := "type=1,product=TIMOS:slot=A chassis=sr-1 card=cpm-1 mda/1=me12-100gb-qsfp28"
	if got := srosSMBIOS(t, args); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	if slices.ContainsFunc(args, func(arg string) bool { return strings.Contains(arg, "fat:") }) {
		t.Fatalf("packaging has no license disk, got %v", args)
	}
}

func TestSrosVariants(t *testing.T) {
	for _, test := range []struct {
		variant    string
		wantMemory string
		wantSFM    bool
		wantMAC    bool
	}{
		{variant: "sr-1", wantMemory: "5120", wantMAC: true},
		{variant: "SR-1s", wantMemory: "6144", wantMAC: true},
		{variant: "ixr-ec", wantMemory: "4096", wantSFM: true, wantMAC: true},
		{variant: "vsr-i", wantMemory: "8192", wantMAC: true},
		// containerlab components of an IXR chassis
		{
			variant:    "cpu=2 ram=6 max_nics=8 chassis=ixr-r6 slot=A card=cpiom-ixr-r6",
			wantMemory: "6144",
			wantSFM:    true,
			wantMAC:    true,
		},
		// a custom variant may bring its own base mac
		{
			variant:    "chassis=sr-1 slot=A card=cpm-1 system-base-mac=0c:00:00:00:00:00",
			wantMemory: "4096",
		},
	} {
		t.Run(test.variant, func(t *testing.T) {
			t.Setenv(boxenconstants.EnvClabQemuMemory, "")

			args := srosQemuArgs(t, test.variant, defaultManagementFormatters(), false)

			if idx := slices.Index(args, "-m"); idx < 0 || args[idx+1] != test.wantMemory {
				t.Errorf("expected -m %s in %v", test.wantMemory, args)
			}

			sfm := slices.Contains(
				args,
				"tap,ifname=sfm,model=virtio-net-pci,script=no,downscript=no",
			)
			if sfm != test.wantSFM || slices.Contains(args, "-nic") != test.wantSFM {
				t.Errorf("sfm nic: got %v, want %v", sfm, test.wantSFM)
			}

			if slices.Contains(args, "") {
				t.Errorf("empty qemu argument in %v", args)
			}

			smbios := srosSMBIOS(t, args)
			if strings.Contains(smbios, srosInstanceMAC) != test.wantMAC ||
				strings.Count(smbios, "system-base-mac=") != 1 {
				t.Errorf("unexpected base mac in %q", smbios)
			}
		})
	}
}

func renderSrosRunConfig(t *testing.T, p *Profile, username, password string) string {
	t.Helper()

	f := NewFormatters(username, password, "sr1", "", p, false)
	f.management = defaultManagementFormatters()

	var out strings.Builder

	for idx := range p.Run.Process {
		step := &p.Run.Process[idx]
		if step.Type != StepTypeWrite {
			continue
		}

		content, err := f.RenderTemplate(step.Write.Content)
		if err != nil {
			t.Fatal(err)
		}

		out.WriteString(content + "\n")
	}

	return out.String()
}

func TestSrosRunConfig(t *testing.T) {
	p := loadEmbeddedProfile(t, "nokia_sros")

	if err := p.ApplyVariant("sr-1s"); err != nil {
		t.Fatal(err)
	}

	p.ResolvedVersion = "26.7.R1"

	config := renderSrosRunConfig(t, p, "", "")

	for _, want := range []string{
		`configure system name "sr1"`,
		`local-user user "boxen" password "Console123!"`,
		"configure system management-interface netconf listen admin-state enable",
		"configure card 1 card-type xcm-1s\n",
		"power-module 4 power-module-type ps-a-dc-6000\n",
		"configure card 1 mda 1 mda-type s36-100gb-qsfp28\n",
		"configure system grpc rib-api admin-state enable\n",
		"edit-config private\n",
		"commit\n",
		"admin save\n",
	} {
		if !strings.Contains(config, want) {
			t.Errorf("missing %q in:\n%s", want, config)
		}
	}

	// containerlab does not pass credentials for SR OS, which keeps admin/admin
	if strings.Contains(config, `password ""`) ||
		strings.Contains(config, `user "admin" password`) {
		t.Errorf("unexpected credentials in:\n%s", config)
	}

	for line := range strings.SplitSeq(config, "\n") {
		if strings.TrimSpace(line) != line {
			t.Errorf("line with surrounding whitespace: %q", line)
		}
	}

	p.ResolvedVersion = "23.10.R2"

	config = renderSrosRunConfig(t, p, "labuser", "lab-pass")

	for _, want := range []string{
		"configure system management-interface netconf admin-state enable",
		`local-user user "labuser" password "lab-pass"`,
		`local-user user "labuser" console member ["administrative"]`,
	} {
		if !strings.Contains(config, want) {
			t.Errorf("missing %q in:\n%s", want, config)
		}
	}

	config = renderSrosRunConfig(t, p, "admin", "lab-pass")
	if !strings.Contains(config, `local-user user "admin" password "lab-pass"`) {
		t.Errorf("expected the admin password to be set in:\n%s", config)
	}

	// 7250 IXR chassis have no gRPC RIB API
	if err := p.ApplyVariant("cpu=2 ram=4 chassis=ixr-r6 slot=A card=cpiom-ixr-r6"); err != nil {
		t.Fatal(err)
	}

	config = renderSrosRunConfig(t, p, "", "")
	if strings.Contains(config, "rib-api") || !strings.Contains(config, "gnmi auto-config-save") {
		t.Errorf("unexpected gRPC configuration for an IXR chassis in:\n%s", config)
	}
}

// TestSrosConsoleLogin checks that the console automation does not depend on the containerlab
// credentials, which the run process changes.
func TestSrosConsoleLogin(t *testing.T) {
	p := loadEmbeddedProfile(t, "nokia_sros")

	for _, steps := range [][]Step{p.Run.Process, p.Run.SaveProcess} {
		for _, step := range steps {
			for _, prompt := range step.Prompts.Prompts {
				if strings.Contains(prompt.Response, "{{") {
					t.Errorf("console login uses templated credentials: %q", prompt.Response)
				}
			}
		}
	}
}

func TestSrosLicenseUUID(t *testing.T) {
	p := loadEmbeddedProfile(t, "nokia_sros")

	const uuid = "0a1b2c3d-4e5f-6071-8293-a4b5c6d7e8f9"

	// the layout of an SR OS license file: comments, then "<uuid> <record>" lines
	path := filepath.Join(t.TempDir(), "license.txt")

	content := "#\n# 26.x license for 11111111-2222-3333-4444-555555555555\n#\n" +
		uuid + " cmVjb3Jk\n\n#\n# 25.x\n#\n" +
		"ffffffff-ffff-ffff-ffff-ffffffffffff cmVjb3Jk"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	source := *p.Run.UUIDFrom
	source.File = path

	got, ok, err := source.Match()
	if err != nil || !ok || got != uuid {
		t.Fatalf("got %q %v %v, want %q", got, ok, err, uuid)
	}
}
