package logging

import (
	"errors"
)

const (
	tcp = "tcp"
)

var (
	ErrSocketFailure = errors.New("error creating socket connection")
)

// ConnData is a struct representing common data attributes required for socket sender and receiver.
type ConnData struct {
	Protocol string
	Address  string
	Port     int
}
