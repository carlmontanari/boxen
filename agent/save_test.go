package agent

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "startup-config.cfg")

	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomic(target, []byte("new\n")); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(target) //nolint: gosec // test temp file
	if err != nil || string(b) != "new\n" {
		t.Fatalf("got %q, %v", b, err)
	}

	stat, err := os.Stat(target)
	if err != nil || stat.Mode().Perm() != savedConfigPermissions {
		t.Fatalf("unexpected mode %v, %v", stat.Mode(), err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temporary file left behind: %v", entries)
	}

	if err := writeFileAtomic(filepath.Join(dir, "missing", "cfg"), nil); err == nil {
		t.Fatal("expected an error for a missing directory")
	}
}

func TestSaveRequiresSaveProcess(t *testing.T) {
	t.Chdir(t.TempDir())

	profile := "name: test\nrun:\n  process: []\n"
	if err := os.WriteFile(profileFilename, []byte(profile), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := NewAgent(slog.LevelError).Save(t.Context(), "", "", "")
	if err == nil || !strings.Contains(err.Error(), "run.saveProcess") {
		t.Fatalf("expected a missing save process error, got %v", err)
	}
}

func TestSaveRequiresRunningNode(t *testing.T) {
	t.Chdir(t.TempDir())

	orig := healthFilePath
	healthFilePath = filepath.Join(t.TempDir(), "health")

	t.Cleanup(func() { healthFilePath = orig })

	if err := os.WriteFile(healthFilePath, []byte("1 booting"), 0o600); err != nil {
		t.Fatal(err)
	}

	profile := "name: test\nrun:\n  saveProcess:\n" +
		"    - type: wait\n      wait:\n        duration: 1s\n"
	if err := os.WriteFile(profileFilename, []byte(profile), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := NewAgent(slog.LevelError).Save(t.Context(), "", "", "")
	if err == nil || !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("expected a node not ready error, got %v", err)
	}
}
