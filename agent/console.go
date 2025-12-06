package agent

import (
	"bytes"
	"context"
	"time"

	scrapligocli "github.com/scrapli/scrapligo/cli"
)

func readUntil(ctx context.Context, l *wrappedSlogger, c *scrapligocli.Cli, s string) error {
	var buf bytes.Buffer

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// this read cant block because its only reading off the internally buffered
		// bits that the session has already read
		b, err := c.Read()
		if err != nil {
			return err
		}

		_, err = buf.Write(b)
		if err != nil {
			return err
		}

		contents := buf.Bytes()

		l.Debug("checking contents", "until", s, "contents", string(contents))

		if bytes.Contains(contents, []byte(s)) {
			return nil
		}

		time.Sleep(time.Second)
	}
}
