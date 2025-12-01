package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenprofile "github.com/carlmontanari/boxen/profile"
	boxenprotov1 "github.com/carlmontanari/boxen/proto/v1"
	boxenutilringbuffer "github.com/carlmontanari/boxen/util/ringbuffer"
	scrapligocli "github.com/scrapli/scrapligo/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
)

const (
	qemuImgBinary = "qemu-img"
	qemuBinary    = "qemu-system-x86_64"

	stderrCheckInterval = time.Second
	stderrCheckDuration = 10 * time.Second

	readUntilRingBufSize = 1_000
)

// Package begins the packaging process for the container.
func (a *Agent) Package(ctx context.Context, host string) error {
	a.l.Info("boxen package starting...")

	defer func() {
		if a.stdoutF == nil {
			return
		}

		_ = a.stdoutF.Close()
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

	errs := make(chan error, 1)

	go a.runPackaging(ctx, errs)

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

func (a *Agent) runPackaging(ctx context.Context, errs chan error) {
	err := a.packageGetProfile(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.packageGetFiles(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.packageConvertDisk(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.packageStartInstance(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.openConsoleConn(ctx)
	if err != nil {
		errs <- err

		return
	}

	err = a.packageRunProcess(ctx)
	if err != nil {
		errs <- err

		return
	}

	fmt.Println("PACKAGING COMPLETE")
	panic("poop")
}

func (a *Agent) packageGetProfile(ctx context.Context) error {
	a.l.Info("requesting packaging info")

	packagingInfoResp, err := a.s.Builder(
		ctx,
		&boxenprotov1.BuilderRequest{
			Request: &boxenprotov1.BuilderRequest_PackageRequest{
				PackageRequest: &boxenprotov1.PackageInfoRequest{},
			},
		},
	)
	if err != nil {
		a.l.Error("failed builder response from server", "error", err.Error())

		return err
	}

	a.l.s = a.s

	r := packagingInfoResp.GetPackageResponse()

	p := &boxenprofile.Profile{}

	err = yaml.Unmarshal(r.GetProfile(), p)
	if err != nil {
		return err
	}

	a.p = p
	a.disk = r.GetDisk()

	a.l.Info("packaging info received", "profile", a.p.Name, "disk", filepath.Base(a.disk))

	return nil
}

func (a *Agent) packageGetFiles(ctx context.Context) error {
	a.l.Info("requesting files from server")

	err := a.packageGetFile(ctx, a.disk)
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
		_ = os.Remove(filepath.Base(a.disk))
	}()

	cmd := exec.CommandContext( //nolint: gosec
		ctx,
		qemuImgBinary,
		"convert",
		"-O",
		"qcow2",
		filepath.Base(a.disk),
		"disk.qcow2",
	)

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (a *Agent) packageStartInstance(ctx context.Context) error {
	var err error

	a.stdoutF, err = os.Create("instance_stdout.log")
	if err != nil {
		return err
	}

	launchArgs, err := boxenprofile.QemuArgsFromProfile(a.p, true)
	if err != nil {
		return err
	}

	a.l.Info("starting vm", "command", qemuBinary, "args", launchArgs)

	var stderrBuf bytes.Buffer

	cmd := exec.CommandContext(ctx, qemuBinary, launchArgs...) //nolint: gosec
	cmd.Stdout = a.stdoutF
	cmd.Stderr = &stderrBuf

	err = cmd.Start()
	if err != nil {
		return err
	}

	errs := make(chan error, 1)

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
					a.p.Packaging.StdErrIgnore,
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
		return err
	case <-time.After(stderrCheckDuration):
		return nil
	}
}

func (a *Agent) packageRunProcess(ctx context.Context) error {
	for idx := range a.p.Packaging.Process {
		step := &a.p.Packaging.Process[idx]

		a.l.Info("starting package process", "step", idx, "type", step.Type)

		var err error

		switch step.Type {
		case boxenprofile.StepTypePrompts:
			err = a.packageRunProcessStepPrompts(ctx, step)
		case boxenprofile.StepTypeReadUntil:
			err = a.packageRunProcessStepReadUntil(ctx, step)
		case boxenprofile.StepTypeWrite:
			err = a.packageRunProcessStepWrite(ctx, step)
		case boxenprofile.StepTypeWait:
			err = a.packageRunProcessStepWait(ctx, step)
		default:
			panic("unimplemented step type")
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) packageRunProcessStepPrompts(ctx context.Context, step *boxenprofile.Step) error {
	t, err := time.ParseDuration(step.Prompts.Timeout)
	if err != nil {
		a.l.Error(
			"failed parsing timeout duration for prompt step",
			"timeout",
			step.Prompts.Timeout,
			"error",
			err.Error(),
		)

		return err
	}

	a.l.Info("handling prompts", "prompts", step.Prompts.Prompts, "timeout", step.Prompts.Timeout)

	cbs := make([]*scrapligocli.ReadCallback, len(step.Prompts.Prompts))

	for idx, p := range step.Prompts.Prompts {
		a.l.Debug(
			"building prompts callback",
			"prompt",
			p.Prompt,
			"response",
			p.Response,
			"completes",
			p.Completes,
		)

		var opts []scrapligocli.Option

		opts = append(
			opts,
			scrapligocli.WithSearchDepth(256),
		)

		if p.Prompt.Contains != "" {
			opts = append(
				opts,
				scrapligocli.WithContains(p.Prompt.Contains),
			)
		}

		if p.Prompt.ContainsPattern != "" {
			opts = append(
				opts,
				scrapligocli.WithContainsPattern(p.Prompt.ContainsPattern),
			)
		}

		if p.Prompt.NotContains != "" {
			opts = append(
				opts,
				scrapligocli.WithNotContains(p.Prompt.NotContains),
			)
		}

		if p.Once {
			opts = append(
				opts,
				scrapligocli.WithOnce(),
			)
		}

		if p.Completes {
			opts = append(
				opts,
				scrapligocli.WithCompletes(),
			)
		}

		cbs[idx] = scrapligocli.NewReadCallback(
			fmt.Sprintf("prompts step idx %d", idx),
			func(ctx context.Context, c *scrapligocli.Cli) error {
				if p.Hidden {
					return c.WriteAndReturn(p.Response)
				}

				err = c.Write(p.Response)
				if err != nil {
					return err
				}

				if p.Hidden {
					return c.WriteReturn()
				}

				err = readUntil(ctx, c, p.Response)
				if err != nil {
					return err
				}

				return c.WriteReturn()
			},
			opts...,
		)
	}

	taskCtx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	_, err = a.conn.ReadWithCallbacks(taskCtx, "", cbs...)
	if err != nil {
		return err
	}

	return nil
}

func (a *Agent) packageRunProcessStepReadUntil(ctx context.Context, step *boxenprofile.Step) error {
	b := boxenutilringbuffer.NewRingBuffer(readUntilRingBufSize)

	t, err := time.ParseDuration(step.ReadUntil.Timeout)
	if err != nil {
		a.l.Error(
			"failed parsing timeout duration for read until step",
			"timeout",
			step.ReadUntil.Timeout,
			"error",
			err.Error(),
		)

		return err
	}

	a.l.Info("reading until", "until", step.ReadUntil.Until, "timeout", step.ReadUntil.Timeout)

	start := time.Now()
	end := start.Add(t)
	doneOrErr := make(chan error)

	go func() {
		for {
			time.Sleep(time.Second)

			// ensure this goroutine exits. we cant have this continue reading from the
			// session buf screwing up other reads (esp since this is going around the
			// operation loop in libscrapli ffi bits)
			if time.Now().After(end) {
				return
			}

			r, err := a.conn.Read()
			if err != nil {
				doneOrErr <- err

				return
			}

			_, err = b.Write(r)
			if err != nil {
				doneOrErr <- err

				return
			}

			a.l.Debug("reading until", "until", step.ReadUntil.Until, "content", string(b.Content))

			check, err := step.ReadUntil.Until.Check(b.Content)
			if err != nil {
				doneOrErr <- err

				return
			}

			if check {
				doneOrErr <- err

				return
			}
		}
	}()

	select {
	case err = <-doneOrErr:
		if err != nil {
			return err
		}

		return nil
	case <-time.After(t):
		return fmt.Errorf("%w: read until timeout expired", boxenerrors.ErrBoxen)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Agent) packageRunProcessStepWrite(ctx context.Context, step *boxenprofile.Step) error {
	a.l.Info("writing to console", "content", step.Write.Content)

	iter := strings.SplitSeq(step.Write.Content, "\n")

	for s := range iter {
		a.l.Info("writing to console", "content", s)

		if step.Write.Hidden {
			err := a.conn.WriteAndReturn(s)
			if err != nil {
				return err
			}

			continue
		}

		err := a.conn.Write(s)
		if err != nil {
			return err
		}

		err = readUntil(ctx, a.conn, s)
		if err != nil {
			return err
		}

		err = a.conn.WriteReturn()
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) packageRunProcessStepWait(ctx context.Context, step *boxenprofile.Step) error {
	d, err := time.ParseDuration(step.Wait.Duration)
	if err != nil {
		a.l.Error(
			"failed parsing duration for wait step",
			"timeout",
			step.Wait.Duration,
			"error",
			err.Error(),
		)

		return err
	}

	a.l.Info("waiting", "duration", step.Wait.Duration)

	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
