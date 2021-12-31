package util

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type Spinner struct {
	mu         *sync.RWMutex
	Delay      time.Duration
	chars      []string
	Prefix     string
	Suffix     string
	active     bool
	stopChan   chan struct{}
	PostUpdate func(s *Spinner)
	Finale     func(s *Spinner) string
}

func NewSpinner() *Spinner {
	s := &Spinner{
		Delay: 250 * time.Millisecond,
		chars: []string{
			"⠈",
			"⠉",
			"⠋",
			"⠓",
			"⠒",
			"⠐",
			"⠐",
			"⠒",
			"⠖",
			"⠦",
			"⠤",
			"⠠",
			"⠠",
			"⠤",
			"⠦",
			"⠖",
			"⠒",
			"⠐",
			"⠐",
			"⠒",
			"⠓",
			"⠋",
			"⠉",
			"⠈",
		},
		mu:       &sync.RWMutex{},
		stopChan: make(chan struct{}, 1),
	}

	return s
}

func (s *Spinner) resetPrompt() {
	// move cursor to column 1 and then erase to end of the line -- this clears our previously
	// written spinner so, we can output an updated one. handy link:
	// https://gist.github.com/fnky/458719343aabd01cfb17a3a4f7296797
	_, _ = fmt.Fprintf(os.Stdout, "\x1b[1G")
	_, _ = fmt.Fprintf(os.Stdout, "\x1b[0K")
}

func (s *Spinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}

	// hides the cursor
	_, _ = fmt.Fprint(os.Stdout, "\033[?25l")

	s.active = true
	s.mu.Unlock()

	go func() {
		for {
			for i := 0; i < len(s.chars); i++ {
				select {
				case <-s.stopChan:
					return
				default:
					s.mu.Lock()
					if !s.active {
						s.mu.Unlock()
						return
					}

					s.resetPrompt()

					out := fmt.Sprintf("\r%s\n\t%s%s ", s.Prefix, s.chars[i], s.Suffix)
					if s.Prefix == "" {
						out = fmt.Sprintf("\r%s\t%s%s ", s.Prefix, s.chars[i], s.Suffix)
					}

					_, _ = fmt.Fprint(
						os.Stdout,
						out,
					)

					if s.PostUpdate != nil {
						s.PostUpdate(s)
					}

					s.mu.Unlock()
					time.Sleep(s.Delay)
				}
			}
		}
	}()
}

func (s *Spinner) Stop() {
	s.mu.Lock()

	defer s.mu.Unlock()

	if s.active {
		s.active = false

		if s.Finale != nil {
			_, _ = fmt.Fprint(
				os.Stdout,
				s.Finale(s),
			)
		}

		// makes the cursor visible again
		_, _ = fmt.Fprint(os.Stdout, "\033[?25h")
		fmt.Println()

		s.stopChan <- struct{}{}
	}
}
