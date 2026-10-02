package profile

import (
	"os/exec"
	"strings"
	"testing"

	boxenassets "github.com/carlmontanari/boxen/assets"
	"go.yaml.in/yaml/v4"
)

func loadCommunitySonicProfile(t *testing.T) Profile {
	t.Helper()

	data, err := boxenassets.Assets.ReadFile("profiles/community_sonic.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}

	return p
}

func TestCommunitySonicCommands(t *testing.T) {
	p := loadCommunitySonicProfile(t)

	for _, mode := range []struct {
		name       string
		management *managementFormatters
	}{
		{"ipv4", &managementFormatters{ipv4: "192.0.2.2/24", ipv4Gateway: "192.0.2.1"}},
		{"ipv6", &managementFormatters{ipv6: "2001:db8::2/64", ipv6Gateway: "2001:db8::1"}},
		{"dual-stack", defaultManagementFormatters()},
		{"dhcp", dhcpManagementFormatters()},
	} {
		t.Run(mode.name, func(t *testing.T) {
			f := NewFormatters("labuser", `a'b $(printf unsafe) " c`, "sonic-check", "", &p, false)
			f.management = mode.management
			phases := [][]Step{p.Packaging.Process, p.Run.Process, p.Run.ConfigProcess}
			for _, steps := range phases {
				for _, step := range steps {
					if step.Type != StepTypeWrite ||
						strings.Contains(step.Write.Content, "fileBase64") {
						continue
					}
					command, err := f.RenderTemplate(step.Write.Content)
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(command, "\n") {
						t.Fatalf("shell command spans console lines: %q", command)
					}
					//nolint:gosec // Only parses trusted profile commands, without executing them.
					output, err := exec.CommandContext(t.Context(), "bash", "-n", "-c", command).
						CombinedOutput()
					if err != nil {
						t.Fatalf("invalid shell command %q: %s", command, output)
					}
				}
			}
		})
	}
}

func TestCommunitySonicCompletion(t *testing.T) {
	p := loadCommunitySonicProfile(t)

	for _, prompt := range []string{
		"admin@sonic:~$ ",
		"operator@leaf1:~$ ",
		"lab-user@leaf.example:~$ ",
	} {
		got, err := p.Run.Process[2].ReadUntil.Until.Check([]byte(prompt))
		if err != nil || !got {
			t.Fatalf("native prompt %q did not match: got %v, err=%v", prompt, got, err)
		}
	}

	complete := p.Run.Process[4].ReadUntil.Until
	for _, result := range []struct {
		output string
		want   bool
	}{
		{`printf '\nBOXEN_OK\n'`, false},
		{"\r\nBOXEN_OK\r\n", false},
		{"\r\nBOXEN_FAILED\r\nadmin@sonic:~$ ", false},
		{"\r\nadmin@sonic:~$ ", false},
		{"\r\nBOXEN_OK\r\nadmin@sonic:~$ ", true},
		{"\nBOXEN_OK\noperator@leaf1:~$ ", true},
		{"\nBOXEN_OK\nlab-user@leaf.example:~$ ", true},
		{"\nBOXEN_OK\nlab-user@leaf.example:~$ echo next", false},
	} {
		got, err := complete.Check([]byte(result.output))
		if err != nil || got != result.want {
			t.Fatalf(
				"completion check for %q: got %v, want %v, err=%v",
				result.output,
				got,
				result.want,
				err,
			)
		}
	}
}

func TestCommunitySonicReadiness(t *testing.T) {
	p := loadCommunitySonicProfile(t)

	// Every required service must be active; systemctl with multiple units accepts any one.
	for _, active := range []bool{true, false} {
		state := "no"
		if active {
			state = "yes"
		}
		command := "BOXEN_TEST_BGP_ACTIVE=" + state + `
sudo() { "$@"; }
timeout() { shift; command timeout 0.1 "$@"; }
systemctl() { [ "$3" != bgp ] || [ "$BOXEN_TEST_BGP_ACTIVE" = yes ]; }
docker() { printf 'PONG\nRUNNING\n'; }
export BOXEN_TEST_BGP_ACTIVE
export -f sudo timeout systemctl docker
` + p.Run.Process[17].Write.Content
		//nolint:gosec // Runs readiness against fixed shell stubs, without contacting services.
		output, err := exec.CommandContext(t.Context(), "bash", "-c", command).CombinedOutput()
		if (err == nil) != active || strings.Contains(string(output), "BOXEN_OK") != active {
			t.Fatalf("readiness with bgp active=%v: err=%v, output=%s", active, err, output)
		}
	}
}
