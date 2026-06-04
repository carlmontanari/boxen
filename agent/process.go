package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenprofile "github.com/carlmontanari/boxen/profile"
	boxenutilringbuffer "github.com/carlmontanari/boxen/util/ringbuffer"
	scrapligocli "github.com/scrapli/scrapligo/v2/cli"
)

func promptCallbackName(idx int, p boxenprofile.Prompt) string {
	name := strings.TrimSpace(p.Name)
	if name != "" {
		return fmt.Sprintf("prompts step name: %s, idx: %d", name, idx)
	}

	return fmt.Sprintf("prompts step idx %d", idx)
}

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
		cbName := promptCallbackName(idx, p)

		a.l.Debug(
			"building prompts callback",
			"callback name",
			cbName,
			"prompt",
			p.Prompt,
			"response",
			p.Response,
			"completes",
			p.Completes,
		)

		var opts []scrapligocli.Option

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
			cbName,
			func(ctx context.Context, c *scrapligocli.Cli, searchBuf, _ string) error {
				a.l.Info("callback triggered", "callback name", cbName)

				a.l.Debug(
					"writing response",
					"contains",
					p.Prompt.Contains,
					"containsPattern",
					p.Prompt.ContainsPattern,
					"notContains",
					p.Prompt.NotContains,
					"response",
					p.Response,
					"hidden",
					p.Hidden,
					"reading until response",
					p.Response,
					"searchBuf",
					searchBuf,
				)

				defer a.l.Info("callback completed", "callback name", cbName)

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

	_, err = a.conn.ReadWithCallbacks(taskCtx, step.Prompts.InitialInput, cbs...)
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

			content := b.GetOrderedContent()

			a.l.Debug(
				"reading until",
				"until",
				step.ReadUntil.Until,
				"content",
				string(content),
			)

			check, err := step.ReadUntil.Until.Check(content)
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

func (a *Agent) processStepWrite(
	ctx context.Context,
	step *boxenprofile.Step,
) error {
	var c string

	formatters, err := a.f.UnpackFormatters(step.Write.Formatters)
	if err != nil {
		return err
	}

	switch {
	case step.Write.Content != "":
		c = step.Write.Content
	case step.Write.ContentFromFile != "":
		b, err := os.ReadFile(step.Write.ContentFromFile)
		if err != nil {
			return err
		}

		c = string(b)
	case step.Write.ContentFromStartupConfig:
		b, err := os.ReadFile(boxenconstants.StartupConfigFilePath)
		if err != nil {
			return err
		}

		c = string(b)
	default:
		panic("unimplemented write type")
	}

	a.l.Info("writing to console", "content", c, "formatters", formatters)

	if len(formatters) > 0 {
		c = fmt.Sprintf(c, formatters...)
	}

	c, err = a.f.RenderTemplate(c)
	if err != nil {
		return err
	}

	writeIterator := strings.SplitSeq(
		c,
		"\n",
	)

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
