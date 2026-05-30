package boxen

import (
	"fmt"
	"slices"
	"testing"

	boxenconstants "github.com/carlmontanari/boxen/constants"
)

func TestBuildBuilderEnv(t *testing.T) {
	host := "127.0.0.1"

	env := buildBuilderEnv(host, nil)

	expectedServerHost := fmt.Sprintf("%s=%s", boxenconstants.EnvServerHost, host)
	if !slices.Contains(env, expectedServerHost) {
		t.Fatalf("builder env missing server host, got %v, want %q", env, expectedServerHost)
	}

	onlyStartVM := fmt.Sprintf("%s=true", boxenconstants.EnvOnlyStartVM)
	if slices.Contains(env, onlyStartVM) {
		t.Fatalf("builder env unexpectedly contains only-start-vm signal, got %v", env)
	}
}

func TestBuildBuilderEnvOnlyStartVM(t *testing.T) {
	host := "127.0.0.1"

	env := buildBuilderEnv(host, &BuildOptions{OnlyStartVM: true})

	expectedServerHost := fmt.Sprintf("%s=%s", boxenconstants.EnvServerHost, host)
	if !slices.Contains(env, expectedServerHost) {
		t.Fatalf("builder env missing server host, got %v, want %q", env, expectedServerHost)
	}

	onlyStartVM := fmt.Sprintf("%s=true", boxenconstants.EnvOnlyStartVM)
	if !slices.Contains(env, onlyStartVM) {
		t.Fatalf("builder env missing only-start-vm signal, got %v, want %q", env, onlyStartVM)
	}
}

func TestBuildConsoleAttachCommand(t *testing.T) {
	actual := buildConsoleAttachCommand("container-id")
	expected := "docker exec -i -t container-id telnet localhost 5001"

	if actual != expected {
		t.Fatalf("attach command incorrect, got %q, want %q", actual, expected)
	}
}
