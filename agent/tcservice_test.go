package agent

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	boxenconstants "github.com/carlmontanari/boxen/constants"
)

type capturedCmd struct {
	name string
	args []string
}

// stubNetCommand replaces runNetCommand with a recorder for the duration of the
// test, returning a pointer to the captured invocations.
func stubNetCommand(t *testing.T) *[]capturedCmd {
	t.Helper()

	var calls []capturedCmd

	orig := runNetCommand
	runNetCommand = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, capturedCmd{name: name, args: args})

		return nil, nil
	}

	t.Cleanup(func() { runNetCommand = orig })

	return &calls
}

func assertCalls(t *testing.T, got []capturedCmd, want [][]string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("expected %d commands, got %d: %v", len(want), len(got), got)
	}

	for i, w := range want {
		full := append([]string{got[i].name}, got[i].args...)
		if !slices.Equal(full, w) {
			t.Fatalf("command %d: expected %v, got %v", i, w, full)
		}
	}
}

func TestBringTapUp(t *testing.T) {
	calls := stubNetCommand(t)

	a := NewAgent(slog.LevelError)

	if err := a.bringTapUp(context.Background(), "tap1"); err != nil {
		t.Fatalf("bringTapUp returned error: %v", err)
	}

	assertCalls(t, *calls, [][]string{
		{"ip", "link", "set", "tap1", "up"},
		{"ip", "link", "set", "tap1", "mtu", "65000"},
	})
}

func TestWireDataTap(t *testing.T) {
	calls := stubNetCommand(t)

	a := NewAgent(slog.LevelError)

	if err := a.wireDataTap(context.Background(), "tap1", "eth1"); err != nil {
		t.Fatalf("wireDataTap returned error: %v", err)
	}

	assertCalls(t, *calls, [][]string{
		{"tc", "qdisc", "replace", "dev", "eth1", "ingress"},
		{
			"tc", "filter", "add", "dev", "eth1", "parent", "ffff:", "protocol", "all",
			"u32", "match", "u8", "0", "0", "action", "mirred", "egress", "redirect", "dev", "tap1",
		},
		{"tc", "qdisc", "replace", "dev", "tap1", "ingress"},
		{
			"tc", "filter", "add", "dev", "tap1", "parent", "ffff:", "protocol", "all",
			"u32", "match", "u8", "0", "0", "action", "mirred", "egress", "redirect", "dev", "eth1",
		},
	})
}

func TestBringMgmtTapUp(t *testing.T) {
	calls := stubNetCommand(t)

	a := NewAgent(slog.LevelError)

	if err := a.bringMgmtTapUp(context.Background(), "tap0"); err != nil {
		t.Fatalf("bringMgmtTapUp returned error: %v", err)
	}

	assertCalls(t, *calls, [][]string{
		{"ip", "link", "set", "tap0", "up"},
		{"ip", "link", "set", "tap0", "mtu", "65000"},
		{"ip", "-6", "addr", "flush", "tap0"},
	})
}

func TestWireMgmtTapWithMAC(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtMAC, "02:00:00:00:00:09")

	calls := stubNetCommand(t)

	a := NewAgent(slog.LevelError)

	if err := a.wireMgmtTap(context.Background(), "tap0", "eth0"); err != nil {
		t.Fatalf("wireMgmtTap returned error: %v", err)
	}

	assertCalls(t, *calls, [][]string{
		{"tc", "qdisc", "replace", "dev", "eth0", "clsact"},
		{
			"tc", "filter", "replace", "dev", "eth0", "ingress", "prio", "1", "protocol", "ip",
			"flower", "ip_proto", "tcp", "dst_port", "5001-5008", "action", "pass",
		},
		{
			"tc", "filter", "replace", "dev", "eth0", "ingress", "prio", "2", "protocol", "arp",
			"flower", "action", "mirred", "egress", "mirror", "dev", "tap0",
		},
		{
			"tc", "filter", "replace", "dev", "eth0", "ingress", "prio", "3",
			"flower", "action", "mirred", "egress", "redirect", "dev", "tap0",
		},
		{"tc", "qdisc", "replace", "dev", "tap0", "clsact"},
		{
			"tc", "filter", "replace", "dev", "tap0", "ingress",
			"flower", "action", "mirred", "egress", "redirect", "dev", "eth0",
		},
		{"ip", "link", "set", "dev", "eth0", "address", "02:00:00:00:00:09"},
	})
}

