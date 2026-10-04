package agent

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
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
		{"tc", "qdisc", "del", "dev", "eth1", "ingress"},
		{"tc", "qdisc", "del", "dev", "tap1", "ingress"},
		{"tc", "qdisc", "add", "dev", "eth1", "ingress"},
		{
			"tc", "filter", "add", "dev", "eth1", "parent", "ffff:", "protocol", "all",
			"u32", "match", "u8", "0", "0", "action", "mirred", "egress", "redirect", "dev", "tap1",
		},
		{"tc", "qdisc", "add", "dev", "tap1", "ingress"},
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
			"flower", "action", "csum", "ip", "and", "tcp", "and", "udp", "and", "icmp", "pipe",
			"action", "mirred", "egress", "redirect", "dev", "tap0",
		},
		{"tc", "qdisc", "replace", "dev", "tap0", "clsact"},
		{
			"tc", "filter", "replace", "dev", "tap0", "ingress",
			"flower", "action", "mirred", "egress", "redirect", "dev", "eth0",
		},
		{"ip", "link", "set", "dev", "eth0", "address", "02:00:00:00:00:09"},
		{"sysctl", "-qw", "net/ipv4/conf/eth0/arp_ignore=8"},
		{"sysctl", "-qw", "net/ipv4/conf/eth0/arp_announce=2"},
	})
}

func TestWireMgmtTapNoMAC(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtMAC, "")

	calls := stubNetCommand(t)

	a := NewAgent(slog.LevelError)

	if err := a.wireMgmtTap(context.Background(), "mgmt-tap", "mgmt.100"); err != nil {
		t.Fatalf("wireMgmtTap returned error: %v", err)
	}

	if len(*calls) != 8 {
		t.Fatalf("expected 6 tc and 2 sysctl commands, got %d: %v", len(*calls), *calls)
	}

	for _, c := range (*calls)[:6] {
		if c.name != "tc" {
			t.Fatalf("expected tc commands before sysctl, got %q", c.name)
		}
	}

	assertCalls(t, (*calls)[3:4], [][]string{
		{
			"tc", "filter", "replace", "dev", "mgmt.100", "ingress", "prio", "3",
			"flower", "action", "csum", "ip", "and", "tcp", "and", "udp", "and", "icmp", "pipe",
			"action", "mirred", "egress", "redirect", "dev", "mgmt-tap",
		},
	})
	assertCalls(t, (*calls)[6:], [][]string{
		{"sysctl", "-qw", "net/ipv4/conf/mgmt.100/arp_ignore=8"},
		{"sysctl", "-qw", "net/ipv4/conf/mgmt.100/arp_announce=2"},
	})
}

func TestWireMgmtTapARPSettingsBestEffort(t *testing.T) {
	t.Setenv(boxenconstants.EnvClabMgmtMAC, "")

	calls := stubNetCommand(t)
	record := runNetCommand
	runNetCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		out, err := record(ctx, name, args...)
		if name == "sysctl" {
			return []byte("sysctl: permission denied"), errors.ErrUnsupported
		}

		return out, err
	}

	a := NewAgent(slog.LevelError)

	if err := a.wireMgmtTap(context.Background(), "tap0", "eth0"); err != nil {
		t.Fatalf("sysctl failure should not fail management wiring: %v", err)
	}

	if len(*calls) != 8 {
		t.Fatalf("expected both sysctl commands to be attempted, got %d commands", len(*calls))
	}

	assertCalls(t, (*calls)[6:], [][]string{
		{"sysctl", "-qw", "net/ipv4/conf/eth0/arp_ignore=8"},
		{"sysctl", "-qw", "net/ipv4/conf/eth0/arp_announce=2"},
	})
}

func TestWireMgmtTapChecksumFailure(t *testing.T) {
	calls := stubNetCommand(t)
	record := runNetCommand
	wantErr := errors.ErrUnsupported
	runNetCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		out, err := record(ctx, name, args...)
		if slices.Contains(args, "csum") {
			return []byte("Failed to load kernel module act_csum"), wantErr
		}

		return out, err
	}

	a := NewAgent(slog.LevelError)

	err := a.wireMgmtTap(context.Background(), "tap0", "eth0")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected checksum action error, got %v", err)
	}

	if !strings.Contains(err.Error(), "Failed to load kernel module act_csum") {
		t.Fatalf("expected tc output in error, got %v", err)
	}

	if len(*calls) != 4 {
		t.Fatalf("expected wiring to stop at checksum failure, got %d commands", len(*calls))
	}
}

// fakeNet is a concurrency-safe stand-in for the host network state used to
// drive watchNIC transitions deterministically.
type fakeNet struct {
	mu        sync.Mutex
	present   map[string]bool
	index     map[string]int
	lastIndex int
	calls     []capturedCmd
}

// setPresent adds or removes an interface; an added interface gets a new index.
func (f *fakeNet) setPresent(name string, present bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if present && !f.present[name] {
		f.newIndex(name)
	}

	f.present[name] = present
}

// replug removes and adds an interface at once, as between two polls.
func (f *fakeNet) replug(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.newIndex(name)
	f.present[name] = true
}

func (f *fakeNet) newIndex(name string) {
	if f.index == nil {
		f.index = map[string]int{}
	}

	f.lastIndex++
	f.index[name] = f.lastIndex
}

func (f *fakeNet) exists(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.present[name]
}

