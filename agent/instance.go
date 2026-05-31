package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenprofile "github.com/carlmontanari/boxen/profile"
)

func (a *Agent) startInstance(ctx context.Context, isPackaging bool) (*os.Process, error) {
	var err error

	a.stdoutF, err = os.Create("instance_stdout.log")
	if err != nil {
		return nil, err
	}

	launchArgs, err := boxenprofile.QemuArgsFromProfile(a.p, a.f, isPackaging)
	if err != nil {
		return nil, err
	}

	a.l.Info("starting vm", "command", qemuBinary, "args", launchArgs)

	var stderrBuf bytes.Buffer

	cmd := exec.CommandContext(ctx, qemuBinary, launchArgs...) //nolint: gosec
	cmd.Stdout = a.stdoutF
	cmd.Stderr = &stderrBuf

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	errs := make(chan error, 1)
	stderrIgnore := []string{}
	if a.p.Packaging != nil {
		stderrIgnore = a.p.Packaging.StdErrIgnore
	}

	go func() {
		for {
			time.Sleep(stderrCheckInterval)

			stderrOut := stderrBuf.String()

			if stderrOut == "" {
				continue
			}

			a.l.Debug("read from stderr", "content", stderrOut)

			lines := strings.Split(stderrOut, "\n") //nolint: modernize

			for _, line := range lines {
				if slices.ContainsFunc(
					stderrIgnore,
					func(sub string) bool {
						return strings.Contains(line, sub)
					},
				) {
					break
				}

				errs <- fmt.Errorf(
					"%w: stderr contains output, assuming failure. stderr: %q",
					boxenerrors.ErrBoxen,
					stderrOut,
				)

				return
			}
		}
	}()

	select {
	case err := <-errs:
		return nil, err
	case <-time.After(stderrCheckDuration):
		return cmd.Process, nil
	}
}
