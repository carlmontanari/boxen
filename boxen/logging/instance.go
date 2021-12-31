package logging

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

const (
	dequeInterval   = 250
	timestampPlaces = 10
	debug           = "debug"
	debugLevel      = 10
	info            = "info"
	infoLevel       = 20
	critical        = "critical"
	criticalLevel   = 50
)

var levelMap = map[string]int{ //nolint:gochecknoglobals
	debug:    debugLevel,
	info:     infoLevel,
	critical: criticalLevel,
}

var ErrLogError = errors.New("logError")

// Instance represents an instance logging. In boxen there can and will be many logging
// instances to hold logs for different instances or different processes or stdout vs stderr etc.
type Instance struct {
	// Level is the level to log at, must be one of debug, info, or critical when setting, internal
	// value of level is assigned to the integer value of the log level.
	level int
	// Queue is a simple slice of *Message that is our logging queue.
	Queue *Queue
	// Logger is the logger provided by the user -- ex: log.Print.
	Logger func(...interface{})
	// Formatter is the message formatter that provides encoding and decoding of messages.
	Formatter MessageFormatter
	wg        *sync.WaitGroup
	done      bool
	doneLock  *sync.Mutex
}

// NewInstance returns a new logging Instance with locks and queue established. Eventually this
// will accept options to set the Formatter and probably other things.
func NewInstance(l func(...interface{}), opts ...Option) (*Instance, error) {
	li := &Instance{
		level:     infoLevel,
		Queue:     newInstanceQueue(),
		Logger:    l,
		Formatter: &DefaultFormatter{},
		wg:        &sync.WaitGroup{},
		done:      false,
		doneLock:  &sync.Mutex{},
	}

	Manager.addInstance(li)

	for _, o := range opts {
		err := o(li)
		if err != nil {
			return nil, err
		}
	}

	return li, nil
}

// setDone safely sets the Instance done attribute.
func (li *Instance) setDone(v bool) {
	li.doneLock.Lock()
	defer li.doneLock.Unlock()

	li.done = v
}

// getDone safely gets the Instance done attribute.
func (li *Instance) getDone() bool {
	li.doneLock.Lock()
	defer li.doneLock.Unlock()

	return li.done
}

// buildMessage builds a Message from a string.
func (li *Instance) buildMessage(l, f string) *Message {
	return &Message{
		Message:   f,
		Level:     l,
		Timestamp: strconv.FormatInt(time.Now().Unix(), timestampPlaces),
	}
}

// queueMsg acquires a lock on the queue and places a Message into the queue before releasing the
// lock. It also increments the queue depth.
func (li *Instance) queueMsg(lm *Message) {
	li.Queue.lock.Lock()
	defer li.Queue.lock.Unlock()

	level := levelMap[lm.Level]

	if level >= li.level {
		li.Queue.queue = append(li.Queue.queue, lm)

		li.Queue.depth++
	}
}

// deQueueMsg pops a message off the queue and sends it to the Logger.
func (li *Instance) deQueueMsg() {
	li.Queue.lock.Lock()
	defer li.Queue.lock.Unlock()

	lm := li.Queue.queue[0]
	li.Queue.queue = li.Queue.queue[1:]

	li.Queue.depth--

	li.Logger(li.Formatter.Encode(lm))
}

// getQueueDepth returns the length of the log queue.
func (li *Instance) getQueueDepth() int {
	li.Queue.lock.Lock()
	defer li.Queue.lock.Unlock()

	return li.Queue.depth
}

// Debug accepts a debug level log message with no formatting.
func (li *Instance) Debug(f string) {
	li.queueMsg(li.buildMessage(debug, f))
}

// Debugf accepts a debug level log message normal fmt.Sprintf type formatting.
func (li *Instance) Debugf(f string, a ...interface{}) {
	li.queueMsg(li.buildMessage(debug, fmt.Sprintf(f, a...)))
}

// Info accepts an info level log message with no formatting.
func (li *Instance) Info(f string) {
	li.queueMsg(li.buildMessage(info, f))
}

// Infof accepts an info level log message normal fmt.Sprintf type formatting.
func (li *Instance) Infof(f string, a ...interface{}) {
	li.queueMsg(li.buildMessage(info, fmt.Sprintf(f, a...)))
}

// Critical accepts a critical level log message with no formatting.
func (li *Instance) Critical(f string) {
	li.queueMsg(li.buildMessage(critical, f))
}

// Criticalf accepts a critical level log message normal fmt.Sprintf type formatting.
func (li *Instance) Criticalf(f string, a ...interface{}) {
	li.queueMsg(li.buildMessage(critical, fmt.Sprintf(f, a...)))
}

// logb provides common functionality for Debugb Infob and Criticalb methods.
func (li *Instance) logb(l string, b []byte) {
	li.wg.Add(1)

	go func() {
		for {
			if li.getDone() && len(b) == 0 {
				li.wg.Done()

				return
			}

			for len(b) > 0 {
				rb := b
				b = []byte{}

				li.queueMsg(li.buildMessage(l, string(rb)))
			}

			time.Sleep(dequeInterval * time.Millisecond)
		}
	}()
}

// Debugb accepts a debug level log message as a byte slice.
func (li *Instance) Debugb(b []byte) {
	li.logb(debug, b)
}

// Infob accepts a debug level log message as a byte slice.
func (li *Instance) Infob(b []byte) {
	li.logb(info, b)
}

// Criticalb accepts a debug level log message as a byte slice.
func (li *Instance) Criticalb(b []byte) {
	li.logb(critical, b)
}

// Start starts the queue listener of the Instance.
func (li *Instance) Start() {
	li.wg.Add(1)

	go func() {
		for {
			if li.getDone() && li.getQueueDepth() == 0 {
				li.Drain()
				li.wg.Done()

				return
			}

			for li.getQueueDepth() > 0 {
				li.deQueueMsg()
			}

			time.Sleep(dequeInterval * time.Millisecond)
		}
	}()
}

// Drain drains any remaining messages in the Instance Queue.
func (li *Instance) Drain() {
	for li.getQueueDepth() > 0 {
		li.deQueueMsg()
	}
}
