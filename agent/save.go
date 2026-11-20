package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenprofile "github.com/carlmontanari/boxen/profile"
)

const (
	saveConsoleOpenTimeout = time.Minute
	savedConfigPermissions = 0o644
)

// Save records the running configuration of a running node with the profile's save process and
// writes it to the node's startup config file, returning that file's path. It runs alongside the
// `boxen run` process, typically through `docker exec`, and needs the serial console: it fails
// while the node is still provisioning or when another client holds the console.
func (a *Agent) Save(ctx context.Context, username, password, hostname string) (string, error) {
	a.l.Info("boxen save starting...")

	target, err := a.save(ctx, username, password, hostname)
	if err != nil {
		a.l.Error("boxen save failed", "error", err.Error())

		return "", err
	}

	return target, nil
}

func (a *Agent) save(ctx context.Context, username, password, hostname string) (string, error) {
	err := a.runLoadProfile()
	if err != nil {
		return "", err
	}

	if a.p.Run == nil || len(a.p.Run.SaveProcess) == 0 {
		return "", fmt.Errorf(
			"%w: profile %q does not define run.saveProcess",
			boxenerrors.ErrBoxen,
			a.p.Name,
		)
	}

	err = Health()
	if err != nil {
		return "", fmt.Errorf(
			"%w: node is not ready, save is possible once provisioning completed: %w",
			boxenerrors.ErrBoxen,
			err,
		)
	}

	a.startupConfigFile, err = a.resolveStartupConfigFile()
	if err != nil {
		return "", err
	}

	a.f = boxenprofile.NewFormatters(username, password, hostname, "", a.p, false)
	a.f.SetStartupConfigFile(a.startupConfigFile)

	openCtx, cancel := context.WithTimeout(ctx, saveConsoleOpenTimeout)
	defer cancel()

	err = a.openConsoleConn(openCtx, "save.console.log")
	if err != nil {
		return "", fmt.Errorf(
			"%w: failed opening the console, it may be in use by another session: %w",
			boxenerrors.ErrBoxen,
			err,
		)
	}

	defer func() {
		if closeErr := a.closeConsoleConn(context.WithoutCancel(ctx)); closeErr != nil {
			a.l.Warn("failed closing console connection", "error", closeErr.Error())
		}
	}()

	err = a.runSteps(ctx, "save process", a.p.Run.SaveProcess)
	if err != nil {
		return "", err
	}

	if a.captured.Len() == 0 {
		return "", fmt.Errorf(
			"%w: save process captured no configuration",
			boxenerrors.ErrBoxen,
		)
	}

	target := a.startupConfigFile
	if target == "" {
		target = a.p.Run.GetStartupConfigFiles()[0]
	}

	err = writeFileAtomic(target, []byte(a.captured.String()))
	if err != nil {
		return "", err
	}

	a.l.Info("saved configuration", "file", target, "bytes", a.captured.Len())

	return target, nil
}

// writeFileAtomic replaces path with content without leaving a partial file behind; the temporary
// file lives next to the target so the rename stays on the same mount.
func writeFileAtomic(path string, content []byte) error {
	dir := filepath.Dir(path)

	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".boxen-*")
	if err != nil {
		return err
	}

	tmpName := f.Name()

	_, err = f.Write(content)

	err = errors.Join(err, f.Chmod(savedConfigPermissions), f.Close())
	if err == nil {
		err = os.Rename(tmpName, path)
	}

	if err != nil {
		_ = os.Remove(tmpName)

		return fmt.Errorf("%w: failed writing %q: %w", boxenerrors.ErrBoxen, path, err)
	}

	return nil
}
