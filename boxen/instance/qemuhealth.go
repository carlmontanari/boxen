package instance

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/carlmontanari/boxen/boxen/util"
)

const (
	HealthOK  = 200
	HealthBad = 500

	watchCount = 12
	watchSleep = 5
)

// WatchMainProc watches until the process is running, and then emits an error on c if the process
// stops running. If it receives anything on the stop channel it exits. This can be used during
// long-running operations such as "Install" to make sure that if the qemu proc dies we exit
// immediately rather than sit around hoping things happen on the console :).
func (i *Qemu) WatchMainProc(c chan error, stop chan bool) {
	started := false
	checkCount := 0

	for {
		select {
		case <-stop:
			return
		default:
			if !started && checkCount == watchCount {
				c <- fmt.Errorf(
					"%w: process not tagged as started, probably exited very quickly, "+
						"something is almost certainly wrong with the qemu launch command",
					util.ErrInspectionError,
				)
			}

			r := i.validatePid()

			if !started && r {
				started = true
			}

			if started && !r {
				c <- fmt.Errorf("%w: process already exited", util.ErrInstanceError)
			}

			checkCount++

			time.Sleep(watchSleep * time.Second)
		}
	}
}

func (i *Qemu) waitMainProc() {
	_ = i.Proc.Wait()

	i.Proc = nil
	i.PID = 0
}

func (i *Qemu) healthEndpoint(w http.ResponseWriter, r *http.Request) {
	_ = r

	if i.Proc != nil && i.PID > 0 {
		w.WriteHeader(HealthOK)

		return
	}

	w.WriteHeader(HealthBad)
}

func (i *Qemu) healthServer() {
	http.HandleFunc("/", i.healthEndpoint)

	_ = http.ListenAndServe(":7777", nil)
}

func (i *Qemu) RunUntilSigInt() {
	go i.healthServer()
	go i.waitMainProc()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		done <- true
	}()
	<-done
}
