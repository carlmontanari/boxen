package profile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	boxenassets "github.com/carlmontanari/boxen/assets"
	"go.yaml.in/yaml/v4"
)

func loadCumulusProfile(t *testing.T) Profile {
	t.Helper()
	data, err := boxenassets.Assets.ReadFile("profiles/nvidia_cumulusvx.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	p.VirtualMachine.ManagementPassthrough = false

	return p
}

type testCumulusLane struct {
	Index int
	Name  string
}

func prepareCumulusFiles(t *testing.T) string {
	t.Helper()
	module, err := os.ReadFile("../assets/profiles/nvidia_cumulusvx_breakout.star")
	if err != nil {
		t.Fatal(err)
	}
	template, err := os.ReadFile("../assets/profiles/nvidia_cumulusvx_breakout.sh.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "ports.conf")
	module = []byte(
		strings.ReplaceAll(string(module), `"/config/ports.conf"`, fmt.Sprintf("%q", path)),
	)
	for name, content := range map[string][]byte{
		"nvidia_cumulusvx_breakout.star":    module,
		"nvidia_cumulusvx_breakout.sh.tmpl": template,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)

	return path
}

func cumulusLanes(t *testing.T) []testCumulusLane {
	t.Helper()
	result, err := callStarlark(nil, "nvidia_cumulusvx_breakout.star", "layout", false)
	if err != nil {
		t.Fatal(err)
	}
	data, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("layout must return a dictionary: %v", result)
	}
	items, ok := data["lanes"].([]any)
	if !ok {
		t.Fatalf("lanes must be a list: %v", data)
	}
	if len(items) == 0 {
		return nil
	}
	lanes := make([]testCumulusLane, 0, len(items))
	for _, item := range items {
		lane, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("lane must be a dictionary: %v", item)
		}
		index, indexOK := lane["Index"].(int)
		name, nameOK := lane["Name"].(string)
		if !indexOK || !nameOK {
			t.Fatalf("invalid lane fields: %v", lane)
		}
		lanes = append(lanes, testCumulusLane{index, name})
	}

	return lanes
}

