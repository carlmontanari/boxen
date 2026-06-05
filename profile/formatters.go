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
	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenutil "github.com/carlmontanari/boxen/util"
)

const (
	defaultMgmtIPv4        = "10.0.0.15/24"
	defaultMgmtIPv4Gateway = "10.0.0.2"
	defaultMgmtIPv6        = "2001:db8::2/64"
	defaultMgmtIPv6Gateway = "2001:db8::1"
)

// optionalMgmtFormatters is the single source of truth for which management
// formatter/template keys may legitimately resolve to an empty value. IPv6
// management connectivity is not guaranteed in every environment, so IPv6 keys
// are optional while IPv4 keys are always expected. The positional formatter
// path errors when a required key is empty; the template path keeps every key
// present-but-empty so profiles can guard them with {{ if .mgmtIPv6 }} or a
// shell `[ -n "..." ]` check.
var optionalMgmtFormatters = map[string]bool{
	"mgmtIPv6":          true,
	"mgmtIPv6Address":   true,
	"mgmtIPv6PrefixLen": true,
	"mgmtIPv6Network":   true,
	"mgmtGatewayIPv6":   true,
}

// Formatters holds all the valid "formatter" options for string interpolation in profile content.
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

// UnpackFormatters accepts the users inputs, and returns a list of formatters to use with Sprintf.
func (f *Formatters) UnpackFormatters(inputs []string) ([]any, error) {
	var formatters []any

	for _, formatter := range inputs {
		switch {
		case formatter == "disk":
			if f.disk == "" {
				return nil, fmt.Errorf(
					"%w: disk unset but disk formatter requested", boxenerrors.ErrBoxen,
				)
			}

			formatters = append(formatters, f.disk)
		case formatter == "version":
			if f.version == "" {
				return nil, fmt.Errorf(
					"%w: version unset but version formatter requested", boxenerrors.ErrBoxen,
				)
			}

			formatters = append(formatters, f.version)
		case strings.HasPrefix(formatter, "extraFile"):
			idxStr := strings.TrimSuffix(strings.TrimPrefix(formatter, "extraFile["), "]")

			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return nil, err
			}

			if idx < 0 || idx >= len(f.extraFiles) {
				return nil, fmt.Errorf(
					"%w: extraFile index %d out of range (have %d extra files)",
					boxenerrors.ErrBoxen,
					idx,
					len(f.extraFiles),
				)
			}

			formatters = append(formatters, f.extraFiles[idx])
		case formatter == "username":
			if f.username == "" {
				return nil, fmt.Errorf(
					"%w: username unset but username formatter requested", boxenerrors.ErrBoxen,
				)
			}

			formatters = append(formatters, f.username)
		case formatter == "password":
			if f.password == "" {
				return nil, fmt.Errorf(
					"%w: password unset but password formatter requested", boxenerrors.ErrBoxen,
				)
			}

			formatters = append(formatters, f.password)
		case formatter == "hostname":
			if f.hostname == "" {
				return nil, fmt.Errorf(
					"%w: hostname unset but hostname formatter requested", boxenerrors.ErrBoxen,
				)
			}

			formatters = append(formatters, f.hostname)
		case strings.HasPrefix(formatter, "mgmt"):
			v, err := f.managementFormatter(formatter)
			if err != nil {
				return nil, err
			}

			formatters = append(formatters, v)
		default:
			return nil, fmt.Errorf("%w: invalid formatter %q", boxenerrors.ErrBoxen, formatter)
		}
	}

	return formatters, nil
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

	data["mgmtIPv4"] = management.ipv4
	data["mgmtIPv4Address"] = management.ipv4Address
	data["mgmtIPv4PrefixLen"] = management.ipv4PrefixLen
	data["mgmtIPv4Network"] = management.ipv4Network
	data["mgmtGatewayIPv4"] = management.ipv4Gateway
	data["mgmtIPv6"] = management.ipv6
	data["mgmtIPv6Address"] = management.ipv6Address
	data["mgmtIPv6PrefixLen"] = management.ipv6PrefixLen
	data["mgmtIPv6Network"] = management.ipv6Network
	data["mgmtGatewayIPv6"] = management.ipv6Gateway

	return data, nil
}

func (f *Formatters) managementFormatter(formatter string) (string, error) {
	management, err := f.getManagementFormatters()
	if err != nil {
		return "", err
	}

	var v string

	switch formatter {
	case "mgmtIPv4":
		v = management.ipv4
	case "mgmtIPv4Address":
		v = management.ipv4Address
	case "mgmtIPv4PrefixLen":
		v = management.ipv4PrefixLen
	case "mgmtIPv4Network":
		v = management.ipv4Network
	case "mgmtGatewayIPv4":
		v = management.ipv4Gateway
	case "mgmtIPv6":
		v = management.ipv6
	case "mgmtIPv6Address":
		v = management.ipv6Address
	case "mgmtIPv6PrefixLen":
		v = management.ipv6PrefixLen
	case "mgmtIPv6Network":
		v = management.ipv6Network
	case "mgmtGatewayIPv6":
		v = management.ipv6Gateway
	default:
		return "", fmt.Errorf("%w: invalid formatter %q", boxenerrors.ErrBoxen, formatter)
	}

	if management.dhcp {
		return "", fmt.Errorf(
			"%w: formatter %q unavailable when %s=true; use mgmtDHCP in a template",
			boxenerrors.ErrBoxen,
			formatter,
			boxenconstants.EnvClabMgmtDHCP,
		)
	}

	if v == "" && !optionalMgmtFormatters[formatter] {
		return "", fmt.Errorf("%w: formatter %q unset", boxenerrors.ErrBoxen, formatter)
	}

	return v, nil
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

	_ = management.applyCIDR(defaultMgmtIPv4, "ipv4")
	_ = management.applyCIDR(defaultMgmtIPv6, "ipv6")

	return management
}

func dhcpManagementFormatters() *managementFormatters {
	return &managementFormatters{
		dhcp: true,
	}
}

func runtimeManagementFormatters(ctx context.Context) (*managementFormatters, error) {
	intfPrefix := boxenutil.GetEnvStrOrDefault(boxenconstants.EnvClabIntfPrefix, "eth")
	mgmtIntf := fmt.Sprintf("%s0", intfPrefix)
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
				err = m.applyCIDR(cidr, "ipv4")
			case "inet6":
				err = m.applyCIDR(cidr, "ipv6")
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

// applyCIDR applies the CIDR to the managementFormatters struct for the given family.
// It parses the CIDR, extracts the address, network, and prefix length, and sets the appropriate fields in the managementFormatters struct.
func (m *managementFormatters) applyCIDR(cidr, family string) error {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return err
	}

	address := prefix.Addr().String()
	network := prefix.Masked().String()
	prefixLen := strconv.Itoa(prefix.Bits())

	switch family {
	case "ipv4":
		m.ipv4 = cidr
		m.ipv4Address = address
		m.ipv4PrefixLen = prefixLen
		m.ipv4Network = network
	case "ipv6":
		m.ipv6 = cidr
		m.ipv6Address = address
		m.ipv6PrefixLen = prefixLen
		m.ipv6Network = network
	}

	return nil
}
