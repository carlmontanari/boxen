package cli

import (
	"strings"
	"sync"

	"github.com/carlmontanari/boxen/boxen/command"
)

// checkSudo executes a command (with sudo) to check if the calling user is a super-user and if they
// have passwordless sudo permissions -- if they do not, the command package will prompt the user
// for their sudo password. We do this in the cli module prior to the "spin" starting so that we
// don't need to interrupt the spin to prompt for passwords. This function should be called prior to
// any operations that may require elevated permissions, tasks requiring elevated permissions are:
//     - packagebuild (required to run privileged containers)
//     - install (because qemu may require elevated permissions for kvm/taps/nat interfaces/etc.)
//     - start (same as install)
//     - stop (because we launch qemu instances w/ sudo)
func checkSudo() error {
	_, err := command.Execute(
		"pwd",
		command.WithSudo(true),
		command.WithWait(true),
	)

	return err
}

func instanceOp(f func(string) error, instances string) error {
	wg := &sync.WaitGroup{}

	instanceSlice := strings.Split(instances, ",")

	var errs []error

	for _, instance := range instanceSlice {
		wg.Add(1)

		i := instance

		go func() {
			err := f(i)

			if err != nil {
				errs = append(errs, err)
			}

			wg.Done()
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}
