package boxen

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	boxenassets "github.com/carlmontanari/boxen/assets"
	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenprotov1 "github.com/carlmontanari/boxen/proto/v1"
	"google.golang.org/grpc"
)

// Filer is rpc endpoint for the builder to ask for files to be streamed to it from the server.
func (b *Boxen) Filer(
	req *boxenprotov1.FilerRequest,
	stream grpc.ServerStreamingServer[boxenprotov1.FilerResponse],
) error {
	filename := req.GetFile()

	b.l.Info("filer request received", "filename", filename)

	f, err := openFile(b.disk, filename)
	if err != nil {
		return err
	}

	defer func() {
		_ = f.Close()
	}()

	buf := make([]byte, 32_768) //nolint: mnd

	for {
		n, err := f.Read(buf)
		if errors.Is(err, io.EOF) {
			err = stream.Send(
				&boxenprotov1.FilerResponse{
					Data: nil,
					Done: true,
				},
			)
			if err != nil {
				return err
			}

			break
		} else if err != nil {
			return err
		}

		err = stream.Send(
			&boxenprotov1.FilerResponse{
				Data: buf[:n],
				Done: false,
			},
		)
		if err != nil {
			return err
		}
	}

	b.l.Info("filer sent file", "filename", filename)

	return nil
}

// openFile uses embedded companions for profile names and host files for custom
// profiles, whose extraFiles paths are made absolute by loadProfileFile.
func openFile(disk, filename string) (fs.File, error) {
	if filename != disk && !filepath.IsAbs(filename) {
		f, err := boxenassets.Assets.Open("profiles/" + filepath.ToSlash(filename))
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	path, err := resolveFilePath(disk, filename)
	if err != nil {
		return nil, err
	}

	return os.Open(path) //nolint:gosec // Profile-controlled paths.
}

// resolves the file at f -- if f exists, great, if not we check in the same directory that the
// users root disk is (d).
func resolveFilePath(d, f string) (string, error) {
	maybeF := filepath.Join(filepath.Dir(d), filepath.Base(f))
	for _, filename := range []string{f, maybeF} {
		info, err := os.Stat(filename)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return "", fmt.Errorf("%w: file %q is a directory", boxenerrors.ErrBoxen, filename)
		}

		return filename, nil
	}

	return "", fmt.Errorf("%w: file %q not found (also checked %q)", os.ErrNotExist, f, maybeF)
}
