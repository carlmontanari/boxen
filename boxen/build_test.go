package boxen

import (
	"fmt"
	"slices"
	"testing"

	boxenconstants "github.com/carlmontanari/boxen/constants"
)

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
