package main

import (
	"os"
	"testing"

	boxenconstants "github.com/carlmontanari/boxen/constants"
)

func TestApplyLegacyResourceFlags(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabQemuSMP, "")
	t.Setenv(boxenconstants.EnvClabQemuMemory, "8192")

	if err := applyLegacyResourceFlags("2", "16384"); err != nil {
		t.Fatal(err)
	}

	if got := os.Getenv(boxenconstants.EnvClabQemuSMP); got != "2" {
		t.Fatalf("expected --vcpu to set QEMU_SMP, got %q", got)
	}

	// an explicit environment value wins over the flag
	if got := os.Getenv(boxenconstants.EnvClabQemuMemory); got != "8192" {
		t.Fatalf("expected QEMU_MEMORY to be kept, got %q", got)
	}
}

// TestRunAcceptsContainerlabFlags checks that the run command defines the flags containerlab passes
// to VM kinds: an undefined flag makes the node exit at once.
func TestRunAcceptsContainerlabFlags(t *testing.T) {
	names := map[string]bool{}

	for _, flag := range runCommand().Flags {
		for _, name := range flag.Names() {
			names[name] = true
		}
	}

	// e.g. the SR OS kind: --trace --connection-mode vrxcon --hostname sr1 --variant "sr-1"
	for _, want := range []string{
		boxenconstants.FlagContainerlabTrace,
		boxenconstants.FlagContainerlabConnectionMode,
		boxenconstants.FlagContainerlabHostname,
		boxenconstants.FlagContainerlabUsername,
		boxenconstants.FlagContainerlabPassword,
		boxenconstants.FlagContainerlabVCPU,
		boxenconstants.FlagContainerlabRAM,
		boxenconstants.FlagContainerlabVariant,
	} {
		if !names[want] {
			t.Errorf("run does not accept --%s", want)
		}
	}
}
