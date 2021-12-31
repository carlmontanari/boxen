package logging

import (
	"strings"
	"sync"
)

type FifoLogQueue struct {
	q    []string
	lock *sync.Mutex
}

func NewFifoLogQueue() *FifoLogQueue {
	return &FifoLogQueue{
		q:    nil,
		lock: &sync.Mutex{},
	}
}

func (l *FifoLogQueue) Accept(o ...interface{}) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if len(o) == 0 {
		return
	}

	s, ok := o[0].(string)
	if !ok {
		return
	}

	l.q = append(l.q, s)
}

func (l *FifoLogQueue) Emit() string {
	l.lock.Lock()
	defer l.lock.Unlock()

	var e []string

	for len(l.q) > 0 {
		e = append(e, l.q[0])
		l.q = l.q[1:]
	}

	return strings.Join(e, "\n")
}
