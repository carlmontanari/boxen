package agent

import (
	"bytes"
	"context"
	"fmt"
	"time"

	scrapligocli "github.com/scrapli/scrapligo/cli"
)

func readUntil(ctx context.Context, c *scrapligocli.Cli, s string) error {
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
		fmt.Println(">>> read ", string(b))
		if err != nil {
			return err
		}

		_, err = buf.Write(b)
		if err != nil {
			return err
		}

		fmt.Println(">>> buf contents ", string(buf.Bytes()))
		if bytes.Contains(buf.Bytes(), []byte(s)) {
			return nil
		}

		time.Sleep(time.Second)
	}
}
