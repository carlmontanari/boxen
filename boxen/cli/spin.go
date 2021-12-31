package cli

import (
	"fmt"
	"math"
	"time"

	"github.com/carlmontanari/boxen/boxen/logging"
	"github.com/carlmontanari/boxen/boxen/util"
)

func spinLogger() (*logging.FifoLogQueue, *logging.Instance, error) {
	logLevel := util.GetEnvStrOrDefault("BOXEN_LOG_LEVEL", "info")

	l := logging.NewFifoLogQueue()
	li, err := logging.NewInstance(l.Accept, logging.WithLevel(logLevel))

	if err != nil {
		return nil, nil, err
	}

	return l, li, nil
}

func spin(l *logging.FifoLogQueue, li *logging.Instance, f func() error) error {
	startTime := time.Now()

	c := make(chan error)

	s := util.NewSpinner()
	s.PostUpdate = func(s *util.Spinner) {
		elapsed := int(math.Round(time.Since(startTime).Seconds()))
		s.Prefix = l.Emit()
		s.Suffix = fmt.Sprintf(" %d seconds elapsed", elapsed)
	}

	s.Start()

	go func() {
		c <- f()
	}()

	err := <-c

	s.Finale = func(s *util.Spinner) string {
		symbol := "✅"
		state := "successfully"

		if err != nil {
			symbol = "🆘"
			state = "unsuccessfully"
		}

		li.Drain()

		elapsed := int(math.Round(time.Since(startTime).Seconds()))
		s.Prefix = l.Emit()
		s.Suffix = fmt.Sprintf("finished %s in %d seconds", state, elapsed)

		out := fmt.Sprintf("\r%s\n\t%s %s ", s.Prefix, symbol, s.Suffix)
		if s.Prefix == "" {
			out = fmt.Sprintf("\r%s\t%s %s ", s.Prefix, symbol, s.Suffix)
		}

		return out
	}

	s.Stop()

	return err
}
