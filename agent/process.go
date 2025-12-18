package agent

import (
	"context"
	"fmt"
	"iter"
	"os"
	"strings"
	"time"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenprofile "github.com/carlmontanari/boxen/profile"
	boxenutilringbuffer "github.com/carlmontanari/boxen/util/ringbuffer"
	scrapligocli "github.com/scrapli/scrapligo/cli"
)

func (a *Agent) processStepPrompts(ctx context.Context, step *boxenprofile.Step) error {
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
			// because of the way we read off the console the individual reads are *very* likely
			// to be just like a handful of characters, meaning we would very infrequently have the
			// whole thing we are looking for in a single read, so, we need to ensure we are
			// looking back far enough.
			scrapligocli.WithSearchDepth(
				uint64(max(len(p.Prompt.Contains)*2, readUntilSearchDepth)), //nolint:gosec,mnd
			),
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

				err = a.readUntil(ctx, p.Response)
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

func (a *Agent) processStepReadUntil(ctx context.Context, step *boxenprofile.Step) error {
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

func (a *Agent) processStepWrite( //nolint: funlen,gocyclo
	ctx context.Context,
	step *boxenprofile.Step,
	args *containerlabArgs,
) error {
	var writeIterator iter.Seq[string]

	switch {
	case step.Write.Content != "":
		a.l.Info("writing to console", "content", step.Write.Content)

		writeIterator = strings.SplitSeq(step.Write.Content, "\n")
	case step.Write.ContentFromFile != "":
		b, err := os.ReadFile(step.Write.ContentFromFile)
		if err != nil {
			return err
		}

		s := string(b)

		a.l.Info("writing to console", "content", s)

		writeIterator = strings.SplitSeq(s, "\n")
	case step.Write.ContentFromStartupConfig:
		b, err := os.ReadFile(boxenconstants.StartupConfigFilePath)
		if err != nil {
			return err
		}

		s := string(b)

		a.l.Info("writing to console", "content", s)

		writeIterator = strings.SplitSeq(s, "\n")
	case step.Write.ContentFromContainerlabFlags != nil:
		if args == nil {
			return fmt.Errorf(
				"%w: contentFromContainerlabFlags set, but no args provided, "+
					"this is only supported during the `configProcess` phase",
				boxenerrors.ErrBoxen,
			)
		}

		var formatters []any

		for _, formatter := range step.Write.ContentFromContainerlabFlags.Formatters {
			switch formatter {
			case "username":
				formatters = append(formatters, args.username)
			case "password":
				formatters = append(formatters, args.password)
			case "hostname":
				formatters = append(formatters, args.hostname)
			default:
				return fmt.Errorf("%w: invalid formatter %q", boxenerrors.ErrBoxen, formatter)
			}
		}

		writeIterator = strings.SplitSeq(
			fmt.Sprintf(step.Write.ContentFromContainerlabFlags.Content, formatters...),
			"\n",
		)
	default:
		panic("unimplemented write type")
	}

	for s := range writeIterator {
		a.l.Debug("writing to console", "content", s)

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

		err = a.readUntil(ctx, s)
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

func (a *Agent) processStepWait(ctx context.Context, step *boxenprofile.Step) error {
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
