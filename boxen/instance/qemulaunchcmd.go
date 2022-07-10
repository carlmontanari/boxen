package instance

import (
	"crypto/rand"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"

	"github.com/carlmontanari/boxen/boxen/config"
	"github.com/carlmontanari/boxen/boxen/util"
)

const defaultSocketPad = 10000

// QemuLaunchCmd represents all the elements of a qemu launch command as slices of strings.
type QemuLaunchCmd struct {
	Name    []string
	UUID    []string
	Accel   []string
	Display []string
	Machine []string
	Memory  []string
	CPU     []string
	Monitor []string
	Serial  []string
	Disk    []string
	Pci     []string
	MgmtNic []string
	DataNic []string
	Extra   []string
}

// Render creates all elements of the launch command and returns it as a slice of string.
func (c *QemuLaunchCmd) Render() ([]string, error) {
	if (c == &QemuLaunchCmd{}) {
		return nil, fmt.Errorf("%w: command has not been generated", util.ErrCommandError)
	}

	var launchCmd []string
	launchCmd = append(launchCmd, c.Name...)
	launchCmd = append(launchCmd, c.UUID...)
	launchCmd = append(launchCmd, c.Accel...)
	launchCmd = append(launchCmd, c.Display...)
	launchCmd = append(launchCmd, c.Machine...)
	launchCmd = append(launchCmd, c.Memory...)
	launchCmd = append(launchCmd, c.CPU...)
	launchCmd = append(launchCmd, c.Monitor...)
	launchCmd = append(launchCmd, c.Serial...)
	launchCmd = append(launchCmd, c.Disk...)
	launchCmd = append(launchCmd, c.Pci...)
	launchCmd = append(launchCmd, c.MgmtNic...)
	launchCmd = append(launchCmd, c.DataNic...)
	launchCmd = append(launchCmd, c.Extra...)

	return launchCmd, nil
}

func (i *Qemu) launchCmdUUID() []string {
	return []string{"-uuid", uuid.NewString()}
}

func (i *Qemu) launchCmdAccel() []string {
	selectedAccel := ""

	for _, prefAccel := range i.Hardware.Acceleration {
		if util.StringSliceContains(prefAccel, i.Qemu.Acceleration) {
			selectedAccel = prefAccel
			break
		}
	}

	if selectedAccel == "" {
		panic("no preferred acceleration type(s) available")
	}

	if selectedAccel == AccelNone {
		return []string{}
	}

	return []string{"-accel", selectedAccel}
}

func (i *Qemu) launchCmdDisplay() []string {
	d := OptNone

	if i.Advanced != nil && i.Advanced.Display != "" {
		d = i.Advanced.Display
	}

	return []string{"-display", d}
}

func (i *Qemu) launchCmdMachine() []string {
	m := OptMachinePc

	if i.Advanced != nil && i.Advanced.Machine != "" {
		m = i.Advanced.Machine
	}

	return []string{"-machine", m}
}

func (i *Qemu) launchCmdMemory() []string {
	return []string{"-m", fmt.Sprintf("%d", i.Hardware.Memory)}
}

func (i *Qemu) launchCmdCPU() []string {
	cpuCmd := []string{}

	if i.Advanced == nil {
		return cpuCmd
	}

	c := i.Advanced.CPU

	if c == nil {
		return cpuCmd
	}

	if c.Emulation != "" {
		cpuCmd = append(cpuCmd, []string{"-cpu", c.Emulation}...)
	}

	if c.Cores != 0 {
		if len(cpuCmd) == 0 {
			cpuCmd = append(cpuCmd, []string{"-cpu", "max"}...)
		}

		if c.Threads != 0 && c.Sockets != 0 {
			cpuCmd = append(
				cpuCmd,
				[]string{
					"-smp",
					fmt.Sprintf("cores=%d,threads=%d,sockets=%d", c.Cores, c.Threads, c.Sockets),
				}...)
		} else if c.Threads == 0 && c.Sockets == 0 {
			cpuCmd = append(
				cpuCmd,
				[]string{"-smp", fmt.Sprint(c.Cores)}...)
		}
	}

	return cpuCmd
}

func (i *Qemu) launchCmdMonitor() []string {
	return []string{
		"-monitor",
		fmt.Sprintf("tcp:0.0.0.0:%d,server,nowait", i.Hardware.MonitorPort),
	}
}

func (i *Qemu) launchCmdSerial() []string {
	var serialCmd []string

	for _, port := range i.Hardware.SerialPorts {
		serialCmd = append(
			serialCmd,
			[]string{"-serial", fmt.Sprintf("telnet:0.0.0.0:%d,server,nowait", port)}...)
	}

	return serialCmd
}

func (i *Qemu) launchCmdDisk() []string {
	return []string{"-drive", fmt.Sprintf("if=ide,file=%s,format=qcow2", i.Disk)}
}

func (i *Qemu) launchCmdPci() []string {
	var pciCmd []string

	nicCount := float64(i.Hardware.NicCount)
	nicPerBus := float64(i.Hardware.NicPerBus)

	busRequired := int(math.Ceil(nicCount / nicPerBus))

	for busID := 1; busID < busRequired+1; busID++ {
		pciCmd = append(
			pciCmd,
			[]string{"-device", fmt.Sprintf("pci-bridge,chassis_nr=%d,id=pci.%d", busID, busID)}...)
	}

	return pciCmd
}

