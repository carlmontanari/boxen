package boxen

import (
	"context"
	"fmt"
	"net"
	"os"

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

func (b *Boxen) getAddr(ctx context.Context) (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}

	addrs, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if !addr.IsGlobalUnicast() {
			// obviously we need something reachable, this should eliminate loopbacks/multicast
			// etc so the container can reach back to us
			continue
		}

		res := addr.String()

		if res == "<nil>" {
			continue
		}

		return res, nil
	}

	return "", fmt.Errorf("%w: failed finding a reachable local address", boxenerrors.ErrBoxen)
}