func TestWireMgmtTapNoMAC(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtMAC, "")

	calls := stubNetCommand(t)

	a := NewAgent(slog.LevelError)

	if err := a.wireMgmtTap(context.Background(), "tap0", "eth0"); err != nil {
		t.Fatalf("wireMgmtTap returned error: %v", err)
	}

	if len(*calls) != 6 {
		t.Fatalf("expected 6 tc commands when no mac is set, got %d: %v", len(*calls), *calls)
	}

	for _, c := range *calls {
		if c.name != "tc" {
			t.Fatalf("expected only tc commands when no mac is set, got %q", c.name)
		}
	}
}

// fakeNet is a concurrency-safe stand-in for the host network state used to
// drive watchNIC transitions deterministically.
type fakeNet struct {
	mu      sync.Mutex
	present map[string]bool
	calls   []capturedCmd
}

func (f *fakeNet) setPresent(name string, present bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.present[name] = present
}

func (f *fakeNet) exists(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.present[name]
}

func (f *fakeNet) record(name string, args ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, capturedCmd{name: name, args: args})
}

func (f *fakeNet) count(pred func(capturedCmd) bool) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	n := 0

	for _, c := range f.calls {
		if pred(c) {
			n++
		}
	}

	return n
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(2 * time.Millisecond) //nolint:mnd
	}

	t.Fatalf("condition not met within %s", timeout)
}

func TestWatchNICWireAndRewire(t *testing.T) {
	origPoll := interfacePollInterval
	interfacePollInterval = 5 * time.Millisecond
	t.Cleanup(func() { interfacePollInterval = origPoll })

	fn := &fakeNet{present: map[string]bool{}}

	origExists := intfExists
	origRun := runNetCommand
	intfExists = fn.exists
	runNetCommand = func(_ context.Context, name string, args ...string) ([]byte, error) {
		fn.record(name, args...)

		return nil, nil
	}

	t.Cleanup(func() {
		intfExists = origExists
		runNetCommand = origRun
	})

	// count "tc qdisc replace dev eth1 ingress" as a proxy for a (re)wire
	wireCount := func() int {
		return fn.count(func(c capturedCmd) bool {
			return c.name == "tc" &&
				slices.Equal(c.args, []string{"qdisc", "replace", "dev", "eth1", "ingress"})
		})
	}

	tapUpCount := func() int {
		return fn.count(func(c capturedCmd) bool {
			return c.name == "ip" &&
				slices.Equal(c.args, []string{"link", "set", "tap1", "up"})
		})
	}

	a := NewAgent(slog.LevelError)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// tap exists from the start; the container interface does not yet
	fn.setPresent("tap1", true)

	done := make(chan struct{})

	go func() {
		a.watchNIC(ctx, "tap1", "eth1", a.bringTapUp, a.wireDataTap)
		close(done)
	}()

	// the tap should be brought up even before the container interface appears
	waitFor(t, time.Second, func() bool { return tapUpCount() >= 1 })

	if wireCount() != 0 {
		t.Fatalf("should not wire before eth1 exists, got %d", wireCount())
	}

	// container interface appears -> wired once
	fn.setPresent("eth1", true)
	waitFor(t, time.Second, func() bool { return wireCount() == 1 })

	// while it stays present, it must not be re-wired on every tick
	time.Sleep(40 * time.Millisecond) //nolint:mnd
	if got := wireCount(); got != 1 {
		t.Fatalf("expected exactly 1 wire while eth1 stays present, got %d", got)
	}

	// interface removed then re-added (hotplug) -> wired again
	fn.setPresent("eth1", false)
	time.Sleep(40 * time.Millisecond) //nolint:mnd
	fn.setPresent("eth1", true)
	waitFor(t, time.Second, func() bool { return wireCount() == 2 })

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watchNIC did not exit after context cancellation")
	}
}

func TestSetupMgmtNICPlugsOnce(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtMAC, "")

	fn := &fakeNet{present: map[string]bool{"tap0": true}}

	origExists := intfExists
	origRun := runNetCommand
	intfExists = fn.exists
	runNetCommand = func(_ context.Context, name string, args ...string) ([]byte, error) {
		fn.record(name, args...)

		return nil, nil
	}

	t.Cleanup(func() {
		intfExists = origExists
		runNetCommand = origRun
	})

	a := NewAgent(slog.LevelError)

	// tap0 is already present, so this returns after plugging exactly once
	a.setupMgmtNIC(context.Background())

	// 3 ip commands (up, mtu, ipv6 flush) + 6 tc rules, with no mac set
	total := fn.count(func(capturedCmd) bool { return true })
	if total != 9 {
		t.Fatalf("expected 9 commands for a single mgmt plug, got %d", total)
	}
}
