package profile

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenutil "github.com/carlmontanari/boxen/util"
	"github.com/google/uuid"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

const (
	cpu          = "cpu"
	memory       = "memory"
	acceleration = "acceleration"
	machine      = "machine"
	disk         = "disk"
	serial       = "serial"
	monitor      = "monitor"
	display      = "display"
	pci          = "pci"
	mgmtNIC      = "mgmtNIC"
	dataNICs     = "dataNICs"

	monitorPort       = 4_001
	serialPortBaseIdx = 5_001

	defaultSocketPad = 10_000

	accelerationKVM = "kvm"
)

// QemuArgsFromProfile builds the qemu launch args from the given profile/disk.
func QemuArgsFromProfile(
	p *Profile,
	formatters *Formatters,
	isPackaging bool,
) ([]string, error) {
	out := []string{
		"-name",
		p.Name,
		"-uuid",
		uuid.NewString(),
	}

	fs := map[string]func(p *Profile) []string{
		cpu:          qemuCPU,
		memory:       qemuMemory,
		acceleration: qemuAccel,
		machine:      qemuMachine,
		disk:         qemuDisk,
		serial:       qemuSerial,
		monitor:      qemuMonitor,
		display:      qemuDisplay,
		pci:          qemuPCI,
		mgmtNIC:      qemuMgmtNIC,
		dataNICs:     qemuDataNICs,
	}

	// access via map but always iterate via slice because *must* be in correct order!
	for _, k := range []string{
		cpu,
		memory,
		acceleration,
		machine,
		disk,
		serial,
		monitor,
		display,
		pci,
		mgmtNIC,
		dataNICs,
	} {
		_, ok := p.VirtualMachine.Overrides[k]
		if ok {
			out = append(out, p.VirtualMachine.Overrides[k]...)

			continue
		}

		args := fs[k](p)

		fBody, mutateOk := p.VirtualMachine.Mutators[k]
		if !mutateOk {
			out = append(out, args...)

			continue
		}

		args, err := invokeStarlarkF(fBody, args)
		if err != nil {
			return nil, err
		}

		out = append(out, args...)
	}

	for _, e := range p.VirtualMachine.Extras {
		if isPackaging && e.OnPackage {
			for _, ev := range e.Val {
				fs, err := formatters.UnpackFormatters(ev.Formatters)
				if err != nil {
					return nil, err
				}

				out = append(out, fmt.Sprintf(ev.Content, fs...))
			}
		} else if e.OnRun {
			for _, ev := range e.Val {
				fs, err := formatters.UnpackFormatters(ev.Formatters)
				if err != nil {
					return nil, err
				}

				out = append(out, fmt.Sprintf(ev.Content, fs...))
			}
		}
	}

	qemuAdditionalArgs := os.Getenv(boxenconstants.EnvClabQemuAdditionalArgs)

	if qemuAdditionalArgs != "" {
		out = append(out, strings.Split(qemuAdditionalArgs, " ")...)
	}

	return out, nil
}

func qemuCPU(p *Profile) []string {
	cpuCmd := []string{}

	cpuOverride := os.Getenv(boxenconstants.EnvClabQemuCPU)
	smpOverride := os.Getenv(boxenconstants.EnvClabQemuSMP)

	if cpuOverride != "" {
		cpuCmd = append(cpuCmd, "-cpu", cpuOverride)
	} else if p.VirtualMachine.CPUEmulation != "" {
		cpuCmd = append(cpuCmd, "-cpu", p.VirtualMachine.CPUEmulation)
	}

	if p.VirtualMachine.CPUCores != 0 {
		if len(cpuCmd) == 0 {
			cpuCmd = append(
				cpuCmd,
				"-cpu",
				"max",
			)
		}

		switch {
		case smpOverride != "":
			cpuCmd = append(
				cpuCmd,
				"-smp",
				smpOverride,
			)
		case p.VirtualMachine.CPUThreads != 0 && p.VirtualMachine.CPUSockets != 0:
			cpuCmd = append(
				cpuCmd,
				"-smp",
				fmt.Sprintf(
					"cores=%d,threads=%d,sockets=%d",
					p.VirtualMachine.CPUCores,
					p.VirtualMachine.CPUThreads,
					p.VirtualMachine.CPUSockets,
				),
			)
		default:
			cpuCmd = append(
				cpuCmd,
				"-smp",
				strconv.Itoa(int(p.VirtualMachine.CPUCores)),
			)
		}
	}

	return cpuCmd
}

