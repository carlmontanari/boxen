package logging

import "fmt"

// InstanceIoWriter is a wrapper around an Instance such that that instance can be used anywhere an
// io.Writer instance is required.
type InstanceIoWriter struct {
	instance *Instance
	level    string
}

type IoWriterOption func(writer *InstanceIoWriter)

func WithIoWriterLevel(l string) IoWriterOption {
	return func(liw *InstanceIoWriter) {
		liw.level = l
	}
}

func NewInstanceIoWriter(li *Instance, opts ...IoWriterOption) *InstanceIoWriter {
	liw := &InstanceIoWriter{
		instance: li,
		level:    info,
	}

	for _, o := range opts {
		o(liw)
	}

	return liw
}

// Write accepts a log message that will be emitted to the embedded Instance log instance at the
// provided log level.
func (liw *InstanceIoWriter) Write(b []byte) (n int, err error) {
	var f func(b []byte)

	switch liw.level {
	case debug:
		f = liw.instance.Debugb
	case info:
		f = liw.instance.Infob
	case critical:
		f = liw.instance.Criticalb
	default:
		return -1, fmt.Errorf("%w: writer setup with invalid log level", ErrLogError)
	}

	f(b)

	return len(b), nil
}
