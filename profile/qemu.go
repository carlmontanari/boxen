package profile

import (
	"context"
	"crypto/rand"
	"encoding/json"
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

	device = "-device"

	monitorPort       = 4_001
	serialPortBaseIdx = 5_001

	accelerationKVM = "kvm"
)

// QemuArgsFromProfile builds the qemu launch args from the given profile/disk.
func QemuArgsFromProfile(
	p *Profile,
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
		kOverrides, ok := p.VirtualMachine.Overrides[k]
		if ok {
			for _, o := range kOverrides {
				out = o.Apply(isPackaging, out)
			}

			continue
		}

		var args []string
		if k == mgmtNIC {
			args = qemuMgmtNIC(p, isPackaging)
		} else {
			args = fs[k](p)
		}

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
		out = e.Apply(isPackaging, out)
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
		strconv.Itoa(int(p.VirtualMachine.Memory)),
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
			device,
			fmt.Sprintf("pci-bridge,chassis_nr=%d,id=pci.%d", busID, busID),
		)
	}

	return pciCmd
}

func qemuMgmtNIC(p *Profile, isPackaging bool) []string {
	managementPassthrough := !isPackaging && p.VirtualMachine.IsManagementPassthroughEnabled()
	mac := ""

	if managementPassthrough {
		mac = os.Getenv(boxenconstants.EnvClabMgmtMAC)
		if mac == "" {
			mac = getIntfMac(context.Background(), boxenutil.ClabMgmtIntfName())
		}

		if mac == "" {
			mac = generateMac(0)
		}
	}

	deviceArgs := fmt.Sprintf("%s,netdev=mgmt", p.VirtualMachine.NicType)
	if mac != "" {
		deviceArgs = fmt.Sprintf("%s,mac=%s", deviceArgs, mac)
	}

	nicCmd := []string{
		device,
		deviceArgs,
		"-netdev",
		"",
	}

	if managementPassthrough {
		nicCmd[3] = "tap,id=mgmt,ifname=tap0,script=no,downscript=no"

		return nicCmd
	}

	mgmtIntf := "user,id=mgmt,net=10.0.0.0/24,host=10.0.0.2," +
		"dns=10.0.0.3,dhcpstart=10.0.0.15,tftp=/tftpboot"

	nats := make([]string, len(p.VirtualMachine.NatPorts))

	for idx := range p.VirtualMachine.NatPorts {
		nats[idx] = fmt.Sprintf(
			"hostfwd=%s:0.0.0.0:%d-10.0.0.15:%d",
			p.VirtualMachine.NatPorts[idx].Type,
			p.VirtualMachine.NatPorts[idx].LocalPort,
			p.VirtualMachine.NatPorts[idx].LocalPort,
		)
	}

	if len(nats) > 0 {
		mgmtIntf = mgmtIntf + "," + strings.Join(nats, ",")
	}

	nicCmd[3] = mgmtIntf

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
	intfName := boxenutil.ClabIntfName(nicID)

	// try to get the mac from the container interface so things match in bridge mode
	mac := getIntfMac(context.Background(), intfName)
	if mac == "" {
		mac = generateMac(nicID)
	}

	// the tap is always created (script=no); the boxen tc service brings it up and
	// stitches it to the container interface when that interface appears
	return []string{
		device,
		fmt.Sprintf(
			"%s,netdev=p%s,bus=pci.%d,addr=0x%x,mac=%s",
			p.VirtualMachine.NicType,
			paddedNicID,
			busID,
			busAddr,
			mac,
		),
		"-netdev",
		fmt.Sprintf(
			"tap,id=p%s,ifname=tap%d,script=no,downscript=no",
			paddedNicID,
			nicID,
		),
	}
}

func generateMac(lastOctet int) string {
	buf := make([]byte, 3) //nolint:mnd

	_, _ = rand.Read(buf)

	if lastOctet > 0 {
		buf[2] = byte(lastOctet) //nolint: gosec
	}

	return fmt.Sprintf("52:54:00:%02x:%02x:%02x", buf[0], buf[1], buf[2])
}

type ipLinkShowOutput []struct {
	Address string `json:"address"`
}

func getIntfMac(ctx context.Context, intf string) string {
	cmd := exec.CommandContext(ctx, "ip", "--json", "link", "show", "dev", intf) //nolint: gosec

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