func qemuMemory(p *Profile) []string {
	memOverride := os.Getenv(boxenconstants.EnvClabQemuMemory)

	if memOverride != "" {
		return []string{
			"-m",
			memOverride,
		}
	}

	return []string{
		"-m",
		strconv.Itoa(int(p.VirtualMachine.Memory)), //nolint:gosec
	}
}

func qemuAccel(_ *Profile) []string {
	_, err := os.Stat("/dev/kvm")
	if err == nil {
		// if kvm available (and this is kind a janky check, but... probably good enough),
		// we'll always enable it
		return []string{"-accel", accelerationKVM}
	}

	return []string{}
}

func qemuMachine(p *Profile) []string {
	if p.VirtualMachine.Machine != "" {
		return []string{"-machine", p.VirtualMachine.Machine}
	}

	return []string{}
}

func qemuDisk(_ *Profile) []string {
	return []string{"-drive", "if=ide,file=disk.qcow2,format=qcow2"}
}

func qemuSerial(p *Profile) []string {
	var serialCmd []string //nolint: prealloc

	for idx := range p.VirtualMachine.SerialPortCount {
		serialCmd = append(
			serialCmd,
			"-serial",
			fmt.Sprintf(
				"telnet:0.0.0.0:%d,server,nowait",
				serialPortBaseIdx+int(idx)),
		)
	}

	return serialCmd
}

func qemuMonitor(_ *Profile) []string {
	return []string{
		"-monitor",
		fmt.Sprintf("tcp:0.0.0.0:%d,server,nowait", monitorPort),
	}
}

func qemuDisplay(p *Profile) []string {
	if p.VirtualMachine.Display != "" {
		return []string{"-display", p.VirtualMachine.Display}
	}

	return []string{"-display", "none"}
}

func qemuPCI(p *Profile) []string {
	var pciCmd []string

	nicCount := float64(p.VirtualMachine.NicCount)
	nicPerBus := float64(p.VirtualMachine.NicPerBus)

	busRequired := int(math.Ceil(nicCount / nicPerBus))

	for busID := 1; busID < busRequired+1; busID++ {
		pciCmd = append(
			pciCmd,
			"-device",
			fmt.Sprintf("pci-bridge,chassis_nr=%d,id=pci.%d", busID, busID),
		)
	}

	return pciCmd
}

func qemuMgmtNIC(p *Profile) []string {
	if p.VirtualMachine.ManagementPassthrough {
		panic("not implemented")
	}

	nicCmd := []string{
		"-device",
		fmt.Sprintf("%s,netdev=mgmt", p.VirtualMachine.NicType),
		"-netdev",
	}

	mgmtIntf := "user,id=mgmt,net=10.0.0.0/24,tftp=/tftpboot"

	nats := make([]string, len(p.VirtualMachine.NatPorts))

	for idx := range p.VirtualMachine.NatPorts {
		nats[idx] = fmt.Sprintf(
			"hostfwd=%s::%d-10.0.0.15:%d",
			p.VirtualMachine.NatPorts[idx].Type,
			p.VirtualMachine.NatPorts[idx].ExternalPort,
			p.VirtualMachine.NatPorts[idx].LocalPort,
		)
	}

	if len(nats) > 0 {
		mgmtIntf = mgmtIntf + "," + strings.Join(nats, ",")
	}

	nicCmd = append(nicCmd, mgmtIntf)

	return nicCmd
}

