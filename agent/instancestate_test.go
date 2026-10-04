package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	boxenconstants "github.com/carlmontanari/boxen/constants"
)

func TestRunPrepareDisk(t *testing.T) {
	if _, err := exec.LookPath(qemuImgBinary); err != nil {
		t.Skip("qemu-img not available")
	}

	t.Chdir(t.TempDir())

	err := exec.CommandContext( //nolint: gosec // fixed test command
		t.Context(), qemuImgBinary, "create", "-q", "-f", "qcow2",
		boxenconstants.DiskFilename, "16M",
	).Run()
	if err != nil {
		t.Fatal(err)
	}

	a := NewAgent(slog.LevelError)

	if err := a.runPrepareDisk(t.Context()); err != nil {
		t.Fatal(err)
	}

	out, err := exec.CommandContext( //nolint: gosec // fixed test command
		t.Context(), qemuImgBinary, "info", "--output=json", boxenconstants.RunDiskFilename,
	).Output()
	if err != nil {
		t.Fatal(err)
	}

	var info struct {
		BackingFilename string `json:"backing-filename"`
		Format          string `json:"format"`
	}

	if err := json.Unmarshal(out, &info); err != nil {
		t.Fatal(err)
	}

	if info.Format != "qcow2" || info.BackingFilename != boxenconstants.DiskFilename {
		t.Fatalf("unexpected overlay: %s", out)
	}

	// a restarted container reuses its overlay instead of recreating it
	marker := time.Unix(1_000_000, 0)

	if err := os.Chtimes(boxenconstants.RunDiskFilename, marker, marker); err != nil {
		t.Fatal(err)
	}

	if err := a.runPrepareDisk(t.Context()); err != nil {
		t.Fatal(err)
	}

	stat, err := os.Stat(boxenconstants.RunDiskFilename)
	if err != nil {
		t.Fatal(err)
	}

	if !stat.ModTime().Equal(marker) {
		t.Fatal("existing overlay was recreated")
	}
}

func TestRunPrepareDiskFailureLeavesNoOverlay(t *testing.T) {
	if _, err := exec.LookPath(qemuImgBinary); err != nil {
		t.Skip("qemu-img not available")
	}

	t.Chdir(t.TempDir())

	a := NewAgent(slog.LevelError)

	// no packaged disk to back the overlay
	if err := a.runPrepareDisk(t.Context()); err == nil {
		t.Fatal("expected an error without a packaged disk")
	}

	matches, _ := filepath.Glob(boxenconstants.RunDiskFilename + "*")
	if len(matches) != 0 {
		t.Fatalf("failed create left files behind: %v", matches)
	}
}

func TestRunResolveInstanceUUID(t *testing.T) {
	const envUUID = "123E4567-E89B-12D3-A456-426614174000"

	t.Run("environment", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv(boxenconstants.EnvClabUUID, envUUID)

		a := NewAgent(slog.LevelError)
		if err := a.runResolveInstanceUUID(); err != nil {
			t.Fatal(err)
		}

		// the uuid is normalized to lower case
		if a.p.InstanceUUID != "123e4567-e89b-12d3-a456-426614174000" {
			t.Fatalf("expected env uuid, got %q", a.p.InstanceUUID)
		}
	})

	t.Run("invalid environment", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv(boxenconstants.EnvClabUUID, "not-a-uuid")

		if err := NewAgent(slog.LevelError).runResolveInstanceUUID(); err == nil {
			t.Fatal("expected an invalid uuid error")
		}
	})

	t.Run("generated once and reused", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv(boxenconstants.EnvClabUUID, "")

		first := NewAgent(slog.LevelError)
		if err := first.runResolveInstanceUUID(); err != nil {
			t.Fatal(err)
		}

		second := NewAgent(slog.LevelError)
		if err := second.runResolveInstanceUUID(); err != nil {
			t.Fatal(err)
		}

		if first.p.InstanceUUID == "" || first.p.InstanceUUID != second.p.InstanceUUID {
			t.Fatalf(
				"expected a stable uuid, got %q then %q",
				first.p.InstanceUUID,
				second.p.InstanceUUID,
			)
		}
	})

	t.Run("invalid stored uuid is replaced", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv(boxenconstants.EnvClabUUID, "")

		if err := os.WriteFile(
			boxenconstants.InstanceUUIDFilename,
			[]byte("junk"),
			0o600,
		); err != nil {
			t.Fatal(err)
		}

		a := NewAgent(slog.LevelError)
		if err := a.runResolveInstanceUUID(); err != nil {
			t.Fatal(err)
		}

		b, _ := os.ReadFile(boxenconstants.InstanceUUIDFilename)
		if strings.TrimSpace(string(b)) != a.p.InstanceUUID {
			t.Fatalf("expected the new uuid to be stored, got %q", b)
		}
	})
}

func TestRunClabNICProvisionDelayTimeout(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabIntfs, "4")
	t.Setenv(boxenconstants.EnvClabIntfPrefix, "boxentestnointf")
	t.Setenv(boxenconstants.EnvIntfWaitTimeout, "50ms")

	start := time.Now()

	if err := NewAgent(slog.LevelError).runClabNICProvisionDelay(t.Context()); err != nil {
		t.Fatal(err)
	}

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("wait was not bounded by the timeout, took %s", elapsed)
	}
}

func TestIntfWaitTimeout(t *testing.T) {
	t.Setenv(boxenconstants.EnvIntfWaitTimeout, "")

	got, err := intfWaitTimeout()
	if err != nil || got != defaultIntfWaitTimeout {
		t.Fatalf("expected default timeout, got %s, %v", got, err)
	}

	t.Setenv(boxenconstants.EnvIntfWaitTimeout, "30s")

	got, err = intfWaitTimeout()
	if err != nil || got != 30*time.Second {
		t.Fatalf("expected 30s, got %s, %v", got, err)
	}

	t.Setenv(boxenconstants.EnvIntfWaitTimeout, "soon")

	if _, err = intfWaitTimeout(); err == nil {
		t.Fatal("expected an invalid duration error")
	}
}

func TestWatchInstanceReportsVMExit(t *testing.T) {
	healthFile := filepath.Join(t.TempDir(), "health")

	orig := healthFilePath
	healthFilePath = healthFile

	t.Cleanup(func() { healthFilePath = orig })

	cmd := exec.CommandContext(t.Context(), "true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	errs := make(chan error, 1)

	NewAgent(slog.LevelError).watchInstance(t.Context(), cmd.Process, errs)

	select {
	case err := <-errs:
		if err == nil || !strings.Contains(err.Error(), "vm exited") {
			t.Fatalf("expected a vm exited error, got %v", err)
		}
	default:
		t.Fatal("vm exit was not reported")
	}

	b, err := os.ReadFile(healthFile) //nolint: gosec // test temp file
	if err != nil || string(b) != boxenconstants.HealthStatusVMExited {
		t.Fatalf("expected vm exited health, got %q, %v", b, err)
	}
}

func TestWatchInstanceIgnoresShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	cmd := exec.CommandContext(ctx, "sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	cancel()

	errs := make(chan error, 1)

	NewAgent(slog.LevelError).watchInstance(ctx, cmd.Process, errs)

	select {
	case err := <-errs:
		t.Fatalf("shutdown should not be reported as a failure, got %v", err)
	default:
	}
}
