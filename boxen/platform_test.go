package boxen

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	boxenerrors "github.com/carlmontanari/boxen/errors"
)

func TestResolveProfileSelection(t *testing.T) {
	t.Chdir(t.TempDir())

	for _, test := range []struct {
		name    string
		profile string
		disk    string
		want    string
	}{
		{
			name: "automatic disk match",
			disk: "cumulus-linux-5.16.5-vx-amd64.qcow2",
			want: "nvidia_cumulusvx",
		},
		{
			name:    "explicit profile overrides disk match",
			profile: "nvidia_cumulusvx",
			disk:    "vEOS-lab-4.35.0F.vmdk",
			want:    "nvidia_cumulusvx",
		},
		{
			name:    "unknown explicit profile",
			profile: "nvidia_cumulus",
			disk:    "cumulus-linux-5.16.5-vx-amd64.qcow2",
		},
		{
			name:    "missing explicit profile path",
			profile: "missing.yaml",
			disk:    "cumulus-linux-5.16.5-vx-amd64.qcow2",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := &Boxen{l: slog.New(slog.DiscardHandler), disk: test.disk}
			p, err := b.resolveProfile(test.profile)
			if test.want == "" {
				if !errors.Is(err, boxenerrors.ErrBoxen) || p != nil ||
					!strings.Contains(err.Error(), test.profile) {
					t.Fatalf("expected profile rejection, got profile=%v error=%v", p, err)
				}

				return
			}
			if err != nil || p.Name != test.want {
				t.Fatalf("expected profile %q, got profile=%v error=%v", test.want, p, err)
			}
		})
	}
}

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
	if err != nil || len(p.ExtraFiles) != 1 || p.ExtraFiles[0] != companionPath {
		t.Fatalf("profile-adjacent companion was not resolved: profile=%v error=%v", p, err)
	}
	resolved, err := resolveFilePath(b.disk, p.ExtraFiles[0])
	if err != nil || resolved != companionPath {
		t.Fatalf("expected companion %q, got %q error=%v", companionPath, resolved, err)
	}
}
