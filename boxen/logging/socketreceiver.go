package logging

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SocketReceiver is an object that receives messages from the SocketSender and emits them to a
// logging Instance.
type SocketReceiver struct {
	*ConnData
	li    *Instance
	l     net.Listener
	conns *sync.Map
}

// NewSocketReceiver returns a SocketReceiver that is ready to receive messages.
func NewSocketReceiver(a string, p int, li *Instance) (*SocketReceiver, error) {
	sr := &SocketReceiver{
		&ConnData{
			Protocol: tcp,
			Address:  a,
			Port:     p,
		},
		li,
		nil,
		&sync.Map{},
	}

	err := sr.listen()

	return sr, err
}

// queue messages received over the TCP connection into the logging Instance.
func (sr *SocketReceiver) queue(s string) {
	lm := sr.li.Formatter.Decode(s)
	sr.li.queueMsg(lm)
}

// handle listens for messages and dispatches them appropriately to queue.
func (sr *SocketReceiver) handle(id string, c net.Conn) {
	defer func() {
		c.Close()
		sr.conns.Delete(id)
	}()

	for {
		scanner := bufio.NewScanner(c)

		for scanner.Scan() {
			sr.queue(scanner.Text())
		}

		time.Sleep(dequeInterval * time.Millisecond)
	}
}

// listen starts the SocketReceiver listening for messages from a SocketSender.
func (sr *SocketReceiver) listen() error {
	var err error

	sr.l, err = net.Listen(sr.Protocol, fmt.Sprintf("%s:%d", sr.Address, sr.Port))
	if err != nil {
		return ErrSocketFailure
	}

	go func() {
		for {
			conn, acceptErr := sr.l.Accept()
			if acceptErr != nil {
				// seems like we get a lot of these errors, but they have not had any impact...
				continue
			}

			id := uuid.New().String()
			sr.conns.Store(id, conn)

			go sr.handle(id, conn)
		}
	}()

	return nil
}

// Close the SocketReceiver.
func (sr *SocketReceiver) Close() error {
	return sr.l.Close()
}
