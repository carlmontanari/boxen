package boxen

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestProfileCompanionsResolveBesideProfile(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "router.yaml")
	companionPath := filepath.Join(dir, "settings.star")
	if err := os.WriteFile(profilePath,
		[]byte("name: example\nextraFiles:\n  - settings.star\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(companionPath, []byte("user-provided file"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	b := &Boxen{l: slog.New(slog.DiscardHandler), disk: "/elsewhere/disk.qcow2"}
	p, err := b.resolveProfile(profilePath)
	if err != nil || len(p.ExtraFiles) != 1 || p.ExtraFiles[0] != companionPath ||
		resolveFilePath(b.disk, p.ExtraFiles[0]) != companionPath {
		t.Fatalf("profile-adjacent companion was not resolved: profile=%v error=%v", p, err)
	}
}
