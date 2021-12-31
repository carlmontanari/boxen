package logging

import (
	"fmt"
	"net"
	"strings"
)

// SocketSender is an object that can emit log messages via TCP socket to a SocketReceiver.
type SocketSender struct {
	*ConnData
	c net.Conn
}

// NewSocketSender returns a new instance of SocketSender with the TCP connection opened.
func NewSocketSender(a string, p int) (*SocketSender, error) {
	ss := &SocketSender{
		&ConnData{
			Protocol: tcp,
			Address:  a,
			Port:     p,
		},
		nil,
	}

	err := ss.open()

	return ss, err
}

// open the TCP session to the SocketReceiver.
func (ss *SocketSender) open() error {
	var err error

	ss.c, err = net.Dial(ss.Protocol, fmt.Sprintf("%s:%d", ss.Address, ss.Port))
	if err != nil {
		return ErrSocketFailure
	}

	return nil
}

// Emit sends log messages to the SocketReceiver -- intended to be used with NewInstance as the
// Logger attribute.
func (ss *SocketSender) Emit(o ...interface{}) {
	if len(o) == 0 {
		return
	}

	s, ok := o[0].(string)
	if !ok {
		return
	}

	if !strings.HasSuffix(s, "\n") {
		s = fmt.Sprintf("%s\n", s)
	}

	_, err := ss.c.Write([]byte(s))
	if err != nil {
		panic(fmt.Sprintf("error emitting message to socket %s\n", err))
	}
}

// Close the SocketSender.
func (ss *SocketSender) Close() error {
	return ss.c.Close()
}
