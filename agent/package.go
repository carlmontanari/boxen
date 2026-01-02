package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenprofile "github.com/carlmontanari/boxen/profile"
	boxenprotov1 "github.com/carlmontanari/boxen/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
)

const (
	qemuImgBinary  = "qemu-img"
	qemuBinary     = "qemu-system-x86_64"
	sparsifyBinary = "virt-sparsify"
	commandBinary  = "/bin/bash"

	stderrCheckInterval = time.Second
	stderrCheckDuration = 10 * time.Second

	readUntilSearchDepth = 256
	readUntilRingBufSize = 1_000
)

// Package begins the packaging process for the container.
func (a *Agent) Package(ctx context.Context, host string) error {
	a.l.Info("boxen package starting...")

	defer func() {
		if a.stdoutF == nil {
			return
		}

		// close and remove the stdout log since we dont want this leftover in
		// the committed image
		_ = a.stdoutF.Close()
		_ = os.Remove(a.stdoutF.Name())
	}()

	var err error

	a.c, err = grpc.NewClient(
		fmt.Sprintf("%s:%d", host, boxenconstants.DefaultBoxenListenPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		a.l.Error("failed creating grpc client", "error", err.Error())

		return err
	}

	a.s = boxenprotov1.NewBoxenServiceClient(a.c)

	err = a.l.setLogStream(ctx, a.s)
	if err != nil {
		a.l.Error("failed establishing log stream", "error", err.Error())

		return err
	}

	defer func() {
		_, err = a.l.stream.CloseAndRecv()
	}()

	err = a.packageGetProfile(ctx)
	if err != nil {
		return err
	}

	a.f = boxenprofile.NewFormatters("", "", "", "", a.p)

	errs := make(chan error, 1)

	go a.startPackage(ctx, errs)

	select {
	case err = <-errs:
		a.l.Error("received error from packaging process, exiting", "error", err.Error())

		return err
	case <-ctx.Done():
		a.l.Info("context cancelled, exiting")

		return nil
	case <-a.done:
		a.l.Info("done reported, exiting")

		return nil
	}
}

func (a *Agent) startPackage(ctx context.Context, errs chan error) {
	err := a.packageGetFiles(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.packageConvertDisk(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.packagePreCommands(ctx)
	if err != nil {
		errs <- err

		return
	}

	p, err := a.startInstance(ctx, true)
	if err != nil {
		errs <- err

		return
	}

	err = a.openConsoleConn(ctx, "package.console.log")
	if err != nil {
		errs <- err

		return
	}

	err = a.packageProcess(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.closeConsoleConn(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = p.Kill()
	if err != nil {
		errs <- err

		return
	}

	if a.p.Packaging.Shrinkify {
		err = a.packageShrinkify(ctx)
		if err != nil {
			errs <- err

			return
		}
	}

	err = a.packagePostCommands(ctx)
	if err != nil {
		errs <- err

		return
	}

	_, err = a.s.Builder(
		ctx,
		&boxenprotov1.BuilderRequest{
			Request: &boxenprotov1.BuilderRequest_PackageCompleteRequest{
				PackageCompleteRequest: &boxenprotov1.PackageCompleteRequest{},
			},
		},
	)
	if err != nil {
		a.l.Error("failed builder response from server", "error", err.Error())

		errs <- err

		return
	}

	a.done <- struct{}{}
}

func (a *Agent) packageGetProfile(ctx context.Context) error {
	a.l.Info("requesting packaging info")

	packagingInfoResp, err := a.s.Builder(
		ctx,
		&boxenprotov1.BuilderRequest{
			Request: &boxenprotov1.BuilderRequest_PackageInfoRequest{
				PackageInfoRequest: &boxenprotov1.PackageInfoRequest{},
			},
		},
	)
	if err != nil {
		a.l.Error("failed builder response from server", "error", err.Error())

		return err
	}

	a.l.s = a.s

	r := packagingInfoResp.GetPackageInfoResponse()

	p := &boxenprofile.Profile{}

	b := r.GetProfile()

	err = yaml.Unmarshal(b, p)
	if err != nil {
		return err
	}

	// also write it to disk so its available in the final committed image
	err = os.WriteFile(profileFilename, b, profilePermissions)
	if err != nil {
		return err
	}

	a.p = p
	a.p.ResolvedDisk = r.GetDisk()

	a.l.Info(
		"packaging info received", "profile", a.p.Name, "disk", filepath.Base(a.p.ResolvedDisk),
	)

	return nil
}

func (a *Agent) packageGetFiles(ctx context.Context) error {
	a.l.Info("requesting files from server")

	err := a.packageGetFile(ctx, a.p.ResolvedDisk)
	if err != nil {
		return err
	}

	for _, filename := range a.p.ExtraFiles {
		err = a.packageGetFile(ctx, filename)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) packageGetFile(ctx context.Context, filename string) error {
	a.l.Debug("requesting file from server", "file", filename)

	stream, err := a.s.Filer(
		ctx,
		&boxenprotov1.FilerRequest{
			File: filename,
		},
	)
	if err != nil {
		return err
	}

	f, err := os.Create(filepath.Base(filename))
	if err != nil {
		return err
	}

	defer func() {
		_ = f.Close()
	}()

	for {
		chunk, err := stream.Recv()
		if err != nil {
			return err
		}

		_, err = f.Write(chunk.GetData())
		if err != nil {
			return err
		}

		if chunk.GetDone() {
			break
		}
	}

	a.l.Debug("received file from server", "file", filename)

	return nil
}

func (a *Agent) packageConvertDisk(ctx context.Context) error {
	defer func() {
		_ = os.Remove(filepath.Base(a.p.ResolvedDisk))
	}()

	cmd := exec.CommandContext( //nolint: gosec
		ctx,
		qemuImgBinary,
		"convert",
		"-O",
		"qcow2",
		filepath.Base(a.p.ResolvedDisk),
		"disk.qcow2",
	)

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (a *Agent) packagePreCommands(ctx context.Context) error {
	a.l.Info("handling package pre commands")

	for _, command := range a.p.PrePackagingCommands {
		err := a.invokeCommand(ctx, command)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) packageProcess(ctx context.Context) error {
	for idx := range a.p.Packaging.Process {
		step := &a.p.Packaging.Process[idx]

		a.l.Info("starting package process", "step", idx, "type", step.Type)

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

func (a *Agent) packageShrinkify(ctx context.Context) error {
	err := os.Rename("disk.qcow2", "fat.qcow2")
	if err != nil {
		return err
	}

	defer func() {
		_ = os.Remove("fat.qcow2")
		_ = os.RemoveAll("/var/tmp/.guestfs-0")
	}()

	args := []string{"fat.qcow2", "--compress", "disk.qcow2"}

	a.l.Info("starting sparsify", "command", sparsifyBinary, "args", args)

	cmd := exec.CommandContext(ctx, sparsifyBinary, args...)

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (a *Agent) packagePostCommands(ctx context.Context) error {
	a.l.Info("handling package post commands")

	for _, command := range a.p.PostPackagingCommands {
		err := a.invokeCommand(ctx, command)
		if err != nil {
			return err
		}
	}

	return nil
}
