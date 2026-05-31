package agent

import (
	"context"
	"log/slog"
	"os"
	"testing"

	boxenprofile "github.com/carlmontanari/boxen/profile"
)

func TestPackagePrepareDiskSkipsExpectedDiskName(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(
		func() {
			_ = os.Chdir(originalDir)
		},
	)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile("disk.qcow2", []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := NewAgent(slog.LevelError)
	a.p = &boxenprofile.Profile{
		ResolvedDisk: "/tmp/disk.qcow2",
	}

	if err := a.packagePrepareDisk(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat("disk.qcow2"); err != nil {
		t.Fatal(err)
	}
}
