package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenprofile "github.com/carlmontanari/boxen/profile"
)

const captureReadInterval = 250 * time.Millisecond

// processStepCapture sends the capture command and records its output, which is appended to the
// agent's captured content.
func (a *Agent) processStepCapture(ctx context.Context, step *boxenprofile.Step) error {
	t, err := time.ParseDuration(step.Capture.Timeout)
	if err != nil {
		return err
	}

	command, err := a.f.RenderTemplate(step.Capture.Command)
	if err != nil {
		return err
	}

	if strings.Contains(command, "\n") {
		return fmt.Errorf("%w: capture command must be a single line", boxenerrors.ErrBoxen)
	}

	a.l.Info("capturing command output", "command", command, "timeout", step.Capture.Timeout)

	taskCtx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	if step.Capture.Hidden {
		err = a.conn.WriteAndReturn(command)
	} else {
		err = a.conn.Write(command)
		if err == nil {
			err = a.readUntil(taskCtx, command)
		}

		if err == nil {
			err = a.conn.WriteReturn()
		}
	}

	if err != nil {
		return err
	}

	output, err := a.readCapture(taskCtx, &step.Capture)
	if err != nil {
		return err
	}

	a.l.Info("captured command output", "bytes", len(output))

	a.captured.WriteString(output)

	return nil
}

func (a *Agent) readCapture(ctx context.Context, c *boxenprofile.StepCapture) (string, error) {
	var buf bytes.Buffer

	for {
		b, err := a.readConsole()
		if err != nil {
			return "", err
		}

		buf.Write(b)

		output, rest, ok, err := extractCapture(buf.Bytes(), c)
		if err != nil {
			return "", err
		}

		if ok {
			// the end marker and output after it belong to the following steps
			a.unreadConsole(rest)

			return output, nil
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf(
				"%w: capture end marker not seen: %w",
				boxenerrors.ErrBoxen,
				ctx.Err(),
			)
		case <-time.After(captureReadInterval):
		}
	}
}

// extractCapture returns the output between the capture start and end markers once both have
// been read. Console line endings are normalized to "\n".
func extractCapture(
	raw []byte,
	c *boxenprofile.StepCapture,
) (output string, rest []byte, done bool, err error) {
	content := bytes.ReplaceAll(raw, []byte{0}, nil)
	content = bytes.ReplaceAll(content, []byte("\r"), nil)

	begin := 0

	if c.Start.Contains != "" || c.Start.ContainsPattern != "" {
		loc, err := c.Start.Find(content)
		if err != nil || loc == nil {
			return "", nil, false, err
		}

		// recording begins on the line after the start marker
		lineEnd := bytes.IndexByte(content[loc[1]:], '\n')
		if lineEnd < 0 {
			return "", nil, false, nil
		}

		begin = loc[1] + lineEnd + 1
	}

	loc, err := c.End.Find(content[begin:])
	if err != nil || loc == nil {
		return "", nil, false, err
	}

	rest = content[begin+loc[0]:]
	output = strings.TrimLeft(string(content[begin:begin+loc[0]]), "\n")

	if c.Decode == boxenprofile.CaptureDecodeBase64 {
		decoded, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(output), ""))
		if err != nil {
			return "", nil, false, fmt.Errorf(
				"%w: decoding captured base64 output: %w",
				boxenerrors.ErrBoxen,
				err,
			)
		}

		return string(decoded), rest, true, nil
	}

	if output != "" && !strings.HasSuffix(output, "\n") {
		output += "\n"
	}

	return output, rest, true, nil
}
