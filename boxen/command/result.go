package command

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/carlmontanari/boxen/boxen/util"
)

const (
	checkErrDuration = 10
	checkErrInterval = 250
)

type Result struct {
	Proc  *exec.Cmd
	Stdin io.WriteCloser
	// stdout and stderr are internal buffers of the stdout/stderr data. Users can of course provide
	// additional io.Writer implementations which will be added to the io.MultiWriter that gets set
	// as the stdout/stderr writers.
	stdout    *util.LockingWriterReader
	stderr    *util.LockingWriterReader
	stderrInt *util.LockingWriterReader
}

type checkArgs struct {
	ignore   [][]byte
	isError  [][]byte
	duration time.Duration
	interval time.Duration
}

func (r *Result) setIO(a *args) error {
	// set the stderr/stderrInt/stdout *before* trying to set stdin -- if stdin fails we need
	// the stderr/stderrInt/stdout set for subsequent CheckStdErr/ReadStdout/ReadStderr methods to
	// not fail
	ow := []io.Writer{r.stdout}

	if a.stdOut != nil {
		ow = append(ow, a.stdOut)
	}

	r.Proc.Stdout = io.MultiWriter(ow...)

	ew := []io.Writer{r.stderr, r.stderrInt}

	if a.stdErr != nil {
		ew = append(ew, a.stdErr)
	}

	r.Proc.Stderr = io.MultiWriter(ew...)

	stdin, err := r.Proc.StdinPipe()
	if err != nil {
		return err
	}

	r.Stdin = stdin

	return nil
}

// ReadStdout returns whatever standard out is currently in the internal stdout buffer.
func (r *Result) ReadStdout() ([]byte, error) {
	return r.stdout.Read()
}

// ReadStderr returns whatever standard out is currently in the internal stderr buffer.
func (r *Result) ReadStderr() ([]byte, error) {
	return r.stderrInt.Read()
}

// CheckStdErr blocks while reading stderr output from a process. CheckOptions define what is safe
// to ignore or what constitutes an error. You can modify the check duration and sleep interval of
// the check with CheckOption settings.
func (r *Result) CheckStdErr(opts ...CheckOption) error {
	if r.Proc.ProcessState != nil && r.Proc.ProcessState.Exited() {
		return fmt.Errorf("%w: cannot check stsderr, process already exited", util.ErrCommandError)
	}

	a := &checkArgs{
		duration: checkErrDuration * time.Second,
		interval: checkErrInterval * time.Millisecond,
	}

	for _, o := range opts {
		err := o(a)

		if err != nil {
			return err
		}
	}

	c := make(chan error, 1)

	go func() {
		for {
			b, err := r.stderrInt.Read()

			if err != nil && !errors.Is(err, io.EOF) {
				c <- err

				return
			}

			if a.isError != nil && util.ByteSliceContains(a.isError, b) {
				c <- fmt.Errorf("%w: stderr contains output explicitly marked as bad", util.ErrCommandError)

				return
			}

			if a.ignore == nil && !util.ByteSliceAllNull(b) {
				c <- fmt.Errorf("%w: stderr contains output but it shouldn't", util.ErrCommandError)

				return
			}

			time.Sleep(a.interval)
		}
	}()

	select {
	case err := <-c:
		return err
	case <-time.After(a.duration):
		return nil
	}
}
