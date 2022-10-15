package platforms

import (
	"fmt"
	"os"

	soptions "github.com/scrapli/scrapligo/driver/options"

	"github.com/carlmontanari/boxen/boxen/config"
	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/util"
)

func GetPlatformType(v, p string) string {
	switch v {
	case VendorArista:
		if p == PlatformAristaVeos {
			return PlatformTypeAristaVeos
		}
	case VendorCisco:
		switch p {
		case PlatformCiscoCsr1000v:
			return PlatformTypeCiscoCsr1000v
		case PlatformCiscoXrv9k:
			return PlatformTypeCiscoXrv9k
		case PlatformCiscoN9kv:
			return PlatformTypeCiscoN9kv
		}
	case VendorJuniper:
		if p == PlatformJuniperVsrx {
			return PlatformTypeJuniperVsrx
		}
	case VendorPaloAlto:
		if p == PlatformPaloAltoPanos {
			return PlatformTypePaloAltoPanos
		}
	case VendorIPInfusion:
		if p == PlatformIPInfusionOcNOS {
			return PlatformTypeIPInfusionOcNOS
		}
	case VendorCheckpoint:
		if p == PlatformCheckpointCloudguard {
			return PlatformTypeCheckpointCloudguard
		}
	}

	return ""
}

func GetPlatformEmptyStruct(pT string) (Platform, error) {
	switch pT {
	case PlatformTypeAristaVeos:
		return &AristaVeos{}, nil
	case PlatformTypeCiscoCsr1000v:
		return &CiscoCsr1000v{}, nil
	case PlatformTypeCiscoXrv9k:
		return &CiscoXrv9k{}, nil
	case PlatformTypeCiscoN9kv:
		return &CiscoN9kv{}, nil
	case PlatformTypeJuniperVsrx:
		return &JuniperVsrx{}, nil
	case PlatformTypePaloAltoPanos:
		return &PaloAltoPanos{}, nil
	case PlatformTypeIPInfusionOcNOS:
		return &IPInfusionOcNOS{}, nil
	case PlatformTypeCheckpointCloudguard:
		return &CheckpointCloudguard{}, nil
	}

	return nil, fmt.Errorf(
		"%w: unknown platform type, this shouldn't happen",
		util.ErrValidationError,
	)
}

// GetPlatformScrapliDefinition sets the scrapli platform definition to a value
// of the BOXEN_SCRAPLI_PLATFORM_DEFINITION env var or to a default string value.
func GetPlatformScrapliDefinition(p string) string {
	scrapliPlatform := os.Getenv("BOXEN_SCRAPLI_PLATFORM_DEFINITION")
	if scrapliPlatform != "" {
		return scrapliPlatform
	}

	// retrieve default scrapli platform url/name
	// when env var is not set
	switch p {
	case PlatformTypeAristaVeos:
		return AristaVeosScrapliPlatform
	case PlatformTypeCiscoCsr1000v:
		return CiscoCsr1000vScrapliPlatform
	case PlatformTypeCiscoXrv9k:
		return CiscoXrv9kScrapliPlatform
	case PlatformTypeCiscoN9kv:
		return CiscoN9kvScrapliPlatform
	case PlatformTypeJuniperVsrx:
		return JuniperVsrxScrapliPlatform
	case PlatformTypePaloAltoPanos:
		return PaloAltoPanosScrapliPlatform
	case PlatformTypeIPInfusionOcNOS:
		return IPInfusionOcNOSScrapliPlatform
	case PlatformTypeCheckpointCloudguard:
		return CheckpointCloudguardScrapliPlatform
	}

	return ""
}

func NewPlatformFromConfig( //nolint:funlen
	n string,
	c *config.Config,
	l *instance.Loggers,
) (Platform, error) {
	iCfg := c.Instances[n]
	pT := iCfg.PlatformType

	q, err := instance.NewQemu(n, c, l)
	if err != nil {
		return nil, err
	}

	var p Platform

	var con *ScrapliConsole

	scrapliPlatform := GetPlatformScrapliDefinition(pT)

	switch pT {
	case PlatformTypeAristaVeos:
		con, err = NewScrapliConsole(
			scrapliPlatform,
			q.Hardware.SerialPorts[0],
			q.Credentials.Username,
			q.Credentials.Password,
			l,
		)

		p = &AristaVeos{
			Qemu:           q,
			ScrapliConsole: con,
		}
	case PlatformTypeCiscoCsr1000v:
		con, err = NewScrapliConsole(
			scrapliPlatform,
			q.Hardware.SerialPorts[0],
			q.Credentials.Username,
			q.Credentials.Password,
			l,
		)

		p = &CiscoCsr1000v{
			Qemu:           q,
			ScrapliConsole: con,
		}
	case PlatformTypeCiscoXrv9k:
		con, err = NewScrapliConsole(
			scrapliPlatform,
			q.Hardware.SerialPorts[0],
			q.Credentials.Username,
			q.Credentials.Password,
			l,
			soptions.WithReturnChar("\r"),
		)

		p = &CiscoXrv9k{
			Qemu:           q,
			ScrapliConsole: con,
		}
	case PlatformTypeCiscoN9kv:
		con, err = NewScrapliConsole(
			scrapliPlatform,
			q.Hardware.SerialPorts[0],
			q.Credentials.Username,
			q.Credentials.Password,
			l,
			soptions.WithReturnChar("\r"),
		)

		p = &CiscoN9kv{
			Qemu:           q,
			ScrapliConsole: con,
		}
	case PlatformTypeJuniperVsrx:
		con, err = NewScrapliConsole(
			scrapliPlatform,
			q.Hardware.SerialPorts[0],
			q.Credentials.Username,
			q.Credentials.Password,
			l,
		)

		p = &JuniperVsrx{
			Qemu:           q,
			ScrapliConsole: con,
		}
	case PlatformTypePaloAltoPanos:
		con, err = NewScrapliConsole(
			scrapliPlatform,
			q.Hardware.SerialPorts[0],
			q.Credentials.Username,
			q.Credentials.Password,
			l,
			soptions.WithReturnChar("\r"),
		)

		p = &PaloAltoPanos{
			Qemu:           q,
			ScrapliConsole: con,
		}
	case PlatformTypeIPInfusionOcNOS:
		con, err = NewScrapliConsole(
			scrapliPlatform,
			q.Hardware.SerialPorts[0],
			q.Credentials.Username,
			q.Credentials.Password,
			l,
			soptions.WithReturnChar("\r"),
		)

		p = &IPInfusionOcNOS{
			Qemu:           q,
			ScrapliConsole: con,
		}
	case PlatformTypeCheckpointCloudguard:
		con, err = NewScrapliConsole(
			scrapliPlatform,
			q.Hardware.SerialPorts[0],
			q.Credentials.Username,
			q.Credentials.Password,
			l,
			soptions.WithReturnChar("\r"),
		)

		p = &CheckpointCloudguard{
			Qemu:           q,
			ScrapliConsole: con,
		}
	default:
		return nil, fmt.Errorf("%w: scrapligo driver is not found for %q platform",
			util.ErrAllocationError, pT)
	}

	return p, err
}
