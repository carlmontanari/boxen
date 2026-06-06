package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	boxenconstants "github.com/carlmontanari/boxen/constants"
	boxenutil "github.com/carlmontanari/boxen/util"
)

const tapMTU = "65000"

// interfacePollInterval is how often watchers poll /sys/class/net for interfaces to
// appear.
var interfacePollInterval = 2 * time.Second

// runNetCommand executes an `ip`/`tc` command.
var runNetCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput() //nolint: gosec
}

// intfExists reports whether a network interface is present in the container
// netns.
var intfExists = func(name string) bool {
	_, err := os.Stat("/sys/class/net/" + name)

	return err == nil
}

// startTCService launches the boxen tc service: a watcher goroutine per data NIC
// that stitches the container's ethN<->tapN with tc and reacts to hotplug, plus
// a one-shot setup of the management interface when transparent management is
// enabled. Goroutines stop when ctx is cancelled. This must only run in `run`
// mode -- packaging boots the VM without datapath stitching.
func (a *Agent) startTCService(ctx context.Context) {
	a.l.Info("starting tc service")

	if a.p.VirtualMachine.IsManagementPassthroughEnabled() {
		// the management interface is always present and never hotplugged, so it is
		// plugged exactly once rather than watched
		go a.setupMgmtNIC(ctx)
	}

	for nicID := 1; nicID <= int(a.p.VirtualMachine.NicCount); nicID++ {
		go a.watchDataNIC(ctx, nicID)
	}
}

// watchNIC waits for tap (created by QEMU) to appear and runs setup once, then
// keeps the tc rules in sync with eth appearing/disappearing, re-wiring when a
// hotplugged interface is re-added.
func (a *Agent) watchNIC(
	ctx context.Context,
	tap, eth string,
	setup func(context.Context, string) error,
	wire func(context.Context, string, string) error,
) {
	tapReady := false
	wired := false

	ticker := time.NewTicker(interfacePollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !tapReady {
				if !intfExists(tap) {
					continue
				}

				if err := setup(ctx, tap); err != nil {
					a.l.Error("tc service: tap setup failed", "tap", tap, "error", err.Error())

					continue
				}

				tapReady = true
			}

			ethUp := intfExists(eth)

			switch {
			case ethUp && !wired:
				if err := wire(ctx, tap, eth); err != nil {
					a.l.Error(
						"tc service: wiring failed",
						"tap", tap, "eth", eth, "error", err.Error(),
					)

					continue
				}

				a.l.Info("tc service: interface wired", "tap", tap, "eth", eth)

				wired = true
			case !ethUp && wired:
				// interface was removed; allow re-wire when it comes back
				a.l.Info(
					"tc service: interface removed, will re-wire on return",
					"tap", tap, "eth", eth,
				)

				wired = false
			}
		}
	}
}

// runTC runs a sequence of `tc` invocations, returning on the first failure.
func (a *Agent) runTC(ctx context.Context, steps [][]string) error {
	for _, step := range steps {
		out, err := runNetCommand(ctx, "tc", step...)
		if err != nil {
			return fmt.Errorf("tc %v failed: %w: %s", step, err, string(out))
		}
	}

	return nil
}

// watchDataNIC stitches a data plane interface: tap{nicID} <-> ethN.
func (a *Agent) watchDataNIC(ctx context.Context, nicID int) {
	a.watchNIC(
		ctx,
		fmt.Sprintf("tap%d", nicID),
		boxenutil.ClabIntfName(nicID),
		a.bringTapUp,
		a.wireDataTap,
	)
}

// bringTapUp brings a tap interface up and sets the large MTU used for the
// datapath. This was previously done by the QEMU ifup script.
func (a *Agent) bringTapUp(ctx context.Context, tap string) error {
	if _, err := runNetCommand(ctx, "ip", "link", "set", tap, "up"); err != nil {
		return fmt.Errorf("failed bringing %s up: %w", tap, err)
	}

	if _, err := runNetCommand(ctx, "ip", "link", "set", tap, "mtu", tapMTU); err != nil {
		return fmt.Errorf("failed setting mtu on %s: %w", tap, err)
	}

	return nil
}

