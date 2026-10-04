package agent

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenprofile "github.com/carlmontanari/boxen/profile"
	"github.com/google/uuid"
)

const instanceStatePermissions = 0o644

// runPrepareDisk creates the qcow2 overlay the VM writes to, backed by the packaged disk. The
// packaged disk stays untouched in the image layer, so starting a container does not copy the
// whole disk into the container layer. A restarted container keeps using its existing overlay,
// and so keeps the guest's changes.
func (a *Agent) runPrepareDisk(ctx context.Context) error {
	_, err := os.Stat(boxenconstants.RunDiskFilename)
	if err == nil {
		a.l.Info("reusing disk overlay from a previous start", "disk",
			boxenconstants.RunDiskFilename)

		return nil
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	a.l.Info(
		"creating disk overlay",
		"disk", boxenconstants.RunDiskFilename,
		"backingDisk", boxenconstants.DiskFilename,
	)

	// create under a temporary name so an interrupted create is not reused on the next start
	tmpDisk := boxenconstants.RunDiskFilename + ".tmp"

	out, err := exec.CommandContext( //nolint: gosec
		ctx,
		qemuImgBinary,
		"create",
		"-q",
		"-f", "qcow2",
		"-F", "qcow2",
		"-b", boxenconstants.DiskFilename,
		tmpDisk,
	).CombinedOutput()
	if err != nil {
		_ = os.Remove(tmpDisk)

		return fmt.Errorf(
			"%w: failed creating disk overlay: %w: %s",
			boxenerrors.ErrBoxen,
			err,
			strings.TrimSpace(string(out)),
		)
	}

	return os.Rename(tmpDisk, boxenconstants.RunDiskFilename)
}

// runResolveInstanceUUID sets the instance identity of the container and, from it, the VM system
// UUID and the instance MAC. The instance id is generated on the first start and stored for the
// next starts, so the identity survives container restarts. The VM system UUID is UUID from the
// environment when set, otherwise the UUID the profile reads from a file (run.uuidFrom), otherwise
// the instance id. Stable UUIDs keep guest serial numbers and UUID bound licenses intact.
func (a *Agent) runResolveInstanceUUID() error {
	instanceID, err := a.resolveInstanceID()
	if err != nil {
		return err
	}

	a.p.InstanceMAC = boxenprofile.InstanceMAC(instanceID)

	vmUUID, err := a.resolveVMUUID()
	if err != nil {
		return err
	}

	if vmUUID == "" {
		vmUUID = instanceID
	}

	a.p.InstanceUUID = vmUUID

	return nil
}

// resolveInstanceID returns the id stored by a previous start of this container, or a new one that
// is stored for the next start.
func (a *Agent) resolveInstanceID() (string, error) {
	b, err := os.ReadFile(boxenconstants.InstanceUUIDFilename)
	if err == nil {
		parsed, parseErr := uuid.Parse(strings.TrimSpace(string(b)))
		if parseErr == nil {
			return parsed.String(), nil
		}

		a.l.Warn("ignoring invalid stored instance uuid", "error", parseErr.Error())
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	instanceID := uuid.NewString()

	err = os.WriteFile(
		boxenconstants.InstanceUUIDFilename,
		[]byte(instanceID+"\n"),
		instanceStatePermissions,
	)
	if err != nil {
		return "", err
	}

	return instanceID, nil
}

// resolveVMUUID returns the VM system UUID from the environment or from the profile's UUID file,
// or an empty string when neither sets one.
func (a *Agent) resolveVMUUID() (string, error) {
	envUUID := os.Getenv(boxenconstants.EnvClabUUID)
	if envUUID != "" {
		parsed, err := uuid.Parse(envUUID)
		if err != nil {
			return "", fmt.Errorf(
				"%w: invalid %s value %q: %w",
				boxenerrors.ErrBoxen,
				boxenconstants.EnvClabUUID,
				envUUID,
				err,
			)
		}

		return parsed.String(), nil
	}

	if a.p.Run == nil || a.p.Run.UUIDFrom == nil {
		return "", nil
	}

	source := a.p.Run.UUIDFrom

	value, ok, err := source.Match()
	if err != nil {
		return "", err
	}

	if !ok {
		a.l.Info("vm uuid source has no uuid, using the instance uuid", "file", source.File)

		return "", nil
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		return "", fmt.Errorf(
			"%w: invalid uuid %q in %s: %w",
			boxenerrors.ErrBoxen,
			value,
			source.File,
			err,
		)
	}

	a.l.Info("using the vm uuid from file", "file", source.File)

	return parsed.String(), nil
}

// resolveStartupConfigFile returns the first of the profile's startup config files that exists,
// or an empty string when there is none.
func (a *Agent) resolveStartupConfigFile() (string, error) {
	for _, path := range a.p.Run.GetStartupConfigFiles() {
		info, err := os.Stat(path)
		if err == nil {
			if info.IsDir() {
				return "", fmt.Errorf(
					"%w: startup config path %q is a directory",
					boxenerrors.ErrBoxen,
					path,
				)
			}

			return path, nil
		}

		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}

	return "", nil
}
