package util

import (
	"bytes"
	"os/exec"
	"runtime"
)

func kvmOk() bool {
	proc := exec.Command("kvm-ok")

	out, _ := proc.Output()

	if bytes.Contains(out, []byte("command not found")) {
		return false
	} else if bytes.Contains(out, []byte("KVM acceleration can NOT be used")) {
		return false
	} else if bytes.Contains(out, []byte("KVM acceleration can be used")) {
		return true
	}

	return false
}

func haxOK() bool {
	proc := exec.Command("kextstat")

	out, err := proc.Output()

	if err != nil {
		return false
	} else if bytes.Contains(out, []byte("com.intel.kext.intelhaxm")) {
		return true
	}

	return false
}

func AvailableAccel() []string {
	availAccel := []string{"none"}

	if kvmOk() {
		availAccel = append(availAccel, "kvm")
	}

	if runtime.GOOS == "darwin" {
		availAccel = append(availAccel, "hvf")

		if haxOK() {
			availAccel = append(availAccel, "hax")
		}
	}

	return availAccel
}

func GetQemuPath() string {
	q, err := exec.LookPath("qemu-system-x86_64")
	if err != nil {
		return ""
	}

	return q
}
