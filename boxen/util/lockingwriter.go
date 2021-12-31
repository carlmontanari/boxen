package util

import (
	"bytes"
	"sync"
)

// LockingWriterReader is a simple type that satisfies io.Writer, but also allows for reads. It does
// this with a lock as well, so it is safe to use with long-running commands. It also allows for
// reading from stderr (for example) while the process is still running without any race issues.
type LockingWriterReader struct {
	buf  *bytes.Buffer
	lock *sync.RWMutex
}

// NewLockingWriterReader returns a new instance of LockingWriterReader.
func NewLockingWriterReader() *LockingWriterReader {
	return &LockingWriterReader{
		buf:  &bytes.Buffer{},
		lock: &sync.RWMutex{},
	}
}

// Write safely writes (with a lock) to the buffer in LockingWriterReader instance.
func (lw *LockingWriterReader) Write(b []byte) (n int, err error) {
	lw.lock.Lock()
	defer lw.lock.Unlock()

	lw.buf.Write(b)

	return len(b), nil
}

// Read safely reads (with a rlock) from the buffer in LockingWriterReader instance.
func (lw *LockingWriterReader) Read() ([]byte, error) {
	lw.lock.RLock()
	defer lw.lock.RUnlock()

	b := make([]byte, MaxBuffer)

	_, err := lw.buf.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}