func TestCumulusPortLayout(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		count         uint16
		lanes         []testCumulusLane
	}{
		{name: "missing", count: 16},
		{name: "empty", content: "# no layout\n", count: 16},
		{name: "base only", content: "64=1x\n", count: 64},
		{
			name: "sparse unordered parents", content: "10=4x, 64=1x\n2 = 2x # note\n",
			count: 70,
			lanes: []testCumulusLane{
				{65, "swp2s0"},
				{66, "swp2s1"},
				{67, "swp10s0"},
				{68, "swp10s1"},
				{69, "swp10s2"},
				{70, "swp10s3"},
			},
		},
		{
			name: "eight lanes", content: "1=8x\n", count: 16,
			lanes: []testCumulusLane{
				{2, "swp1s0"},
				{3, "swp1s1"},
				{4, "swp1s2"},
				{5, "swp1s3"},
				{6, "swp1s4"},
				{7, "swp1s5"},
				{8, "swp1s6"},
				{9, "swp1s7"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := loadCumulusProfile(t)
			path := prepareCumulusFiles(t)
			if tc.name != "missing" {
				if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before := slices.Clone(p.Run.Process)
			if err := p.VirtualMachine.ApplyConfiguration(false); err != nil {
				t.Fatal(err)
			}
			if p.VirtualMachine.NicCount != tc.count ||
				!reflect.DeepEqual(cumulusLanes(t), tc.lanes) {
				t.Fatalf("count=%d lanes=%v, want count=%d lanes=%v",
					p.VirtualMachine.NicCount, cumulusLanes(t), tc.count, tc.lanes)
			}
			if !reflect.DeepEqual(before, p.Run.Process) {
				t.Fatal("NIC sizing changed the profile's primitive console steps")
			}
		})
	}
}

func TestCumulusPortLayoutErrors(t *testing.T) {
	for _, content := range []string{
		"0=4x", "-1=4x", "1000=1x", "65536=2x", "1=0x", "1=3x", "1=9x",
		"1=4", "1=100G", "1", "garbage", "1=4x trailing", "1=4x\n1=2x",
		"1=8x\n992=1x", // 1000 NICs after allocating lanes.
	} {
		t.Run(content, func(t *testing.T) {
			path := prepareCumulusFiles(t)
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			p := loadCumulusProfile(t)
			before := slices.Clone(p.Run.Process)
			if err := p.VirtualMachine.ApplyConfiguration(
				false,
			); err == nil ||
				!strings.Contains(err.Error(), path) {
				t.Fatalf("expected an error identifying ports.conf, got %v", err)
			}
			if p.VirtualMachine.NicCount != 16 || !reflect.DeepEqual(before, p.Run.Process) {
				t.Fatal("invalid configuration changed profile state")
			}
		})
	}
}

func TestCumulusExternalFilesAndPackaging(t *testing.T) {
	for _, name := range []string{
		"nvidia_cumulusvx_breakout.star", "nvidia_cumulusvx_breakout.sh.tmpl",
	} {
		if _, err := boxenassets.Assets.ReadFile("profiles/" + name); !os.IsNotExist(err) {
			t.Fatalf("companion must remain external: %s error=%v", name, err)
		}
	}
	path := prepareCumulusFiles(t)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	p := loadCumulusProfile(t)
	if !reflect.DeepEqual(p.ExtraFiles, []string{
		"nvidia_cumulusvx_breakout.star", "nvidia_cumulusvx_breakout.sh.tmpl",
	}) {
		t.Fatalf("companion files must be explicitly listed for transfer: %v", p.ExtraFiles)
	}
	p.Name = "custom_recipe"
	if err := p.VirtualMachine.ApplyConfiguration(true); err != nil ||
		p.VirtualMachine.NicCount != 16 {
		t.Fatalf("packaging must keep baseline hardware without reading ports.conf: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("64=1x\n2=2x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.VirtualMachine.ApplyConfiguration(false); err != nil ||
		p.VirtualMachine.NicCount != 66 {
		t.Fatalf("custom recipe must use external runtime data: %v", err)
	}
	if err := os.WriteFile(path, []byte("64=1x\n2=4x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.VirtualMachine.ApplyConfiguration(false); err != nil ||
		p.VirtualMachine.NicCount != 68 {
		t.Fatalf("runtime file changes must be read without rebuilding: %v", err)
	}
}

func TestCumulusBreakoutCommands(t *testing.T) {
	p := loadCumulusProfile(t)
	path := prepareCumulusFiles(t)
	if err := os.WriteFile(path, []byte("64=1x\n2=2x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.VirtualMachine.ApplyConfiguration(false); err != nil {
		t.Fatal(err)
	}
	var source string
	var complete Contains
	for _, step := range p.Run.Process {
		if step.Write.ContentFromFile != "" {
			if !step.Write.Hidden {
				t.Fatal("breakout commands must support disabled console echo")
			}
			data, err := os.ReadFile(step.Write.ContentFromFile)
			if err != nil {
				t.Fatal(err)
			}
			source = string(data)
		}
		if strings.Contains(step.ReadUntil.Until.ContainsPattern, "BOXEN_BREAKOUT_OK") {
			complete = step.ReadUntil.Until
		}
	}
	f := NewFormatters("", "", "leaf", "", &p, false)
	command, err := f.RenderTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	transfer, apply, ok := strings.Cut(command, "stty echo && ")
	if !ok || complete.ContainsPattern == "" {
		t.Fatal("missing breakout companion commands or completion check")
	}
	apply = "stty echo && " + apply
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	for _, packaging := range []bool{false, true} {
		other := loadCumulusProfile(t)
		otherFormatters := NewFormatters("", "", "leaf", "", &other, packaging)
		command, err := otherFormatters.RenderTemplate(source)
		if err != nil || strings.Contains(command, "rename_lane") ||
			!strings.Contains(command, "BOXEN_BREAKOUT_OK") {
			t.Fatalf("no layout/packaging must skip renames: command=%q error=%v", command, err)
		}
	}
	for _, result := range []struct {
		output string
		want   bool
	}{
		{apply, false},
		{"\nBOXEN_BREAKOUT_OK\n", false},
		{"\nBOXEN_BREAKOUT_FAILED\ncumulus@leaf:mgmt:~$ ", false},
		{"\r\nBOXEN_BREAKOUT_OK\r\ncumulus@leaf:mgmt:~$ ", true},
		{"\r\nBOXEN_BREAKOUT_OK\r\n\x1b[?2004hcumulus@leaf:mgmt:~$ \r\n" +
			"\x1b[?2004l\r\x1b[?2004hcumulus@leaf:mgmt:~$ ", true},
	} {
		got, err := complete.Check([]byte(result.output))
		if err != nil || got != result.want {
			t.Fatalf("completion %q: got=%v err=%v", result.output, got, err)
		}
	}
	runCumulusBreakoutCommands(t, transfer, apply)
}

func runCumulusBreakoutCommands(t *testing.T, transfer, apply string) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{
		"swp65": "52:54:00:00:00:41\n", "swp66": "52:54:00:00:00:42\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	quoted, err := shellQuote(dir)
	if err != nil {
		t.Fatal(err)
	}
	commands := `export BOXEN_TEST_DIR=` + quoted + `
stty() { :; }
sudo() { shift; "$@"; }
udevadm() { :; }
systemctl() { :; }
ifreload() {
  test -f "$BOXEN_TEST_DIR/swp2s0" && test -f "$BOXEN_TEST_DIR/swp2s1" || return 1
  rm -f "$BOXEN_TEST_DIR/needs-reload"
}
cat() {
  case "$1" in
    /sys/class/net/*/address)
      name=${1#/sys/class/net/}; command cat "$BOXEN_TEST_DIR/${name%/address}" ;;
    *) command cat "$@" ;;
  esac
}
ip() {
  if [ "$2" = show ]; then test -f "$BOXEN_TEST_DIR/$4"; return; fi
  test -f "$BOXEN_TEST_DIR/$4" || return 1
  if [ "$5" = down ]; then touch "$BOXEN_TEST_DIR/needs-reload"; fi
  if [ "$5" = name ]; then mv "$BOXEN_TEST_DIR/$4" "$BOXEN_TEST_DIR/$6"; fi
}
export -f stty sudo udevadm systemctl ifreload cat ip
` + transfer + "\n" + apply
	commands = strings.ReplaceAll(commands, "/tmp/boxen-breakout.sh", dir+"/breakout.sh")
	commands = strings.ReplaceAll(commands, "/etc/udev/rules.d/70-cumulus-breakout.rules",
		dir+"/rules")
	// Repeat on an already renamed guest, then remove one NIC to exercise failure.
	for attempt := range 3 {
		if attempt == 2 {
			if err := os.Remove(filepath.Join(dir, "swp2s1")); err != nil {
				t.Fatal(err)
			}
		}
		//nolint:gosec // Runs profile commands against filesystem-backed NIC and sudo stubs.
		output, err := exec.CommandContext(t.Context(), "bash", "-c", commands).CombinedOutput()
		if (err == nil) != (attempt < 2) ||
			strings.Contains(string(output), "BOXEN_BREAKOUT_OK") != (attempt < 2) {
			t.Fatalf("attempt %d: err=%v, output=%s", attempt, err, output)
		}
		if attempt < 2 {
			if _, err := os.Stat(filepath.Join(dir, "needs-reload")); !os.IsNotExist(err) {
				t.Fatalf("saved interface state was not restored after renaming: %v", err)
			}
			//nolint:gosec // Reads a fixed filename in this test's temporary directory.
			rules, err := os.ReadFile(filepath.Join(dir, "rules"))
			if err != nil {
				t.Fatal(err)
			}
			for _, rule := range []string{
				`ATTR{address}=="52:54:00:00:00:41", NAME="swp2s0"`,
				`ATTR{address}=="52:54:00:00:00:42", NAME="swp2s1"`,
			} {
				if !strings.Contains(string(rules), rule) {
					t.Fatalf("missing udev rule %s in %s", rule, rules)
				}
			}
		}
	}
}
