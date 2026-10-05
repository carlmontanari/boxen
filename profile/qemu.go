package profile

import (
	"cmp"
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

	accelerationKVM = "kvm"
)

// QemuArgsFromProfile builds the qemu launch args from the given profile/disk.
func QemuArgsFromProfile(
	p *Profile,
	isPackaging bool,
) ([]string, error) {
	if err := p.VirtualMachine.ApplyConfiguration(isPackaging); err != nil {
		return nil, err
	}

	instanceUUID := p.InstanceUUID
	if isPackaging || instanceUUID == "" {
		instanceUUID = uuid.NewString()
	}

	out := []string{
		"-name",
		p.Name,
		"-uuid",
		instanceUUID,
	}

	fs := map[string]func(p *Profile) []string{
		cpu:          qemuCPU,
		memory:       qemuMemory,
		acceleration: qemuAccel,
		machine:      qemuMachine,
		disk:         qemuDisk,
		serial: func(p *Profile) []string {
			return qemuSerial(p, isPackaging)
		},
		monitor:  qemuMonitor,
		display:  qemuDisplay,
		pci:      qemuPCI,
		dataNICs: qemuDataNICs,
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

		args, err := invokeStarlarkF(fBody, args, isPackaging)
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
		out = append(out, strings.Fields(qemuAdditionalArgs)...)
	}

	// rewrite last, so drive references added via QEMU_ADDITIONAL_ARGS point at the overlay too
	if !isPackaging {
		out = useRunDisk(out)
	}

	return out, nil
}

// useRunDisk points the VM at the per-container overlay instead of the packaged disk. It rewrites
// every reference to the packaged disk, including path-qualified ones and ones from profile
// overrides, mutators, and extras, so the packaged disk is only ever opened read-only as the
// overlay's backing file.
func useRunDisk(args []string) []string {
	const fileOpt = "file="

	for idx, arg := range args {
		if arg == boxenconstants.DiskFilename {
			args[idx] = boxenconstants.RunDiskFilename

			continue
		}

		opts := strings.Split(arg, ",")

		for optIdx, opt := range opts {
			value, ok := strings.CutPrefix(opt, fileOpt)
			if !ok {
				continue
			}

			// match the packaged disk by exact name or as a path suffix, so both relative and
			// absolute references are covered; disk.qcow2.bak and friends stay untouched
			if value != boxenconstants.DiskFilename &&
				!strings.HasSuffix(value, "/"+boxenconstants.DiskFilename) {
				continue
			}

			opts[optIdx] = fileOpt + boxenconstants.RunDiskFilename
			args[idx] = strings.Join(opts, ",")
		}
	}

	return args
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
		strconv.FormatUint(uint64(p.VirtualMachine.Memory), 10),
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

func qemuDisk(p *Profile) []string {
	return []string{
		"-drive",
		"if=" + cmp.Or(p.VirtualMachine.DiskInterface, "ide") +
			",file=" + boxenconstants.DiskFilename + ",format=qcow2",
	}
}

func qemuSerial(p *Profile, isPackaging bool) []string {
	var serialCmd []string //nolint: prealloc
	bootLog := boxenconstants.RunBootLogFilename
	if isPackaging {
		bootLog = boxenconstants.PackageBootLogFilename
	}

	for idx := range p.VirtualMachine.SerialPortCount {
		logFilename := bootLog
		if idx > 0 {
			logFilename = fmt.Sprintf("%s.%d", bootLog, idx+1)
		}

		serialCmd = append(
			serialCmd,
			"-chardev",
			fmt.Sprintf(
				"socket,id=serial%d,host=0.0.0.0,port=%d,server=on,wait=off,telnet=on,"+
					"logfile=%s,logappend=off",
				idx,
				boxenconstants.ConsolePort+int(idx),
				logFilename,
			),
			"-serial",
			fmt.Sprintf("chardev:serial%d", idx),
		)
	}

	return serialCmd
}

func qemuMonitor(_ *Profile) []string {
	return []string{
		"-monitor",
		fmt.Sprintf("tcp:0.0.0.0:%d,server,nowait", boxenconstants.MonitorPort),
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

	// NIC indices start at one; an exact bus multiple is on the next bridge.
	busRequired := int(math.Floor(nicCount/nicPerBus)) + 1

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

	deviceArgs := fmt.Sprintf("%s,netdev=mgmt", p.VirtualMachine.GetNicType())
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

	natPorts := p.VirtualMachine.GetNatPorts()
	nats := make([]string, len(natPorts))

	for idx := range natPorts {
		nats[idx] = fmt.Sprintf(
			"hostfwd=%s:0.0.0.0:%d-10.0.0.15:%d",
			natPorts[idx].Type,
			natPorts[idx].LocalPort,
			natPorts[idx].LocalPort,
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

// ResolveDataNICMACs sets the MAC of each data nic: the MAC of the matching container interface
// when it exists, so the guest uses the MAC containerlab assigned, otherwise a generated one.
func (p *Profile) ResolveDataNICMACs() {
	p.DataNICMACs = make([]string, p.VirtualMachine.NicCount)

	for idx := range p.DataNICMACs {
		nicID := idx + 1

		mac := getIntfMac(context.Background(), boxenutil.ClabIntfName(nicID))
		if mac == "" {
			mac = generateMac(nicID)
		}

		p.DataNICMACs[idx] = mac
	}
}

func buildDataNic(
	p *Profile,
	nicID,
	busID,
	busAddr int,
	paddedNicID string,
) []string {
	if len(p.DataNICMACs) != int(p.VirtualMachine.NicCount) {
		p.ResolveDataNICMACs()
	}

	mac := p.DataNICMACs[nicID-1]

	// the tap is always created (script=no); the boxen tc service brings it up and
	// stitches it to the container interface when that interface appears
	return []string{
		device,
		fmt.Sprintf(
			"%s,netdev=p%s,bus=pci.%d,addr=0x%x,mac=%s",
			p.VirtualMachine.GetNicType(),
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

func invokeStarlarkF(fBody string, cmd []string, isPackaging bool) ([]string, error) {
	if cmd == nil {
		cmd = []string{}
	}
	result, err := callStarlark(fBody, "qemu.mutator", "mutate", isPackaging, cmd)
	if err != nil {
		return nil, err
	}
	items, ok := result.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: mutate(items) must return a list", boxenerrors.ErrBoxen)
	}
	out := make([]string, len(items))
	for i, value := range items {
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%w: mutate(items) must return strings", boxenerrors.ErrBoxen)
		}
		out[i] = s
	}

	return out, nil
}
