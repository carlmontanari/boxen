package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenutil "github.com/carlmontanari/boxen/util"
)

const (
	defaultMgmtIPv4        = "10.0.0.15/24"
	defaultMgmtIPv4Gateway = "10.0.0.2"
	defaultMgmtIPv6        = "2001:db8::2/64"
	defaultMgmtIPv6Gateway = "2001:db8::1"
)

// Formatters holds the values available for Go template interpolation in profile content.
type Formatters struct {
	disk           string
	version        string
	extraFiles     []string
	username       string
	password       string
	hostname       string
	connectionMode string
	p              *Profile
	isPackaging    bool

	management *managementFormatters
}

type managementFormatters struct {
	dhcp          bool
	ipv4          string
	ipv4Address   string
	ipv4PrefixLen string
	ipv4Network   string
	ipv4Gateway   string
	ipv6          string
	ipv6Address   string
	ipv6PrefixLen string
	ipv6Network   string
	ipv6Gateway   string
}

// toMap returns the management values keyed by their template names. It is the single
// source of truth for the values exposed to write content templates via TemplateData.
func (m *managementFormatters) toMap() map[string]string {
	return map[string]string{
		"mgmtIPv4":          m.ipv4,
		"mgmtIPv4Address":   m.ipv4Address,
		"mgmtIPv4PrefixLen": m.ipv4PrefixLen,
		"mgmtIPv4Network":   m.ipv4Network,
		"mgmtIPv4Gateway":   m.ipv4Gateway,
		"mgmtIPv6":          m.ipv6,
		"mgmtIPv6Address":   m.ipv6Address,
		"mgmtIPv6PrefixLen": m.ipv6PrefixLen,
		"mgmtIPv6Network":   m.ipv6Network,
		"mgmtIPv6Gateway":   m.ipv6Gateway,
	}
}

type ipAddressShowOutput []struct {
	AddrInfo []struct {
		Family    string `json:"family"`
		Local     string `json:"local"`
		PrefixLen int    `json:"prefixlen"`
		Scope     string `json:"scope"`
	} `json:"addr_info"`
}

type ipRouteShowDefaultOutput []struct {
	Gateway string `json:"gateway"`
}

var (
	ipAddressShowCommand = func(ctx context.Context, intf string) ([]byte, error) {
		return exec.CommandContext(ctx, "ip", "--json", "address", "show", "dev", intf).Output() //nolint:gosec
	}

	ipRouteShowDefaultCommand = func(ctx context.Context, family, intf string) ([]byte, error) {
		return exec.CommandContext(
			ctx, "ip", "--json", family, "route", "show", "default", "dev", intf,
		).Output() //nolint:gosec
	}
)

// NewFormatters returns a Formatters object based on the given inputs/profile.
func NewFormatters(
	username,
	password,
	hostname,
	connectionMode string,
	p *Profile,
	isPackaging bool,
) *Formatters {
	// disk is always disk.qcow2 in "run" mode, but we maybe have a disk that we resolved
	// during packaging, so override that if thats the case
	disk := "disk.qcow2"

	if p.ResolvedDisk != "" {
		// resolved disk we received from boxen builder (the main cli) will be fully qualified,
		// but that file will just be in . on the agent container; same applies to extra files
		disk = filepath.Base(p.ResolvedDisk)
	}

	extraFiles := make([]string, len(p.ExtraFiles))

	for idx := range p.ExtraFiles {
		extraFiles[idx] = filepath.Base(p.ExtraFiles[idx])
	}

	return &Formatters{
		disk:           disk,
		version:        p.ResolvedVersion,
		extraFiles:     extraFiles,
		username:       username,
		password:       password,
		hostname:       hostname,
		connectionMode: connectionMode,
		p:              p,
		isPackaging:    isPackaging,
	}
}

// RenderTemplate renders write content with named Go template values.
func (f *Formatters) RenderTemplate(content string) (string, error) {
	if !strings.Contains(content, "{{") {
		return content, nil
	}

	t, err := template.New("content").Option("missingkey=error").Parse(content)
	if err != nil {
		return "", err
	}

	data, err := f.TemplateData()
	if err != nil {
		return "", err
	}

	var b bytes.Buffer

	err = t.Execute(&b, data)
	if err != nil {
		return "", err
	}

	return b.String(), nil
}

// TemplateData returns named values available to write content Go templates.
func (f *Formatters) TemplateData() (map[string]any, error) {
	data := map[string]any{
		"disk":           f.disk,
		"version":        f.version,
		"extraFiles":     f.extraFiles,
		"username":       f.username,
		"password":       f.password,
		"hostname":       f.hostname,
		"connectionMode": f.connectionMode,
	}

	management, err := f.getManagementFormatters()
	if err != nil {
		return nil, err
	}

	data["mgmtDHCP"] = management.dhcp
	if management.dhcp {
		return data, nil
	}

	for k, v := range management.toMap() {
		data[k] = v
	}

	return data, nil
}