func (i *Qemu) launchCmdMgmtNic() []string {
	nicCmd := []string{"-device", fmt.Sprintf("%s,netdev=mgmt", i.Hardware.NicType), "-netdev"}

	if i.MgmtIntf.Nat != nil {
		mgmtIntf := "user,id=mgmt,net=10.0.0.0/24,tftp=/tftpboot"

		if len(i.MgmtIntf.Nat.TCP) > 0 {
			var tcpNats []string

			for _, natPair := range i.MgmtIntf.Nat.TCP {
				tcpNats = append(
					tcpNats,
					fmt.Sprintf(
						"hostfwd=tcp::%d-10.0.0.15:%d",
						natPair.HostSide,
						natPair.InstanceSide,
					),
				)
			}

			joinedTCPNats := strings.Join(tcpNats, ",")
			mgmtIntf = mgmtIntf + "," + joinedTCPNats
		}

		if len(i.MgmtIntf.Nat.UDP) > 0 {
			var udpNats []string

			for _, natPair := range i.MgmtIntf.Nat.UDP {
				udpNats = append(
					udpNats,
					fmt.Sprintf(
						"hostfwd=udp::%d-10.0.0.15:%d",
						natPair.HostSide,
						natPair.InstanceSide,
					),
				)
			}

			joinedUDPNats := strings.Join(udpNats, ",")
			mgmtIntf = mgmtIntf + "," + joinedUDPNats
		}

		nicCmd = append(nicCmd, mgmtIntf)
	} else if i.MgmtIntf.Bridge != nil {
		nicCmd = append(nicCmd, fmt.Sprintf("tap,ifname=tap%d,script=no,downscript=no,id=mgmt", i.ID))
	} else {
		panic("one of nat or bridge must be set on instance...")
	}

	return nicCmd
}

// GenerateMac generates a mac address with a last octet of lastOctet.
func (i *Qemu) GenerateMac(lastOctet int) string {
	buf := make([]byte, 3) //nolint:gomnd

	_, _ = rand.Read(buf)

	if lastOctet > 0 {
		buf[2] = byte(lastOctet)
	}

	return fmt.Sprintf("52:54:00:%02x:%02x:%02x", buf[0], buf[1], buf[2])
}

func (i *Qemu) buildDataNicSocketConn(
	paddedNicID string,
	nicMap *config.SocketConnectPair,
) []string {
	nicCmd := make([]string, 0)

	if nicMap.Connect != -1 {
		nicCmd = append(
			nicCmd,
			fmt.Sprintf(
				"socket,id=p%s,udp=127.0.0.1:%d,listen=:%d",
				paddedNicID,
				nicMap.Connect,
				nicMap.Listen,
			),
		)
	} else {
		nicCmd = append(
			nicCmd,
			fmt.Sprintf(
				"socket,id=p%s,listen=:%d",
				paddedNicID,
				nicMap.Listen,
			),
		)
	}

	return nicCmd
}

// BuildDataNic builds a string representation of a "dataplane" nic for a qemu virtual machine. This
// method is exported such that platforms may use it if they need to modify some behavior of the
// qemu command creation.
func (i *Qemu) BuildDataNic(nicID, busID, busAddr int, paddedNicID string) []string {
	nicCmd := []string{
		"-device",
		fmt.Sprintf(
			"%s,netdev=p%s,bus=pci.%d,addr=0x%x,mac=%s",
			i.Hardware.NicType,
			paddedNicID,
			busID,
			busAddr,
			i.GenerateMac(nicID),
		),
		"-netdev",
	}

	var nicMap *config.SocketConnectPair

	ok := false

	if i.DataPlaneIntf != nil {
		nicMap, ok = i.DataPlaneIntf.SocketConnectMap[nicID]
	}

	if util.DirectoryExists(fmt.Sprintf("/sys/class/net/eth%d", nicID)) {
		nicCmd = append(
			nicCmd,
			fmt.Sprintf(
				"tap,id=p%s,ifname=tap%d,script=/etc/tc-tap-ifup,downscript=no",
				paddedNicID,
				nicID,
			),
		)
	} else if ok && nicMap != nil {
		nicCmd = append(nicCmd, i.buildDataNicSocketConn(paddedNicID, nicMap)...)
	} else {
		nicCmd = append(
			nicCmd, fmt.Sprintf("socket,id=p%s,listen=:%d", paddedNicID, nicID+defaultSocketPad))
	}

	return nicCmd
}

func (i *Qemu) launchCmdDataNic() []string {
	var nicCmd []string

	for nicID := 1; nicID < i.Hardware.NicCount+1; nicID++ {
		busID := int(math.Floor(float64(nicID)/float64(i.Hardware.NicPerBus))) + 1
		busAddr := (nicID % i.Hardware.NicPerBus) + 1
		paddedNicID := fmt.Sprintf("%03d", nicID)

		nicCmd = append(
			nicCmd,
			i.BuildDataNic(nicID, busID, busAddr, paddedNicID)...)
	}

	return nicCmd
}
