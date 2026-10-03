package profile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestTemplateReadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.txt")
	for _, packaging := range []bool{true, false} {
		f := NewFormatters("", "", "", "", &Profile{}, packaging)
		for _, content := range []string{"first\n{{ .literal }}", "second", ""} {
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := f.RenderTemplate(fmt.Sprintf(`{{ readFile %q "fallback" }}`, path))
			if err != nil || got != content {
				t.Fatalf("packaging=%v contents=%q error=%v", packaging, got, err)
			}
		}
		missing := path + ".missing"
		if _, err := f.RenderTemplate(fmt.Sprintf(`{{ readFile %q }}`, missing)); err == nil {
			t.Fatal("a missing required file must fail")
		}
		got, err := f.RenderTemplate(fmt.Sprintf(`{{ readFile %q "fallback" }}`, missing))
		if err != nil || got != "fallback" {
			t.Fatalf("optional file: contents=%q error=%v", got, err)
		}
	}
	if _, err := readFile(filepath.Dir(path), "fallback"); err == nil {
		t.Fatal("defaults must not hide other filesystem errors")
	}
	if _, err := readFile(path, "one", "two"); err == nil {
		t.Fatal("multiple defaults must fail")
	}
}

func TestStarlarkVMSettings(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir("helpers", 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"hardware.json": `{"memory": 2048, "cpuCores": 4, "nicCount": 26}`,
		"helpers/settings.star": `load("numbers.star", "add")
def settings(vm):
    data = json.decode(read_file("hardware.json"))
    data["cpuCores"] = add(data["cpuCores"], vm["cpuCores"])
    return data
`,
		"helpers/numbers.star": "def add(a, b):\n    return a + b\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p := testQemuProfile(false)
	p.VirtualMachine.CPUCores = 2
	p.VirtualMachine.NicCount = 1
	p.VirtualMachine.NicPerBus = 26
	p.VirtualMachine.Configure = `load("helpers/settings.star", "settings")
def configure(vm):
    if is_packaging:
        return {"memory": 512}
    return settings(vm)
`
	args, err := QemuArgsFromProfile(p, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.VirtualMachine.Memory != 2048 || p.VirtualMachine.CPUCores != 6 ||
		p.VirtualMachine.NicCount != 26 ||
		!slices.Contains(args, "tap,id=p026,ifname=tap26,script=no,downscript=no") ||
		!slices.Contains(args, "pci-bridge,chassis_nr=2,id=pci.2") {
		t.Fatalf("derived settings were not used for QEMU/forwarding: vm=%+v args=%v",
			p.VirtualMachine, args)
	}
	if err := os.WriteFile("helpers/numbers.star",
		[]byte("def add(a, b):\n    return a * b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.VirtualMachine.ApplyConfiguration(false); err != nil ||
		p.VirtualMachine.CPUCores != 24 {
		t.Fatalf("editing a companion script must change behavior without rebuilding: %v", err)
	}
	if err := os.Remove("hardware.json"); err != nil {
		t.Fatal(err)
	}
	if err := p.VirtualMachine.ApplyConfiguration(true); err != nil ||
		p.VirtualMachine.Memory != 512 {
		t.Fatalf("packaging phase should not read the runtime file: %v", err)
	}
}

func TestInvalidStarlarkVMSettings(t *testing.T) {
	for _, source := range []string{
		`def other(vm): return {}`,
		`def configure(vm): return []`,
		`def configure(vm): return {"nicCount": -1}`,
		`def configure(vm): return {"nicCount": 65536}`,
		`def configure(vm): return {"nicCount": "invalid"}`,
		`def configure(vm): return {"misspelledField": 3}`,
		`def configure(vm):
    vm["mutators"]["cpu"] = "changed"
    vm["nicCount"] = -1
    return vm`,
	} {
		v := &VirtualMachine{
			NicCount: 16, Memory: 4096, Configure: source,
			Mutators: map[string]string{"cpu": "original"},
		}
		before, err := yaml.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := v.ApplyConfiguration(false); err == nil {
			t.Fatalf("expected invalid configuration to fail: %s", source)
		}
		after, err := yaml.Marshal(v)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("invalid configuration changed VM state: %s error=%v", source, err)
		}
	}
}

func TestStarlarkTemplateAndMutatorFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	for path, content := range map[string]string{
		"data.txt": "runtime\n{{ .literal }}",
		"helper.star": `def values(raw):
    return {"raw": raw, "count": 26, "packaging": is_packaging}
def mutate(items):
    return items + [read_file("data.txt"), str(is_packaging)]
`,
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, packaging := range []bool{true, false} {
		f := NewFormatters("", "", "", "", &Profile{}, packaging)
		got, err := f.RenderTemplate(
			`{{ $v := starlark "helper.star" "values" (readFile "data.txt") }}` +
				`{{ $v.raw }}|{{ eq $v.count 26 }}|{{ $v.packaging }}|{{ .isPackaging }}`,
		)
		want := fmt.Sprintf("runtime\n{{ .literal }}|true|%t|%t", packaging, packaging)
		if err != nil || got != want {
			t.Fatalf("Starlark template: result=%q error=%v", got, err)
		}
		args, err := invokeStarlarkF(`load("helper.star", helper_mutate="mutate")
def mutate(items):
    return helper_mutate(items)
`, []string{"before"}, packaging)
		phase := "False"
		if packaging {
			phase = "True"
		}
		if err != nil || !reflect.DeepEqual(args,
			[]string{"before", "runtime\n{{ .literal }}", phase}) {
			t.Fatalf("file-backed mutator: args=%v error=%v", args, err)
		}
	}
	if _, err := invokeStarlarkF(`def mutate(items): return [1]`, nil, false); err == nil {
		t.Fatal("QEMU arguments must remain strings")
	}
	if got, err := invokeStarlarkF(
		`def mutate(items): return items + ["added"]`,
		nil,
		false,
	); err != nil ||
		!reflect.DeepEqual(got, []string{"added"}) {
		t.Fatalf(
			"empty argument sections must reach mutators as a list: args=%v error=%v",
			got,
			err,
		)
	}
}

func TestStarlarkLoadCycle(t *testing.T) {
	t.Chdir(t.TempDir())
	for path, content := range map[string]string{
		"a.star": "load(\"b.star\", \"b\")\na = b\n",
		"b.star": "load(\"a.star\", \"a\")\nb = a\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := callStarlark(nil, "a.star", "a", false); err == nil ||
		!strings.Contains(err.Error(), "cycle") {
		t.Fatalf("module load cycle must fail: %v", err)
	}
}
