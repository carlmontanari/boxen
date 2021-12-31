package instance

import (
	"fmt"
	"io"
	"os"

	"github.com/carlmontanari/boxen/boxen/logging"
)

type Loggers struct {
	// Base is the logger for "normal" logging -- meaning not stdout/stderr of the process and not
	// console output -- just for "boxen" logs.
	Base *logging.Instance
	// Stdout is the writer for qemu process stdout.
	Stdout io.Writer
	// Stderr is the writer for qemu process stderr.
	Stderr io.Writer
	// Console is the writer for console output (if applicable).
	Console io.Writer
}

func NewInstanceLoggersFOut(l *logging.Instance, d string) (*Loggers, error) {
	stdoutF, err := os.Create(fmt.Sprintf("%s/stdout.log", d))
	if err != nil {
		return nil, err
	}

	stderrF, err := os.Create(fmt.Sprintf("%s/stderr.log", d))
	if err != nil {
		return nil, err
	}

	consoleF, err := os.Create(fmt.Sprintf("%s/console.log", d))
	if err != nil {
		return nil, err
	}

	il := &Loggers{
		Base:    l,
		Stdout:  stdoutF,
		Stderr:  stderrF,
		Console: consoleF,
	}

	return il, nil
}
