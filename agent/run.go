package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenutil "github.com/carlmontanari/boxen/util"
)

const (
	socatBinary = "socat"
)

// Run runs the packaged container -- starting the vm, handling containerlab inputs, etc.
func (a *Agent) Run(ctx context.Context) error {
	a.l.Info("boxen run starting...")

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

	// no need to capture the process to kill as we passed the root ctx so itll cancel anyway
	// if we catch a sigint/sigkill
	_, err = a.startInstance(ctx, false)
	if err != nil {
		errs <- err

		return
	}

	// TODO wait for console ready i think
	// TODO install startup config (is this only from og boxen things or does clab do this too?)

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

func (a *Agent) runClabNICProvisionDelay(ctx context.Context) error {
	clabIntfCount := boxenutil.GetEnvIntOrDefault("CLAB_INTFS", 0)

	if clabIntfCount == 0 {
		return nil
	}

	a.l.Info("waiting for clab nics to be priviosined", "count", clabIntfCount)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			provisionedNics, err := filepath.Glob("/sys/class/net/eth*")
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
