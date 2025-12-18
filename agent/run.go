package agent

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenprofile "github.com/carlmontanari/boxen/profile"
	boxenutil "github.com/carlmontanari/boxen/util"
)

const (
	socatBinary = "socat"
)

type containerlabArgs struct {
	username       string
	password       string
	hostname       string
	connectionMode string
}

// Run runs the packaged container -- starting the vm, handling containerlab inputs, etc.
func (a *Agent) Run(
	ctx context.Context,
	username,
	password,
	hostname,
	connectionMode string,
) error {
	a.l.Info("boxen run starting...")

	defer func() {
		if a.stdoutF == nil {
			return
		}

		_ = a.stdoutF.Close()
	}()

	errs := make(chan error, 1)

	go a.startRun(
		ctx,
		errs,
		containerlabArgs{
			username:       username,
			password:       password,
			hostname:       hostname,
			connectionMode: connectionMode,
		},
	)

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

func (a *Agent) startRun(ctx context.Context, errs chan error, args containerlabArgs) {
	err := a.runLoadProfile()
	if err != nil {
		errs <- err

		return
	}

	err = a.runClabNICProvisionDelay(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.runSocatProcesses(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.runClabStartDelay(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.runPreCommands(ctx)
	if err != nil {
		errs <- err

		return
	}

	// no need to capture the process to kill as we passed the root ctx so itll cancel anyway
	// if we catch a sigint/sigkill
	_, err = a.startInstance(ctx, false)
	if err != nil {
		errs <- err

		return
	}

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

	err = a.runStartupConfig(ctx, args)
	if err != nil {
		errs <- err

		return
	}

	err = a.closeConsoleConn(ctx)
	if err != nil {
		errs <- err

		return
	}

	<-ctx.Done()

	a.done <- struct{}{}
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
	clabIntfCount := boxenutil.GetEnvIntOrDefault("CLAB_INTFS", 0)

	if clabIntfCount == 0 {
		return nil
	}

	intfPrefix := boxenutil.GetEnvStrOrDefault(boxenconstants.EnvClabIntfPrefix, "eth")

	a.l.Info("waiting for clab nics to be priviosined", "count", clabIntfCount)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			provisionedNics, err := filepath.Glob(fmt.Sprintf("/sys/class/net/%s*", intfPrefix))
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

func (a *Agent) runSocatProcesses(ctx context.Context) error {
	for idx := range a.p.VirtualMachine.Management.NatPorts {
		args := make([]string, 2) //nolint: mnd

		args[0] = fmt.Sprintf(
			"%s-LISTEN:%d,fork",
			strings.ToUpper(string(a.p.VirtualMachine.Management.NatPorts[idx].Type)),
			a.p.VirtualMachine.Management.NatPorts[idx].LocalPort,
		)

		externalPort := a.p.VirtualMachine.Management.NatPorts[idx].ExternalPort
		if externalPort == 0 {
			externalPort = a.p.VirtualMachine.Management.NatPorts[idx].LocalPort
		}

		args[1] = fmt.Sprintf(
			"%s:127.0.01:%d",
			strings.ToUpper(string(a.p.VirtualMachine.Management.NatPorts[idx].Type)),
			externalPort,
		)

		cmd := exec.CommandContext(ctx, socatBinary, args...) //nolint: gosec

		err := cmd.Start()
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) runProcesses(ctx context.Context) error {
	for idx := range a.p.Run.Process {
		step := &a.p.Run.Process[idx]

		a.l.Info("starting run process", "step", idx, "type", step.Type)

		var err error

		switch step.Type {
		case boxenprofile.StepTypePrompts:
			err = a.processStepPrompts(ctx, step)
		case boxenprofile.StepTypeReadUntil:
			err = a.processStepReadUntil(ctx, step)
		case boxenprofile.StepTypeWrite:
			err = a.processStepWrite(ctx, step, nil)
		case boxenprofile.StepTypeWait:
			err = a.processStepWait(ctx, step)
		default:
			panic("unimplemented step type")
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) runStartupConfig(ctx context.Context, args containerlabArgs) error {
	a.l.Info("handling startup config")

	_, err := os.Stat(boxenconstants.StartupConfigFilePath)
	if errors.Is(err, fs.ErrNotExist) {
		a.l.Debug("startup config file not present, nothing to do")

		return nil
	}

	for idx := range a.p.Run.ConfigProcess {
		step := &a.p.Run.ConfigProcess[idx]

		a.l.Info("starting run configProcess", "step", idx, "type", step.Type)

		var err error

		switch step.Type {
		case boxenprofile.StepTypePrompts:
			err = a.processStepPrompts(ctx, step)
		case boxenprofile.StepTypeReadUntil:
			err = a.processStepReadUntil(ctx, step)
		case boxenprofile.StepTypeWrite:
			err = a.processStepWrite(ctx, step, &args)
		case boxenprofile.StepTypeWait:
			err = a.processStepWait(ctx, step)
		default:
			panic("unimplemented step type")
		}

		if err != nil {
			return err
		}
	}

	return nil
}
