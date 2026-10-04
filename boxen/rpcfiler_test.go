package boxen

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	boxenassets "github.com/carlmontanari/boxen/assets"
	boxenprotov1 "github.com/carlmontanari/boxen/proto/v1"
	"google.golang.org/grpc"
)

type testFilerStream struct {
	grpc.ServerStream

	data []byte
	done bool
}

func (s *testFilerStream) Send(response *boxenprotov1.FilerResponse) error {
	s.data = append(s.data, response.GetData()...)
	s.done = response.GetDone()

	return nil
}

func TestFilerEmbeddedCompanionsAndOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	disk := filepath.Join(dir, "cumulus.qcow2")
	profileDirectory := t.TempDir()
	profilePath := filepath.Join(profileDirectory, "custom.yaml")
	if err := os.WriteFile(profilePath, []byte(`name: custom_cumulus
extraFiles:
  - nvidia_cumulusvx_breakout.star
  - nvidia_cumulusvx_breakout.sh.tmpl
virtualMachine:
  memory: 1024
  serialPortCount: 1
  nicType: virtio-net-pci
packaging: {}
run: {}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	b := &Boxen{l: slog.New(slog.DiscardHandler), disk: disk}
	for _, selection := range []string{"nvidia_cumulusvx", profilePath} {
		p, err := b.resolveProfile(selection)
		if err != nil {
			t.Fatal(err)
		}
		for _, filename := range p.ExtraFiles {
			name := filepath.Base(filename)
			want, err := boxenassets.Assets.ReadFile("profiles/" + name)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				filepath.Join(dir, name),
				[]byte("beside disk"),
				0o600,
			); err != nil {
				t.Fatal(err)
			}
			if selection == profilePath {
				want = []byte("custom override for " + name)
				if err := os.WriteFile(filename, want, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			stream := &testFilerStream{}
			if err := b.Filer(&boxenprotov1.FilerRequest{File: filename}, stream); err != nil {
				t.Fatal(err)
			}
			if !stream.done || !bytes.Equal(stream.data, want) {
				t.Fatalf("profile %q file %q: received %q, want %q, done=%v",
					selection, filename, stream.data, want, stream.done)
			}
		}
	}
}
