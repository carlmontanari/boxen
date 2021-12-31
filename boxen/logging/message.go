package logging

import (
	"fmt"
	"regexp"
	"sync"
)

// Message is a simple struct that contains some fields relevant for log messages within boxen.
type Message struct {
	Message   string
	Level     string
	Timestamp string
}

// MessageFormatter is an interface that defines the requirements for a (future) optional message
// formatter that can be set on logging Instance objects.
type MessageFormatter interface {
	Encode(lm *Message) string
	Decode(s string) *Message
}

// DefaultFormatter is a simple log MessageFormatter implementation.
type DefaultFormatter struct {
	decodeRe    *regexp.Regexp
	compileOnce sync.Once
}

func (df *DefaultFormatter) Encode(lm *Message) string {
	return fmt.Sprintf("%10s %12s %s", lm.Level, lm.Timestamp, lm.Message)
}

func (df *DefaultFormatter) Decode(s string) *Message {
	df.compileOnce.Do(func() {
		df.decodeRe = regexp.MustCompile(`(?mis)^\s+(\w{4,8})\s+(\d{10})\s(.*)`)
	})

	parts := df.decodeRe.FindStringSubmatch(s)

	if len(parts) != 4 { //nolint:gomnd
		panic("cannot decode message")
	}

	return &Message{
		Message:   parts[3],
		Level:     parts[1],
		Timestamp: parts[2],
	}
}

// NoopFormatter is a MessageFormatter implementation that does nothing but pass the message.
type NoopFormatter struct{}

func (df *NoopFormatter) Encode(lm *Message) string {
	return lm.Message
}

func (df *NoopFormatter) Decode(s string) *Message {
	return &Message{
		Message: s,
	}
}