func qemuDataNICs(p *Profile) []string {
	var nicCmd []string

	for nicID := 1; nicID < int(p.VirtualMachine.NicCount)+1; nicID++ {
		busID := int(math.Floor(float64(nicID)/float64(p.VirtualMachine.NicPerBus))) + 1
		busAddr := (nicID % int(p.VirtualMachine.NicPerBus)) + 1
		paddedNicID := fmt.Sprintf("%03d", nicID)

		nicCmd = append(
			nicCmd,
			buildDataNic(p, nicID, busID, busAddr, paddedNicID)...,
		)
	}

	return nicCmd
}

func buildDataNic(
	p *Profile,
	nicID,
	busID,
	busAddr int,
	paddedNicID string,
) []string {
	intfPrefix := boxenutil.GetEnvStrOrDefault(boxenconstants.EnvClabIntfPrefix, "eth")

	_, err := os.Stat(fmt.Sprintf("/sys/class/net/%s%d", intfPrefix, nicID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{
				"-device",
				fmt.Sprintf(
					"%s,netdev=p%s,bus=pci.%d,addr=0x%x,mac=%s",
					p.VirtualMachine.NicType,
					paddedNicID,
					busID,
					busAddr,
					generateMac(nicID),
				),
				"-netdev",
				fmt.Sprintf("socket,id=p%s,listen=:%d", paddedNicID, nicID+defaultSocketPad),
			}
		}
	}

	// try to get the mac from the container interface so things match in bridge mode
	mac := getIntfMac(context.Background(), fmt.Sprintf("%s%d", intfPrefix, nicID))
	if mac == "" {
		mac = generateMac(nicID)
	}

	nicCmd := []string{
		"-device",
		fmt.Sprintf(
			"%s,netdev=p%s,bus=pci.%d,addr=0x%x,mac=%s",
			p.VirtualMachine.NicType,
			paddedNicID,
			busID,
			busAddr,
			mac,
		),
	}

	// TODO tc mode we then append -netdev and the tap info

	return nicCmd
}

func generateMac(lastOctet int) string {
	buf := make([]byte, 3) //nolint:mnd

	_, _ = rand.Read(buf)

	if lastOctet > 0 {
		buf[2] = byte(lastOctet)
	}

	return fmt.Sprintf("52:54:00:%02x:%02x:%02x", buf[0], buf[1], buf[2])
}

type ipLinkShowOutput []struct {
	Address string `json:"address"`
}

func getIntfMac(ctx context.Context, intf string) string {
	cmd := exec.CommandContext(ctx, "ip", "--json", "link", "show", "dev", intf)

	err := cmd.Run()
	if err != nil {
		return ""
	}

	b, err := cmd.Output()
	if err != nil {
		return ""
	}

	var o ipLinkShowOutput

	err = json.Unmarshal(b, &o)
	if err != nil {
		return ""
	}

	if len(o) > 0 {
		return o[0].Address
	}

	return ""
}

func invokeStarlarkF(fBody string, cmd []string) ([]string, error) {
	starlarkNicCmd := starlark.List{}
	for _, elem := range cmd {
		err := starlarkNicCmd.Append(starlark.String(elem))
		if err != nil {
			return nil, err
		}
	}

	thread := &starlark.Thread{Name: "mutator"}

	globals, err := starlark.ExecFileOptions(
		syntax.LegacyFileOptions(),
		thread,
		"",
		fBody,
		nil,
	)
	if err != nil {
		return nil, err
	}

	f, ok := globals["mutate"]
	if !ok {
		return nil, err
	}

	result, err := starlark.Call(
		thread,
		f,
		starlark.Tuple{&starlarkNicCmd},
		nil,
	)
	if err != nil {
		return nil, err
	}

	outList, ok := result.(*starlark.List)
	if !ok {
		return nil, fmt.Errorf(
			"%w: starlark program did not return list type",
			boxenerrors.ErrBoxen,
		)
	}

	out := make([]string, outList.Len())

	resultIter := outList.Iterate()
	defer resultIter.Done()

	var v starlark.Value

	var count int

	for resultIter.Next(&v) {
		s, ok := starlark.AsString(v)
		if !ok {
			return nil, fmt.Errorf(
				"%w: failed re-casting starlark value to string",
				boxenerrors.ErrBoxen,
			)
		}

		out[count] = s

		count++
	}

	return out, nil
}
