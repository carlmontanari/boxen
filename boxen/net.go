package boxen

import (
	"context"
	"fmt"
	"net"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenutil "github.com/carlmontanari/boxen/util"
)

func (b *Boxen) getListener(ctx context.Context) (net.Listener, error) {
	host := boxenutil.GetEnvStrOrDefault(
		boxenconstants.EnvListenHost,
		boxenconstants.DefaultListenHost,
	)

	port := boxenconstants.DefaultBoxenListenPort

	l := &net.ListenConfig{}

	lis, err := l.Listen(ctx, "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		b.l.Error(
			"failed establishing listener",
			"host",
			host,
			"port",
			port,
		)

		return nil, err
	}

	return lis, nil
}

func (b *Boxen) getAddr() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		if iface.Flags&(net.FlagUp|net.FlagLoopback) != net.FlagUp {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			var ip net.IP

			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil {
				continue
			}

			ip = ip.To4()
			if ip == nil {
				continue
			}

			if !ip.IsGlobalUnicast() {
				continue
			}

			return ip.String(), nil
		}
	}

	return "", fmt.Errorf("%w: failed finding a reachable local address", boxenerrors.ErrBoxen)
}
