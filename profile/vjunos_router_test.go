package profile

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"testing"

	boxenassets "github.com/carlmontanari/boxen/assets"
	boxenconstants "github.com/carlmontanari/boxen/constants"
	"go.yaml.in/yaml/v4"
)

func vjunosRouterProfile(t *testing.T) *Profile {
	t.Helper()

	data, err := boxenassets.Assets.ReadFile("profiles/juniper_vjunos-router.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p := &Profile{}
	if err = yaml.Unmarshal(data, p); err != nil {
		t.Fatal(err)
	}

	return p
}

func TestVJunosRouterQemu(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "")
	t.Setenv(boxenconstants.EnvClabMgmtMAC, "02:00:00:00:00:01")
	p := vjunosRouterProfile(t)
	disk := "vJunos-router-25.2R1.9.qcow2"
	if !regexp.MustCompile(p.DiskPatterns[0]).MatchString(disk) ||
		regexp.MustCompile(p.VersionPattern).FindString(disk) != "25.2R1.9" {
		t.Fatal("profile does not recognize the vJunos-router disk and version")
	}
	for _, packaging := range []bool{true, false} {
		args, err := QemuArgsFromProfile(p, packaging)
		if err != nil {
			t.Fatal(err)
		}
		for _, required := range []string{
			"host", "4", "5120",
			"if=virtio,file=disk.qcow2,format=qcow2", "type=1,product=VM-VMX,family=lab",
			"tap,id=p001,ifname=tap1,script=no,downscript=no",
			"tap,id=p096,ifname=tap96,script=no,downscript=no",
		} {
			if !slices.Contains(args, required) {
				t.Fatalf("packaging=%v: missing QEMU argument %q", packaging, required)
			}
		}
		if slices.Contains(args, "tap,id=mgmt,ifname=tap0,script=no,downscript=no") == packaging {
			t.Fatalf("packaging=%v: incorrect management networking: %v", packaging, args)
		}
		if !packaging && strings.Contains(strings.Join(args, " "), "hostfwd=") {
			t.Fatal("transparent management must not use port forwarding")
		}
		if slices.Contains(args, "file=config.img,format=raw,if=none,id=config_disk") != packaging {
			t.Fatalf("packaging=%v: configuration disk attached in the wrong phase", packaging)
		}
	}
}

func TestVJunosRouterManagementTemplates(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "")
	oldAddress, oldRoute := ipAddressShowCommand, ipRouteShowDefaultCommand
	t.Cleanup(func() {
		ipAddressShowCommand, ipRouteShowDefaultCommand = oldAddress, oldRoute
	})
	ipAddressShowCommand = func(context.Context, string) ([]byte, error) {
		return []byte(`[{"addr_info":[
			{"family":"inet","local":"192.0.2.10","prefixlen":24,"scope":"global"},
			{"family":"inet6","local":"2001:db8::10","prefixlen":64,"scope":"global"}
		]}]`), nil
	}
	ipRouteShowDefaultCommand = func(_ context.Context, family, _ string) ([]byte, error) {
		if family == "-4" {
			return []byte(`[{"gateway":"192.0.2.1"}]`), nil
		}

		return []byte(`[{"gateway":"2001:db8::1"}]`), nil
	}
	p := vjunosRouterProfile(t)
	for _, dhcp := range []string{"false", "true"} {
		t.Setenv(boxenconstants.EnvClabMgmtDHCP, dhcp)
		formatter := NewFormatters("admin", "admin@123", "router", "tc", p, false)
		var content strings.Builder
		for _, step := range p.Run.Process {
			if step.Type != StepTypeWrite {
				continue
			}
			rendered, err := formatter.RenderTemplate(step.Write.Content)
			if err != nil {
				t.Fatal(err)
			}
			content.WriteString(rendered)
			content.WriteByte('\n')
		}
		if dhcp == "true" {
			if !strings.Contains(content.String(), "family inet dhcp") ||
				strings.Contains(content.String(), "family inet address") {
				t.Fatal("DHCP provisioning includes a static management address")
			}

			continue
		}
		for _, required := range []string{
			"host-name router", "user admin class super-user", "admin@123",
			"family inet address 192.0.2.10/24", "family inet6 address 2001:db8::10/64",
			"route 0.0.0.0/0 next-hop 192.0.2.1", "route ::/0 next-hop 2001:db8::1",
		} {
			if !strings.Contains(content.String(), required) {
				t.Fatalf("missing rendered Junos command %q", required)
			}
		}
	}
	if !slices.ContainsFunc(p.Run.ConfigProcess, func(step Step) bool {
		return step.Type == StepTypeWrite && step.Write.ContentFromStartupConfig
	}) {
		t.Fatal("startup configuration is not loaded")
	}
}

func TestVJunosRouterPromptsBeforeAndAfterHostname(t *testing.T) {
	p := vjunosRouterProfile(t)
	for _, prompt := range []string{"root> ", "root@boxen-vjunos> "} {
		matched, err := p.Packaging.Process[2].ReadUntil.Until.Check([]byte(prompt))
		if err != nil || !matched {
			t.Fatalf("CLI prompt %q did not match: %v", prompt, err)
		}
	}
	for _, prompt := range []string{"root# ", "root@boxen-vjunos# "} {
		matched, err := p.Run.Process[4].ReadUntil.Until.Check([]byte(prompt))
		if err != nil || !matched {
			t.Fatalf("configuration prompt %q did not match: %v", prompt, err)
		}
	}
}

func TestVJunosRouterWaitsForJunosBeforeLogin(t *testing.T) {
	p := vjunosRouterProfile(t)
	for _, steps := range [][]Step{p.Packaging.Process, p.Run.Process} {
		if steps[0].Type != StepTypeReadUntil || steps[1].Type != StepTypePrompts {
			t.Fatal("login must wait for the nested Junos VM to boot")
		}
		for _, prompt := range steps[1].Prompts.Prompts[:2] {
			if !prompt.Hidden || prompt.Once {
				t.Fatal("login must send a return without waiting for echo and allow retries")
			}
		}
		for _, output := range []string{"login: ", "Last login: Tue Jun 24 on ttyu0"} {
			matched, err := steps[1].Prompts.Prompts[0].Prompt.Check([]byte(output))
			if err != nil || matched != (output == "login: ") {
				t.Fatalf("login prompt for %q matched=%v: %v", output, matched, err)
			}
		}
		for _, output := range []string{
			"Wind River Linux 9.0.0.20 qemux86-64 console\nqemux86-64 login: ",
			"FreeBSD/amd64 (boxen-vjunos) (ttyu0)\nlogin: ",
		} {
			matched, err := steps[0].ReadUntil.Until.Check([]byte(output))
			if err != nil || matched != strings.Contains(output, "FreeBSD/amd64") {
				t.Fatalf("boot gate for %q matched=%v: %v", output, matched, err)
			}
		}
	}
}