// wireDataTap installs the bidirectional tc mirred redirect rules between the
// container data interface and the VM tap. `qdisc replace` keeps it idempotent
// so initial wiring, hotplug, and re-plug all converge cleanly.
func (a *Agent) wireDataTap(ctx context.Context, tap, eth string) error {
	return a.runTC(ctx, [][]string{
		{"qdisc", "replace", "dev", eth, "ingress"},
		{
			"filter", "add", "dev", eth, "parent", "ffff:", "protocol", "all",
			"u32", "match", "u8", "0", "0",
			"action", "mirred", "egress", "redirect", "dev", tap,
		},
		{"qdisc", "replace", "dev", tap, "ingress"},
		{
			"filter", "add", "dev", tap, "parent", "ffff:", "protocol", "all",
			"u32", "match", "u8", "0", "0",
			"action", "mirred", "egress", "redirect", "dev", eth,
		},
	})
}

// setupMgmtNIC plugs the management interface a single time. Unlike data NICs,
// the management interface is always present and never hotplugged, so there is
// nothing to keep watching: wait for QEMU to create tap0, bring it up, and
// install the tc rules once.
func (a *Agent) setupMgmtNIC(ctx context.Context) {
	const tap = "tap0"

	mgmt := boxenutil.ClabMgmtIntfName()

	// wait only for QEMU's tap0 to appear before configuring it
	ticker := time.NewTicker(interfacePollInterval)
	defer ticker.Stop()

	for !intfExists(tap) {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}

	if err := a.bringMgmtTapUp(ctx, tap); err != nil {
		a.l.Error("tc service: mgmt tap setup failed", "tap", tap, "error", err.Error())

		return
	}

	if err := a.wireMgmtTap(ctx, tap, mgmt); err != nil {
		a.l.Error(
			"tc service: mgmt wiring failed",
			"tap", tap, "mgmt", mgmt, "error", err.Error(),
		)

		return
	}

	a.l.Info("tc service: management interface wired", "tap", tap, "mgmt", mgmt)
}

// bringMgmtTapUp brings the management tap up, sets the MTU, and flushes any
// IPv6 addresses so periodic traffic from the container itself does not leak
// into the VM management path.
func (a *Agent) bringMgmtTapUp(ctx context.Context, tap string) error {
	if _, err := runNetCommand(ctx, "ip", "link", "set", tap, "up"); err != nil {
		return fmt.Errorf("failed bringing %s up: %w", tap, err)
	}

	if _, err := runNetCommand(ctx, "ip", "link", "set", tap, "mtu", tapMTU); err != nil {
		return fmt.Errorf("failed setting mtu on %s: %w", tap, err)
	}

	// best-effort; the tap may have no IPv6 addresses to flush
	_, _ = runNetCommand(ctx, "ip", "-6", "addr", "flush", tap)

	return nil
}

// wireMgmtTap reproduces tc-tap-mgmt-ifup: it keeps the QEMU serial console
// reachable, mirrors ARP, and redirects management traffic between the mgmt
// interface and tap0, optionally pinning the mgmt MAC.
func (a *Agent) wireMgmtTap(ctx context.Context, tap, mgmt string) error {
	err := a.runTC(ctx, [][]string{
		{"qdisc", "replace", "dev", mgmt, "clsact"},
		// keep the QEMU serial console (tcp 5001-5008) reachable on the container
		{
			"filter", "replace", "dev", mgmt, "ingress", "prio", "1", "protocol", "ip",
			"flower", "ip_proto", "tcp", "dst_port", "5001-5008", "action", "pass",
		},
		// mirror ARP so the container network stack can still resolve neighbors
		{
			"filter", "replace", "dev", mgmt, "ingress", "prio", "2", "protocol", "arp",
			"flower", "action", "mirred", "egress", "mirror", "dev", tap,
		},
		// redirect normal management traffic to the VM management tap
		{
			"filter", "replace", "dev", mgmt, "ingress", "prio", "3",
			"flower", "action", "mirred", "egress", "redirect", "dev", tap,
		},
		{"qdisc", "replace", "dev", tap, "clsact"},
		{
			"filter", "replace", "dev", tap, "ingress",
			"flower", "action", "mirred", "egress", "redirect", "dev", mgmt,
		},
	})
	if err != nil {
		return err
	}

	mac := os.Getenv(boxenconstants.EnvClabMgmtMAC)
	if mac != "" {
		_, err = runNetCommand(ctx, "ip", "link", "set", "dev", mgmt, "address", mac)
		if err != nil {
			return fmt.Errorf("failed setting mgmt mac on %s: %w", mgmt, err)
		}
	}

	return nil
}
