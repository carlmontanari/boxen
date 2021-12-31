package cli

import (
	"strings"
	"sync"
)

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
