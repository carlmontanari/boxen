package command

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

var (
	sudoerInstance *sudoer   //nolint:gochecknoglobals
	sudoerOnce     sync.Once //nolint:gochecknoglobals
)

func newSudoer() *sudoer {
	sudoerOnce.Do(func() {
		sudoerInstance = &sudoer{}
		sudoerInstance.init()
	})

	return sudoerInstance
}

type sudoer struct {
	available    bool
	passwordless bool
	password     string
}

func (s *sudoer) init() {
	r, err := Execute("sudo", WithArgs([]string{"-ln"}), WithWait(true))
	if err != nil {
		b, _ := r.ReadStderr()

		if bytes.Contains(b, []byte("password is required")) {
			s.available = true

			return
		}

		return
	}

	b, _ := r.ReadStdout()

	if bytes.Contains(b, []byte("command not found")) {
		return
	}

	s.available = true

	// user is a passwordless user *or* user is root (indicating user ran boxen w/ sudo)
	if bytes.Contains(b, []byte("(ALL) NOPASSWD: ALL")) ||
		bytes.Contains(b, []byte("(ALL : ALL) NOPASSWD: ALL")) ||
		bytes.Contains(b, []byte("root may run the following commands")) {
		s.passwordless = true
	}
}

func (s *sudoer) getSudoPassword() {
	fmt.Printf(
		"privilege escalation required and user is not passwordless sudoer, please enter sudo password: ",
	)

	userInput, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		panic("something went wrong getting password from user, cannot continue")
	}

	s.password = string(userInput)
}

func (s *sudoer) updateCmd(cmd string, args []string) (string, []string) { //nolint:gocritic
	if !s.available {
		return cmd, args
	}

	if s.passwordless {
		args = append([]string{cmd}, args...)

		return "sudo", args
	}

	if s.password == "" {
		s.getSudoPassword()
	}

	args = []string{
		"-c",
		fmt.Sprintf("echo '%s' | sudo -S %s %s", s.password, cmd, strings.Join(args, " ")),
	}

	return "sh", args
}
