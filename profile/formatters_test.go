package profile

import (
	"context"
	"testing"

	boxenconstants "github.com/carlmontanari/boxen/constants"
)

const mgmtTemplate = `{{ .mgmtIPv4 }} {{ .mgmtIPv4Address }} {{ .mgmtIPv4PrefixLen }} ` +
	`{{ .mgmtIPv4Network }} {{ .mgmtIPv4Gateway }} {{ .mgmtIPv6 }} {{ .mgmtIPv6Address }} ` +
	`{{ .mgmtIPv6PrefixLen }} {{ .mgmtIPv6Network }} {{ .mgmtIPv6Gateway }}`

func TestManagementFormattersDefault(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "")

	f := NewFormatters("", "", "", "", testQemuProfile(true), true)

	actual, err := f.RenderTemplate(mgmtTemplate)
	if err != nil {
		t.Fatalf("rendering management formatters failed: %v", err)
	}

	expected := "10.0.0.15/24 10.0.0.15 24 10.0.0.0/24 10.0.0.2 " +
		"2001:db8::2/64 2001:db8::2 64 2001:db8::/64 2001:db8::1"

	if actual != expected {
		t.Fatalf("management formatters mismatch, got %q, want %q", actual, expected)
	}
}

func TestManagementFormattersRuntime(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "true")

	oldAddressShowCommand := ipAddressShowCommand
	oldRouteShowDefaultCommand := ipRouteShowDefaultCommand

	t.Cleanup(func() {
		ipAddressShowCommand = oldAddressShowCommand
		ipRouteShowDefaultCommand = oldRouteShowDefaultCommand
	})

	ipAddressShowCommand = func(_ context.Context, intf string) ([]byte, error) {
		if intf != "eth0" {
			t.Fatalf("unexpected management interface %q", intf)
		}

		return []byte(`[
			{
				"addr_info": [
					{"family": "inet", "local": "172.20.20.10", "prefixlen": 24, "scope": "global"},
					{"family": "inet6", "local": "2001:db8:20::10", "prefixlen": 64, "scope": "global"}
				]
			}
		]`), nil
	}

	ipRouteShowDefaultCommand = func(_ context.Context, family, intf string) ([]byte, error) {
		if intf != "eth0" {
			t.Fatalf("unexpected management interface %q", intf)
		}

		switch family {
		case "-4":
			return []byte(`[{"gateway": "172.20.20.1"}]`), nil
		case "-6":
			return []byte(`[{"gateway": "2001:db8:20::1"}]`), nil
		default:
			t.Fatalf("unexpected route family %q", family)
		}

		return nil, nil
	}

	f := NewFormatters("", "", "", "", testQemuProfile(false), false)

	actual, err := f.RenderTemplate(mgmtTemplate)
	if err != nil {
		t.Fatalf("rendering management formatters failed: %v", err)
	}

	expected := "172.20.20.10/24 172.20.20.10 24 172.20.20.0/24 172.20.20.1 " +
		"2001:db8:20::10/64 2001:db8:20::10 64 2001:db8:20::/64 2001:db8:20::1"

	if actual != expected {
		t.Fatalf("management formatters mismatch, got %q, want %q", actual, expected)
	}
}

func TestRenderTemplateExtraFiles(t *testing.T) {
	p := &Profile{
		Name:           "test",
		ExtraFiles:     []string{"/some/dir/license.lic", "startup.cfg"},
		VirtualMachine: &VirtualMachine{},
	}

	f := NewFormatters("", "", "", "", p, true)

	actual, err := f.RenderTemplate(`{{ index .extraFiles 0 }} {{ index .extraFiles 1 }}`)
	if err != nil {
		t.Fatalf("rendering extraFiles template failed: %v", err)
	}

	expected := "license.lic startup.cfg"

	if actual != expected {
		t.Fatalf("extraFiles mismatch, got %q, want %q", actual, expected)
	}
}

func TestRenderTemplateManagementFormatters(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "true")

	oldAddressShowCommand := ipAddressShowCommand
	oldRouteShowDefaultCommand := ipRouteShowDefaultCommand

	t.Cleanup(func() {
		ipAddressShowCommand = oldAddressShowCommand
		ipRouteShowDefaultCommand = oldRouteShowDefaultCommand
	})

	ipAddressShowCommand = func(_ context.Context, _ string) ([]byte, error) {
		return []byte(`[
			{
				"addr_info": [
					{"family": "inet", "local": "172.20.20.10", "prefixlen": 24, "scope": "global"}
				]
			}
		]`), nil
	}

	ipRouteShowDefaultCommand = func(_ context.Context, family, _ string) ([]byte, error) {
		switch family {
		case "-4":
			return []byte(`[{"gateway": "172.20.20.1"}]`), nil
		case "-6":
			return []byte(`[]`), nil
		default:
			t.Fatalf("unexpected route family %q", family)
		}

		return nil, nil
	}

	f := NewFormatters("", "", "leaf1", "", testQemuProfile(false), false)

	actual, err := f.RenderTemplate(
		`hostname {{ .hostname }}
address {{ .mgmtIPv4Address }}
network {{ .mgmtIPv4Network }}
ipv6 {{ .mgmtIPv6Address }}`,
	)
	if err != nil {
		t.Fatalf("rendering template failed: %v", err)
	}

	expected := `hostname leaf1
address 172.20.20.10
network 172.20.20.0/24
ipv6 `

	if actual != expected {
		t.Fatalf("rendered template mismatch, got %q, want %q", actual, expected)
	}
}

func TestRenderTemplateDHCPManagementFormatters(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtPassthrough, "true")
	t.Setenv(boxenconstants.EnvClabMgmtDHCP, "true")

	f := NewFormatters("", "", "", "", testQemuProfile(false), false)

	actual, err := f.RenderTemplate(
		`{{ if .mgmtDHCP }}address dhcp{{ else }}address {{ .mgmtIPv4 }}{{ end }}`,
	)
	if err != nil {
		t.Fatalf("rendering DHCP template failed: %v", err)
	}

	expected := `address dhcp`

	if actual != expected {
		t.Fatalf("rendered template mismatch, got %q, want %q", actual, expected)
	}

	_, err = f.RenderTemplate(`address {{ .mgmtIPv4 }}`)
	if err == nil {
		t.Fatal("expected direct management address use to fail in DHCP mode")
	}
}
