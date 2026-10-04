package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenprofile "github.com/carlmontanari/boxen/profile"
	scrapligocli "github.com/scrapli/scrapligo/v2/cli"
)

func promptCallbackName(idx int, promptName string) string {
	name := strings.TrimSpace(promptName)
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

	// the prompts engine reads the session itself and cannot see output read before it, so
	// that output is stale once this step runs
	a.pendingConsole = nil

	a.l.Info(
		"handling prompts",
		"count",
		len(step.Prompts.Prompts),
		"timeout",
		step.Prompts.Timeout,
	)

	cbs := make([]*scrapligocli.ReadCallback, len(step.Prompts.Prompts))

	for idx := range step.Prompts.Prompts {
		cbs[idx], err = a.promptCallback(idx, &step.Prompts.Prompts[idx])
		if err != nil {
			return err
		}
	}

	taskCtx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	_, err = a.conn.ReadWithCallbacks(taskCtx, step.Prompts.InitialInput, cbs...)
	if err != nil {
		if step.Prompts.ContinueOnTimeout && ctx.Err() == nil && taskCtx.Err() != nil {
			a.l.Warn(
				"prompts not completed within the timeout, continuing",
				"timeout", step.Prompts.Timeout,
			)

			return nil
		}

		return err
	}

	return nil
}

// promptCallback builds the read callback that answers a prompt of a prompts step.
func (a *Agent) promptCallback(
	idx int,
	p *boxenprofile.Prompt,
) (*scrapligocli.ReadCallback, error) {
	cbName := promptCallbackName(idx, p.Name)

	response, err := a.f.RenderTemplate(p.Response)
	if err != nil {
		return nil, fmt.Errorf("rendering response of %s: %w", cbName, err)
	}

	var delay time.Duration

	if p.Delay != "" {
		delay, err = time.ParseDuration(p.Delay)
		if err != nil {
			return nil, fmt.Errorf("parsing delay of %s: %w", cbName, err)
		}
	}

	a.l.Debug(
		"building prompts callback",
		"callback name",
		cbName,
		"prompt",
		p.Prompt,
		"response",
		response,
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

	return scrapligocli.NewReadCallback(
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
				response,
				"hidden",
				p.Hidden,
				"reading until response",
				response,
				"searchBuf",
				searchBuf,
			)

			defer a.l.Info("callback completed", "callback name", cbName)

			if delay > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
			}

			err := c.Write(response)
			if err != nil {
				return err
			}

			if p.Hidden {
				return c.WriteReturn()
			}

			err = a.waitForEcho(ctx, response)
			if err != nil {
				return err
			}

			return c.WriteReturn()
		},
		opts...,
	), nil
}

func (a *Agent) processStepReadUntil(ctx context.Context, step *boxenprofile.Step) error {
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

	taskCtx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	ticker := time.NewTicker(readUntilPollInterval)
	defer ticker.Stop()

	var window []byte

	for {
		r, err := a.readConsole()
		if err != nil {
			return err
		}

		if len(r) > 0 {
			a.l.Debug("console output", "content", string(r))
		}

		// check all new output, plus the tail of earlier output for matches spanning reads
		window = append(window, r...)

		loc, err := step.ReadUntil.Until.Find(window)
		if err != nil {
			return err
		}

		if loc != nil {
			// output past the match belongs to the following steps
			a.unreadConsole(window[loc[1]:])

			return nil
		}

		if len(window) > readUntilWindowSize {
			window = append([]byte(nil), window[len(window)-readUntilWindowSize:]...)
		}

		select {
		case <-taskCtx.Done():
			if ctx.Err() != nil {
				return ctx.Err()
			}

			return fmt.Errorf("%w: read until timeout expired", boxenerrors.ErrBoxen)
		case <-ticker.C:
		}
	}
}

func (a *Agent) processStepWrite(
	ctx context.Context,
	step *boxenprofile.Step,
) error {
	var c string

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
		if a.startupConfigFile == "" {
			return fmt.Errorf("%w: no startup config file present", boxenerrors.ErrBoxen)
		}

		b, err := os.ReadFile(a.startupConfigFile)
		if err != nil {
			return err
		}

		c = string(b)
	default:
		return fmt.Errorf("%w: write step has no content", boxenerrors.ErrBoxen)
	}

	a.l.Info("writing to console", "content", c)

	c, err := a.f.RenderTemplate(c)
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

		err = a.waitForEcho(ctx, s)
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
