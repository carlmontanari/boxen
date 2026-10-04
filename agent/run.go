package agent

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenprofile "github.com/carlmontanari/boxen/profile"
	boxenutil "github.com/carlmontanari/boxen/util"
	"go.yaml.in/yaml/v4"
)

const (
	// defaultIntfWaitTimeout bounds the wait for containerlab data interfaces before starting the
	// vm.
	defaultIntfWaitTimeout = 2 * time.Minute
	intfPollInterval       = 250 * time.Millisecond
)

// Run runs the packaged container -- starting the vm, handling containerlab inputs, etc.
func (a *Agent) Run(
	ctx context.Context,
	username,
	password,
	hostname,
	connectionMode string,
) error {
	a.l.Info("boxen run starting...")

	err := a.runLoadProfile()
	if err != nil {
		return err
	}

	a.f = boxenprofile.NewFormatters(username, password, hostname, connectionMode, a.p, false)

	defer func() {
		if a.stdoutF == nil {
			return
		}

		_ = a.stdoutF.Close()
	}()

	errs := make(chan error, 1)

	go a.startRun(ctx, errs)

	select {
	// here we just wait for the error or done, because we block on the context in the run
	// method so we can defer killing the process if we get cancellation
	case err := <-errs:
		a.l.Error("received error from run process, exiting", "error", err.Error())

		return err
	case <-a.done:
		a.l.Info("done reported, exiting")

		return nil
	}
}

func (a *Agent) startRun(ctx context.Context, errs chan error) {
	// mark the node as booting so the healthcheck reports unhealthy until the run
	// process completes successfully; best-effort, non-fatal.
	if err := a.writeHealth(boxenconstants.HealthStatusBooting); err != nil {
		a.l.Error("failed writing booting health status", "error", err.Error())
	}

	err := a.runPrepare(ctx)
	if err != nil {
		errs <- err

		return
	}

	// no need to capture the process to kill as we passed the root ctx so itll cancel anyway
	// if we catch a sigint/sigkill
	proc, err := a.startInstance(ctx, false)
	if err != nil {
		errs <- err

		return
	}

	go a.watchInstance(ctx, proc, errs)

	// start the tc service that stitches the container interfaces to the vm taps;
	// this only runs during `run` (not packaging)
	a.startTCService(ctx)

	err = a.openConsoleConn(ctx, "run.console.log")
	if err != nil {
		errs <- err

		return
	}

	err = a.runProcesses(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.runStartupConfig(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.closeConsoleConn(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.writeHealth(boxenconstants.HealthStatusRunning)
	if err != nil {
		errs <- err

		return
	}

	a.l.Info("run process completed successfully; marked healthy")

	<-ctx.Done()

	a.done <- struct{}{}
}

// runPrepare does everything that has to happen before the vm starts.
func (a *Agent) runPrepare(ctx context.Context) error {
	for _, prepare := range []func(context.Context) error{
		a.runClabNICProvisionDelay,
		a.runClabStartDelay,
		a.runPreCommands,
		a.runPrepareDisk,
		func(context.Context) error { return a.runResolveInstanceUUID() },
	} {
		err := prepare(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) runLoadProfile() error {
	b, err := os.ReadFile(profileFilename)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(b, a.p)
	if err != nil {
		return err
	}

	return nil
}

func (a *Agent) runClabStartDelay(ctx context.Context) error {
	startDelaySeconds := boxenutil.GetEnvIntOrDefault(
		boxenconstants.EnvClabBootDelay,
		0,
	)

	if startDelaySeconds == 0 {
		return nil
	}

	delay := time.Second * time.Duration(startDelaySeconds)

	a.l.Info("delaying instance start", "duration", delay)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		a.l.Debug("startup delay done, continuing")

		return nil
	}
}

func (a *Agent) runPreCommands(ctx context.Context) error {
	a.l.Info("handling run pre commands")

	for _, command := range a.p.PreRunCommands {
		err := a.invokeCommand(ctx, command)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) runClabNICProvisionDelay(ctx context.Context) error {
	clabIntfCount := boxenutil.GetEnvIntOrDefault(boxenconstants.EnvClabIntfs, 0)

	if clabIntfCount == 0 {
		return nil
	}

	timeout, err := intfWaitTimeout()
	if err != nil {
		return err
	}

	intfPrefix := boxenutil.ClabIntfPrefix()

	a.l.Info(
		"waiting for clab nics to be provisioned",
		"count", clabIntfCount,
		"timeout", timeout,
	)

	ticker := time.NewTicker(intfPollInterval)
	defer ticker.Stop()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	var provisionedNics []string

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			// CLAB_INTFS is fixed when the container is created, so it overcounts once links are
			// removed from a running node; the tc service wires late interfaces when they appear
			a.l.Warn(
				"not all clab nics were provisioned in time, starting the vm anyway",
				"expected", clabIntfCount,
				"provisioned", max(len(provisionedNics)-1, 0),
			)

			return nil
		case <-ticker.C:
			provisionedNics, err = filepath.Glob(fmt.Sprintf("/sys/class/net/%s*", intfPrefix))
			if err != nil {
				return err
			}

			if len(provisionedNics) >= clabIntfCount+1 {
				a.l.Debug("all expected clab interfaces provisioned")

				return nil
			}
		}
	}
}

func intfWaitTimeout() (time.Duration, error) {
	v := os.Getenv(boxenconstants.EnvIntfWaitTimeout)
	if v == "" {
		return defaultIntfWaitTimeout, nil
	}

	timeout, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf(
			"%w: invalid %s value %q: %w",
			boxenerrors.ErrBoxen,
			boxenconstants.EnvIntfWaitTimeout,
			v,
			err,
		)
	}

	return timeout, nil
}

// watchInstance reports the VM exiting on its own (crash or guest power off) as a run failure,
// so the node becomes unhealthy and the container exits instead of looking healthy without a VM.
func (a *Agent) watchInstance(ctx context.Context, proc *os.Process, errs chan<- error) {
	state, err := proc.Wait()
	if ctx.Err() != nil {
		// the vm is stopped because the container is shutting down
		return
	}

	if healthErr := a.writeHealth(boxenconstants.HealthStatusVMExited); healthErr != nil {
		a.l.Error("failed writing vm exited health status", "error", healthErr.Error())
	}

	if err == nil {
		err = fmt.Errorf("%w: vm exited: %s", boxenerrors.ErrBoxen, state.String())
	}

	select {
	case errs <- err:
	default:
	}
}

func (a *Agent) runProcesses(ctx context.Context) error {
	return a.runSteps(ctx, "run process", a.p.Run.Process)
}

func (a *Agent) runStartupConfig(ctx context.Context) error {
	a.l.Info("handling startup config")

	_, err := os.Stat(boxenconstants.StartupConfigFilePath)
	if errors.Is(err, fs.ErrNotExist) {
		a.l.Debug("startup config file not present, nothing to do")

		return nil
	}

	return a.runSteps(ctx, "run configProcess", a.p.Run.ConfigProcess)
}