func (f *fakeNet) indexOf(name string) (int, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.index[name], f.present[name]
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

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	const timeout = time.Second

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("condition not met within %s", timeout)
}

func TestWatchNICWireAndRewire(t *testing.T) {
	origPoll := interfacePollInterval
	interfacePollInterval = 5 * time.Millisecond
	t.Cleanup(func() { interfacePollInterval = origPoll })

	fn := &fakeNet{present: map[string]bool{}}

	origExists := intfExists
	origIndex := intfIndex
	origRun := runNetCommand
	intfExists = fn.exists
	intfIndex = fn.indexOf
	runNetCommand = func(_ context.Context, name string, args ...string) ([]byte, error) {
		fn.record(name, args...)

		return nil, nil
	}

	t.Cleanup(func() {
		intfExists = origExists
		intfIndex = origIndex
		runNetCommand = origRun
	})

	// count "tc qdisc add dev eth1 ingress" as a proxy for a (re)wire
	wireCount := func() int {
		return fn.count(func(c capturedCmd) bool {
			return c.name == "tc" &&
				slices.Equal(c.args, []string{"qdisc", "add", "dev", "eth1", "ingress"})
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
	waitFor(t, func() bool { return tapUpCount() >= 1 })

	if wireCount() != 0 {
		t.Fatalf("should not wire before eth1 exists, got %d", wireCount())
	}

	// container interface appears -> wired once
	fn.setPresent("eth1", true)
	waitFor(t, func() bool { return wireCount() == 1 })

	// while it stays present, it must not be re-wired on every tick
	time.Sleep(40 * time.Millisecond)
	if got := wireCount(); got != 1 {
		t.Fatalf("expected exactly 1 wire while eth1 stays present, got %d", got)
	}

	// interface removed then re-added (hotplug) -> wired again
	fn.setPresent("eth1", false)
	time.Sleep(40 * time.Millisecond)
	fn.setPresent("eth1", true)
	waitFor(t, func() bool { return wireCount() == 2 })

	// removed and re-added between two polls -> recognized by its new index
	fn.replug("eth1")
	waitFor(t, func() bool { return wireCount() == 3 })

	time.Sleep(40 * time.Millisecond)
	if got := wireCount(); got != 3 {
		t.Fatalf("expected no further wiring while eth1 stays present, got %d", got)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watchNIC did not exit after context cancellation")
	}
}

func TestWatchNICRetriesPartialWiringFailure(t *testing.T) {
	origPoll := interfacePollInterval
	origExists, origIndex, origRun := intfExists, intfIndex, runNetCommand
	t.Cleanup(func() {
		interfacePollInterval = origPoll
		intfExists, intfIndex, runNetCommand = origExists, origIndex, origRun
	})

	interfacePollInterval = 5 * time.Millisecond
	fn := &fakeNet{present: map[string]bool{}}
	fn.setPresent("tap1", true)
	fn.setPresent("eth1", true)
	intfExists, intfIndex = fn.exists, fn.indexOf

	wantErr := errors.ErrUnsupported
	failOnce := true
	runNetCommand = func(_ context.Context, name string, args ...string) ([]byte, error) {
		fn.record(name, args...)
		// Fail the final redirect after the other qdiscs and filter have been installed.
		if failOnce && name == "tc" && len(args) >= 4 &&
			slices.Equal(args[:4], []string{"filter", "add", "dev", "tap1"}) {
			failOnce = false

			return []byte("injected redirect failure"), wantErr
		}

		return nil, nil
	}

	a := NewAgent(slog.LevelError)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	defer func() {
		cancel()
		<-done
	}()

	results := make(chan error, 2)
	go func() {
		defer close(done)
		a.watchNIC(
			ctx,
			"tap1",
			"eth1",
			a.bringTapUp,
			func(ctx context.Context, tap, eth string) error {
				err := a.wireDataTap(ctx, tap, eth)
				select {
				case results <- err:
				case <-ctx.Done():
				}

				return err
			},
		)
	}()

	// The same interface index must retry after failure and then wire successfully.
	for attempt, want := range []error{wantErr, nil} {
		select {
		case err := <-results:
			if !errors.Is(err, want) {
				t.Fatalf("attempt %d: expected %v, got %v", attempt+1, want, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("wiring attempt %d did not complete", attempt+1)
		}
	}

	cancel()
	<-done

	// Tap setup runs once, followed by two complete six-command wiring attempts.
	if len(fn.calls) != 14 {
		t.Fatalf(
			"expected 14 commands for setup and two wiring attempts, got %d: %v",
			len(fn.calls),
			fn.calls,
		)
	}

	first, retry := fn.calls[2:8], fn.calls[8:]
	assertCalls(t, retry[:2], [][]string{
		{"tc", "qdisc", "del", "dev", "eth1", "ingress"},
		{"tc", "qdisc", "del", "dev", "tap1", "ingress"},
	})
	if !slices.EqualFunc(first, retry, func(a, b capturedCmd) bool {
		return a.name == b.name && slices.Equal(a.args, b.args)
	}) {
		t.Fatalf("retry should repeat the full wiring sequence: first=%v retry=%v", first, retry)
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

	// 3 ip commands (up, mtu, ipv6 flush) + 6 tc rules + 2 sysctl settings
	total := fn.count(func(capturedCmd) bool { return true })
	if total != 11 {
		t.Fatalf("expected 11 commands for a single mgmt plug, got %d", total)
	}
}
