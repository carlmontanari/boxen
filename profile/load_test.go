package profile

import (
	"path/filepath"
	"strings"
	"testing"

	boxenassets "github.com/carlmontanari/boxen/assets"
)

const minimalProfile = `name: test
virtualMachine:
  memory: 1024
  serialPortCount: 1
  nicType: virtio-net-pci
  nicCount: 1
  nicPerBus: 26
packaging:
  process:
    - type: readUntil
      readUntil:
        timeout: 1m
        until:
          contains: "login:"
run:
  process:
    - type: wait
      wait:
        duration: 1s
`

func TestEmbeddedProfilesLoad(t *testing.T) {
	entries, err := boxenassets.Assets.ReadDir("profiles")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		extension := filepath.Ext(entry.Name())
		if entry.IsDir() || (extension != ".yaml" && extension != ".yml") {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			b, err := boxenassets.Assets.ReadFile("profiles/" + entry.Name())
			if err != nil {
				t.Fatal(err)
			}

			if _, err := Load(b); err != nil {
				t.Fatalf("embedded profile does not load: %v", err)
			}
		})
	}
}

func TestLoadMinimalProfile(t *testing.T) {
	_, err := Load([]byte(minimalProfile))
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	data := strings.Replace(
		minimalProfile,
		"  memory: 1024\n",
		"  memory: 1024\n  emulate: max\n",
		1,
	)

	_, err := Load([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "emulate") {
		t.Fatalf("expected unknown key error naming emulate, got %v", err)
	}
}

func TestValidateReportsProblems(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(p *Profile)
		want   string
	}{
		{
			name:   "missing name",
			mutate: func(p *Profile) { p.Name = "" },
			want:   "name is required",
		},
		{
			name:   "missing memory",
			mutate: func(p *Profile) { p.VirtualMachine.Memory = 0 },
			want:   "virtualMachine.memory is required",
		},
		{
			name:   "missing serial port",
			mutate: func(p *Profile) { p.VirtualMachine.SerialPortCount = 0 },
			want:   "serialPortCount",
		},
		{
			name:   "nics without bus size",
			mutate: func(p *Profile) { p.VirtualMachine.NicPerBus = 0 },
			want:   "nicPerBus",
		},
		{
			name: "unknown step type",
			mutate: func(p *Profile) {
				p.Run.Process = append(p.Run.Process, Step{Type: "type"})
			},
			want: `run.process[1]: unknown step type "type"`,
		},
		{
			name: "bad duration",
			mutate: func(p *Profile) {
				p.Run.Process[0].Wait.Duration = "soon"
			},
			want: "run.process[0]: wait.duration",
		},
		{
			name: "write without content",
			mutate: func(p *Profile) {
				p.Run.ConfigProcess = []Step{{Type: StepTypeWrite}}
			},
			want: "run.configProcess[0]: write requires content",
		},
		{
			name: "bad pattern",
			mutate: func(p *Profile) {
				p.Packaging.Process[0].ReadUntil.Until = Contains{ContainsPattern: "("}
			},
			want: "packaging.process[0]: readUntil.until",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			p, err := Load([]byte(minimalProfile))
			if err != nil {
				t.Fatal(err)
			}

			test.mutate(p)

			err = p.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}
