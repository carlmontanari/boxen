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
