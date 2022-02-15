package util

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/carlmontanari/boxen/boxen"
)

// ExpandPath expands user home path in provided path p.
func ExpandPath(p string) string {
	userPath, _ := os.UserHomeDir()

	p = strings.Replace(p, "~", userPath, 1)

	return p
}

// DirectoryExists checks if a given path exists (and is a directory).
func DirectoryExists(d string) bool {
	info, err := os.Stat(d)
	if os.IsNotExist(err) {
		return false
	}

	return info.IsDir()
}

// FileExists checks if a given file exists (and is not a directory).
func FileExists(f string) bool {
	info, err := os.Stat(f)
	if os.IsNotExist(err) {
		return false
	}

	return !info.IsDir()
}

// ResolveFile resolves provided file path.
func ResolveFile(f string) (string, error) {
	expanded := ExpandPath(f)

	if FileExists(expanded) {
		return filepath.Abs(expanded)
	}

	return "", fmt.Errorf("%w: failed resolving file '%s'", ErrInspectionError, f)
}

// CopyFile copies a file source `s` to destination `d`.
func CopyFile(s, d string) error {
	srcPath, err := ResolveFile(s)
	if err != nil {
		return err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}

	defer src.Close()

	dest, err := os.Create(d)
	if err != nil {
		return err
	}

	_, err = io.Copy(dest, src)
	if err != nil {
		return err
	}

	err = dest.Sync()

	return err
}

// CopyAsset copies an asset file source `s` to destination `d`.
func CopyAsset(s, d string) error {
	sFile, err := boxen.Assets.Open(fmt.Sprintf("assets/%s", s))
	if err != nil {
		return err
	}

	sReader := bufio.NewReader(sFile)

	dest, err := os.Create(d)
	if err != nil {
		return err
	}

	_, err = io.Copy(dest, sReader)
	if err != nil {
		return err
	}

	err = dest.Sync()

	return err
}

// CommandExists checks if a command `cmd` exists in the PATH.
func CommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
