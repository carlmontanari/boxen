package boxen

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	boxenprotov1 "github.com/carlmontanari/boxen/proto/v1"
	"google.golang.org/grpc"
)

// Filer is rpc endpoint for the builder to ask for files to be streamed to it from the server.
func (b *Boxen) Filer(
	req *boxenprotov1.FilerRequest,
	stream grpc.ServerStreamingServer[boxenprotov1.FilerResponse],
) error {
	filename := req.GetFile()

	resolvedFilename := resolveFilePath(b.disk, filename)

	b.l.Info("filer request received", "filename", resolvedFilename)

	f, err := os.Open(resolvedFilename) //nolint: gosec
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

// resolves the file at f -- if f exists, great, if not we check in the same directory that the
// users root disk is (d).
func resolveFilePath(d, f string) string {
	_, err := os.Stat(f)
	if err == nil {
		return f
	}

	maybeF := fmt.Sprintf("%s/%s", filepath.Dir(d), filepath.Base(f))

	_, err = os.Stat(maybeF)
	if err == nil {
		return maybeF
	}

	return ""
}
