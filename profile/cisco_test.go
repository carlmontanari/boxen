package profile

import (
	"strings"
	"testing"

	boxenassets "github.com/carlmontanari/boxen/assets"
)

func loadEmbeddedProfile(t *testing.T, name string) *Profile {
	t.Helper()

	b, err := boxenassets.Assets.ReadFile("profiles/" + name + ".yaml")
	if err != nil {
		t.Fatal(err)
	}

	p, err := Load(b)
	if err != nil {
		t.Fatal(err)
	}

	return p
}

// renderRunConfig renders the templated write steps of the run process.
func renderRunConfig(t *testing.T, p *Profile, management *managementFormatters) string {
	t.Helper()

	f := NewFormatters("labuser", "lab-pass", "node1", "", p, false)
	f.management = management

	var out strings.Builder

	for idx := range p.Run.Process {
		step := &p.Run.Process[idx]
		if step.Type != StepTypeWrite || !strings.Contains(step.Write.Content, "{{") {
			continue
		}

		content, err := f.RenderTemplate(step.Write.Content)
		if err != nil {
			t.Fatal(err)
		}

		out.WriteString(content)
	}

	return out.String()
}

func TestCiscoRunConfig(t *testing.T) {
	ipv4 := &managementFormatters{ipv4Gateway: "192.0.2.1"}
	if err := ipv4.applyIPv4CIDR("192.0.2.2/24"); err != nil {
		t.Fatal(err)
	}

	ipv6 := &managementFormatters{ipv6Gateway: "2001:db8::1"}
	if err := ipv6.applyIPv6CIDR("2001:db8::2/64"); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		profile string
		mode    string
		mgmt    *managementFormatters
		want    []string
		notWant []string
	}{
		{
			profile: "cisco_csr1000v",
			mode:    "ipv4",
			mgmt:    ipv4,
			want: []string{
				"hostname node1",
				"username labuser privilege 15 secret 0 lab-pass",
				"ip address 192.0.2.2 255.255.255.0",
				"ip route vrf clab-mgmt 0.0.0.0 0.0.0.0 192.0.2.1",
			},
			notWant: []string{"ipv6 address", "ipv6 route"},
		},
		{
			profile: "cisco_csr1000v",
			mode:    "ipv6",
			mgmt:    ipv6,
			want: []string{
				"ipv6 address 2001:db8::2/64",
				"ipv6 route vrf clab-mgmt ::/0 2001:db8::1",
			},
			notWant: []string{"ip address 192"},
		},
		{
			profile: "cisco_csr1000v",
			mode:    "dhcp",
			mgmt:    dhcpManagementFormatters(),
			want:    []string{"ip address dhcp"},
			notWant: []string{"ip route vrf"},
		},
		{
			profile: "cisco_n9kv",
			mode:    "dual-stack",
			mgmt:    defaultManagementFormatters(),
			want: []string{
				"hostname node1",
				"username labuser password 0 lab-pass role network-admin",
				"ip address 10.0.0.15/24",
				"ipv6 address 2001:db8::2/64",
				"ip route 0.0.0.0/0 10.0.0.2",
				"ipv6 route ::/0 2001:db8::1",
			},
		},
		{
			profile: "cisco_n9kv",
			mode:    "dhcp",
			mgmt:    dhcpManagementFormatters(),
			want:    []string{"ip address dhcp"},
			notWant: []string{"vrf context management"},
		},
	} {
		t.Run(test.profile+"/"+test.mode, func(t *testing.T) {
			config := renderRunConfig(t, loadEmbeddedProfile(t, test.profile), test.mgmt)

			for _, want := range test.want {
				if !strings.Contains(config, want) {
					t.Errorf("missing %q in:\n%s", want, config)
				}
			}

			for _, notWant := range test.notWant {
				if strings.Contains(config, notWant) {
					t.Errorf("unexpected %q in:\n%s", notWant, config)
				}
			}
		})
	}
}

// TestCiscoConsoleLogin checks that the console automation does not depend on the containerlab
// credentials, which the run process changes.
func TestCiscoConsoleLogin(t *testing.T) {
	for _, name := range []string{"cisco_csr1000v", "cisco_n9kv"} {
		p := loadEmbeddedProfile(t, name)

		for _, steps := range [][]Step{p.Run.Process, p.Run.SaveProcess} {
			for _, step := range steps {
				for _, prompt := range step.Prompts.Prompts {
					if strings.Contains(prompt.Response, "{{") {
						t.Errorf(
							"%s: console login uses templated credentials: %q",
							name,
							prompt.Response,
						)
					}
				}
			}
		}
	}
}

func TestN9kvInterfaceMACs(t *testing.T) {
	p := loadEmbeddedProfile(t, "cisco_n9kv")
	p.DataNICMACs = []string{"aa:c1:ab:94:6d:16", "52:54:00:00:00:02"}

	f := NewFormatters("labuser", "lab-pass", "node1", "", p, false)
	f.management = defaultManagementFormatters()

	var rendered string

	for _, step := range p.Run.Process {
		if strings.Contains(step.Write.Content, "dataNICMACs") {
			content, err := f.RenderTemplate(step.Write.Content)
			if err != nil {
				t.Fatal(err)
			}

			rendered = content
		}
	}

	for _, want := range []string{
		"interface Ethernet1/1\nmac-address aac1.ab94.6d16",
		"interface Ethernet1/2\nmac-address 5254.0000.0002",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("missing %q in:\n%s", want, rendered)
		}
	}
}