func (f *Formatters) getManagementFormatters() (*managementFormatters, error) {
	if f.management != nil {
		return f.management, nil
	}

	switch {
	case f.isPackaging:
		f.management = defaultManagementFormatters()
	case f.p == nil || f.p.VirtualMachine == nil:
		f.management = defaultManagementFormatters()
	case !f.p.VirtualMachine.IsManagementPassthroughEnabled():
		f.management = defaultManagementFormatters()
	case boxenutil.EnvBoolTrue(boxenconstants.EnvClabMgmtDHCP):
		f.management = dhcpManagementFormatters()
	default:
		management, err := runtimeManagementFormatters(context.Background())
		if err != nil {
			return nil, err
		}

		f.management = management
	}

	return f.management, nil
}

// defaultManagementFormatters returns the default management formatters.
// It sets the default IPv4 and IPv6 gateways and applies the default CIDRs to the managementFormatters struct.
func defaultManagementFormatters() *managementFormatters {
	management := &managementFormatters{
		ipv4Gateway: defaultMgmtIPv4Gateway,
		ipv6Gateway: defaultMgmtIPv6Gateway,
	}

	_ = management.applyIPv4CIDR(defaultMgmtIPv4)
	_ = management.applyIPv6CIDR(defaultMgmtIPv6)

	return management
}

func dhcpManagementFormatters() *managementFormatters {
	return &managementFormatters{
		dhcp: true,
	}
}

func runtimeManagementFormatters(ctx context.Context) (*managementFormatters, error) {
	mgmtIntf := boxenutil.ClabMgmtIntfName()
	management := &managementFormatters{}

	err := management.setRuntimeAddresses(ctx, mgmtIntf)
	if err != nil {
		return nil, err
	}

	err = management.setRuntimeGateways(ctx, mgmtIntf)
	if err != nil {
		return nil, err
	}

	return management, nil
}

// setRuntimeAddresses sets the runtime addresses for the managementFormatters struct.
// It reads the addresses for the given interface and sets the appropriate fields in the managementFormatters struct.
func (m *managementFormatters) setRuntimeAddresses(ctx context.Context, intf string) error {
	b, err := ipAddressShowCommand(ctx, intf)
	if err != nil {
		return err
	}

	var o ipAddressShowOutput

	err = json.Unmarshal(b, &o)
	if err != nil {
		return err
	}

	for _, link := range o {
		for _, addrInfo := range link.AddrInfo {
			if addrInfo.Scope != "global" {
				continue
			}

			cidr := fmt.Sprintf("%s/%d", addrInfo.Local, addrInfo.PrefixLen)

			switch addrInfo.Family {
			case "inet":
				err = m.applyIPv4CIDR(cidr)
			case "inet6":
				err = m.applyIPv6CIDR(cidr)
			default:
				continue
			}

			if err != nil {
				return err
			}
		}
	}

	return nil
}

// setRuntimeGateways sets the runtime default gateways for the managementFormatters struct.
// It reads the default routes for IPv4 and IPv6 on the management interface and sets the
// appropriate fields in the managementFormatters struct.
func (m *managementFormatters) setRuntimeGateways(ctx context.Context, intf string) error {
	ipv4Gateway, err := runtimeDefaultGateway(ctx, "-4", intf)
	if err != nil {
		return err
	}

	ipv6Gateway, err := runtimeDefaultGateway(ctx, "-6", intf)
	if err != nil {
		return err
	}

	m.ipv4Gateway = ipv4Gateway
	m.ipv6Gateway = ipv6Gateway

	return nil
}

// runtimeDefaultGateway returns the default gateway for the given family on the given interface.
// Scoping the lookup to the management interface avoids picking up a default route that egresses a
// different interface when the container has more than one. It returns the gateway IP address, or
// an empty string when no default route exists on that interface for the family.
func runtimeDefaultGateway(ctx context.Context, family, intf string) (string, error) {
	b, err := ipRouteShowDefaultCommand(ctx, family, intf)
	if err != nil {
		return "", err
	}

	var o ipRouteShowDefaultOutput

	err = json.Unmarshal(b, &o)
	if err != nil {
		return "", err
	}

	if len(o) == 0 {
		return "", nil
	}

	return o[0].Gateway, nil
}

// parseCIDR parses a CIDR string into its address, network, and prefix-length parts.
func parseCIDR(cidr string) (address, network, prefixLen string, err error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return "", "", "", err
	}

	return prefix.Addr().String(), prefix.Masked().String(), strconv.Itoa(prefix.Bits()), nil
}

// applyIPv4CIDR sets the IPv4 address, network, and prefix-length fields from the given CIDR.
func (m *managementFormatters) applyIPv4CIDR(cidr string) error {
	address, network, prefixLen, err := parseCIDR(cidr)
	if err != nil {
		return err
	}

	m.ipv4 = cidr
	m.ipv4Address = address
	m.ipv4PrefixLen = prefixLen
	m.ipv4Network = network

	return nil
}

// applyIPv6CIDR sets the IPv6 address, network, and prefix-length fields from the given CIDR.
func (m *managementFormatters) applyIPv6CIDR(cidr string) error {
	address, network, prefixLen, err := parseCIDR(cidr)
	if err != nil {
		return err
	}

	m.ipv6 = cidr
	m.ipv6Address = address
	m.ipv6PrefixLen = prefixLen
	m.ipv6Network = network

	return nil
}
