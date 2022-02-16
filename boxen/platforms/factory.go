package platforms

import (
	"fmt"

	"github.com/carlmontanari/boxen/boxen/config"
	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/util"

	"github.com/scrapli/scrapligo/driver/base"
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
	}

	panic("unknown platform type, this shouldn't happen!")
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

	switch pT {
	case PlatformTypeAristaVeos:
		con, err = NewScrapliConsole(
			AristaVeosScrapliPlatform,
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
			CiscoCsr1000vScrapliPlatform,
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
			CiscoXrv9kScrapliPlatform,
			q.Hardware.SerialPorts[0],
			q.Credentials.Username,
			q.Credentials.Password,
			l,
			base.WithCommsReturnChar("\r"),
		)

		p = &CiscoXrv9k{
			Qemu:           q,
			ScrapliConsole: con,
		}
	case PlatformTypeCiscoN9kv:
		con, err = NewScrapliConsole(
			CiscoN9kvScrapliPlatform,
			q.Hardware.SerialPorts[0],
			q.Credentials.Username,
			q.Credentials.Password,
			l,
			base.WithCommsReturnChar("\r"),
		)

		p = &CiscoN9kv{
			Qemu:           q,
			ScrapliConsole: con,
		}
	case PlatformTypeJuniperVsrx:
		con, err = NewScrapliConsole(
			JuniperVsrxScrapliPlatform,
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
			PaloAltoPanosScrapliPlatform,
			q.Hardware.SerialPorts[0],
			q.Credentials.Username,
			q.Credentials.Password,
			l,
			base.WithCommsReturnChar("\r"),
		)

		p = &PaloAltoPanos{
			Qemu:           q,
			ScrapliConsole: con,
		}
	default:
		return nil, fmt.Errorf("%w: scrapligo driver is not found for %q platform",
			util.ErrAllocationError, pT)
	}

	return p, err
}
