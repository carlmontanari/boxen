package boxen

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	boxenconstants "github.com/carlmontanari/boxen/constants"
)

func TestPrepareBuildRejectsInvalidInputs(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	disk := filepath.Join(dir, "cumulus-linux-5.16.5-vx-amd64.qcow2")
	if err := os.WriteFile(disk, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		profile string
		want    string
	}{
		{profile: "nvidia_cumulus", want: "nvidia_cumulus"},
		{profile: "nvidia_cumulusvx", want: "nvidia_cumulusvx_breakout.star"},
	} {
		t.Run(test.profile, func(t *testing.T) {
			b := &Boxen{l: slog.New(slog.DiscardHandler)}
			err := b.prepareBuild(disk, test.profile)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected an error naming %q, got %v", test.want, err)
			}
			if test.profile == "nvidia_cumulusvx" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("expected missing companion file, got %v", err)
			}
		})
	}
}

func TestPrepareBuildCompanionsBesideDisk(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(t.TempDir())
	disk := filepath.Join(dir, "cumulus-linux-5.16.5-vx-amd64.qcow2")
	module := filepath.Join(dir, "nvidia_cumulusvx_breakout.star")
	template := filepath.Join(dir, "nvidia_cumulusvx_breakout.sh.tmpl")
	for _, path := range []string{disk, module} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	b := &Boxen{l: slog.New(slog.DiscardHandler)}
	if err := b.prepareBuild(disk, "nvidia_cumulusvx"); !errors.Is(err, os.ErrNotExist) ||
		!strings.Contains(err.Error(), filepath.Base(template)) {
		t.Fatalf("expected missing second companion, got %v", err)
	}
	if err := os.Mkdir(template, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := b.prepareBuild(disk, "nvidia_cumulusvx"); err == nil ||
		!strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("expected directory rejection, got %v", err)
	}
	if err := os.Remove(template); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(template, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := b.prepareBuild(disk, "nvidia_cumulusvx"); err != nil {
		t.Fatalf("companions beside disk should resolve: %v", err)
	}
	if b.p.ResolvedVersion != "5.16.5" {
		t.Fatalf("expected disk version 5.16.5, got %q", b.p.ResolvedVersion)
	}
}

func TestBuildBuilderEnv(t *testing.T) {
	host := "127.0.0.1"

	env := buildBuilderEnv(host, false)

	expectedServerHost := fmt.Sprintf("%s=%s", boxenconstants.EnvServerHost, host)
	if !slices.Contains(env, expectedServerHost) {
		t.Fatalf("builder env missing server host, got %v, want %q", env, expectedServerHost)
	}

	vmConsole := fmt.Sprintf("%s=true", boxenconstants.EnvVMConsole)
	if slices.Contains(env, vmConsole) {
		t.Fatalf("builder env unexpectedly contains vm-console signal, got %v", env)
	}
}

func TestBuildBuilderEnvVMConsole(t *testing.T) {
	host := "127.0.0.1"

	env := buildBuilderEnv(host, true)

	expectedServerHost := fmt.Sprintf("%s=%s", boxenconstants.EnvServerHost, host)
	if !slices.Contains(env, expectedServerHost) {
		t.Fatalf("builder env missing server host, got %v, want %q", env, expectedServerHost)
	}

	vmConsole := fmt.Sprintf("%s=true", boxenconstants.EnvVMConsole)
	if !slices.Contains(env, vmConsole) {
		t.Fatalf("builder env missing vm-console signal, got %v, want %q", env, vmConsole)
	}
}

func TestBuildBuilderVolumes(t *testing.T) {
	volumes := buildBuilderVolumes()

	for _, volume := range []string{
		"/boot:/boot:ro",
		"/lib/modules:/lib/modules:ro",
	} {
		if !slices.Contains(volumes, volume) {
			t.Fatalf("builder volumes missing mount, got %v, want %q", volumes, volume)
		}
	}
}

func TestBuildConsoleAttachCommand(t *testing.T) {
	actual := buildOpenConsoleCommand("container-id")
	expected := "docker exec -i -t container-id telnet localhost 5001"

	if actual != expected {
		t.Fatalf("attach command incorrect, got %q, want %q", actual, expected)
	}
}
