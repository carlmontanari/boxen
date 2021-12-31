package logging

import "sync"

// Queue represents a single log message queue.
type Queue struct {
	queue []*Message
	depth int
	lock  *sync.Mutex
}

// newInstanceQueue returns a new Queue with nothing but the lock attribute instantiated.
func newInstanceQueue() *Queue {
	return &Queue{
		lock: &sync.Mutex{},
	}
}
