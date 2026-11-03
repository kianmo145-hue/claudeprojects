package guard

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeNet struct {
	mu     sync.Mutex
	rtt    map[string]time.Duration // missing = down
	direct bool
	sel    []string
	probes map[string]int
}

func newFake() *fakeNet {
	return &fakeNet{rtt: map[string]time.Duration{}, direct: true, probes: map[string]int{}}
}

func (f *fakeNet) set(tag string, rtt time.Duration) { f.mu.Lock(); f.rtt[tag] = rtt; f.mu.Unlock() }
func (f *fakeNet) down(tag string)                   { f.mu.Lock(); delete(f.rtt, tag); f.mu.Unlock() }

func (f *fakeNet) probe(ctx context.Context, tag string) (time.Duration, error) {
	f.mu.Lock()
	f.probes[tag]++
	rtt, ok := f.rtt[tag]
	f.mu.Unlock()
	if !ok {
		select {
		case <-time.After(20 * time.Millisecond):
		case <-ctx.Done():
		}
		return 0, errors.New("timeout")
	}
	select {
	case <-time.After(rtt / 100): // keep tests fast; ranking uses the reported value
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	return rtt, nil
}

func (f *fakeNet) selectTag(tag string) error {
	f.mu.Lock()
	f.sel = append(f.sel, tag)
	f.mu.Unlock()
	return nil
}
func (f *fakeNet) directOK(context.Context) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.direct
}

func testConfig() Config {
	c := DefaultConfig()
	c.ActiveInterval = 30 * time.Millisecond
	c.ConfirmDelay = 5 * time.Millisecond
	c.StandbyInterval = 60 * time.Millisecond
	c.SweepInterval = 200 * time.Millisecond
	c.ProbeTimeout = 100 * time.Millisecond
	c.UpgradeMinGap = 300 * time.Millisecond
	c.OfflineRetry = 50 * time.Millisecond
	return c
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestFailoverToWorkingServer(t *testing.T) {
	f := newFake()
	f.set("a", 100*time.Millisecond)
	f.set("b", 300*time.Millisecond)
	f.set("c", 200*time.Millisecond)
	g := New(testConfig(), []string{"a", "b", "c"}, "a", f.probe, f.selectTag, f.directOK, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)

	time.Sleep(100 * time.Millisecond)
	if g.Current() != "a" {
		t.Fatalf("moved off a healthy server: %s", g.Current())
	}
	f.down("a")
	waitFor(t, "failover", func() bool { return g.Current() != "a" })
	if g.Current() != "c" {
		t.Errorf("failed over to %s, want the faster standby c", g.Current())
	}
}

func TestStartsOnFirstWorkingWhenRememberedIsDead(t *testing.T) {
	f := newFake()
	f.set("b", 300*time.Millisecond)
	g := New(testConfig(), []string{"a", "b", "c"}, "a", f.probe, f.selectTag, f.directOK, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)
	waitFor(t, "rescue", func() bool { return g.Current() == "b" })
}

func TestNoSwitchingWhenLocalNetworkIsDown(t *testing.T) {
	f := newFake()
	f.set("a", 100*time.Millisecond)
	f.set("b", 100*time.Millisecond)
	g := New(testConfig(), []string{"a", "b"}, "a", f.probe, f.selectTag, f.directOK, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)
	time.Sleep(80 * time.Millisecond)

	f.mu.Lock()
	f.rtt = map[string]time.Duration{}
	f.direct = false
	f.mu.Unlock()
	waitFor(t, "offline", g.Offline)
	time.Sleep(150 * time.Millisecond)
	if g.Current() != "a" {
		t.Errorf("switched to %s while offline", g.Current())
	}
	for _, h := range g.Snapshot() {
		if h.Tag == "b" && h.Fails > 0 {
			t.Errorf("standby b penalised while offline: %+v", h)
		}
	}
	f.set("a", 100*time.Millisecond)
	f.mu.Lock()
	f.direct = true
	f.mu.Unlock()
	waitFor(t, "online", func() bool { return !g.Offline() })
}

func TestManualModeNeverSwitches(t *testing.T) {
	f := newFake()
	f.set("b", 100*time.Millisecond)
	c := testConfig()
	c.AutoSwitch = false
	g := New(c, []string{"a", "b"}, "a", f.probe, f.selectTag, f.directOK, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)
	time.Sleep(300 * time.Millisecond)
	if g.Current() != "a" {
		t.Errorf("manual mode switched to %s", g.Current())
	}
}

func TestUpgradeNeedsBigMargin(t *testing.T) {
	f := newFake()
	f.set("a", 800*time.Millisecond)
	f.set("b", 600*time.Millisecond) // better, but not by enough
	g := New(testConfig(), []string{"a", "b"}, "a", f.probe, f.selectTag, f.directOK, nil)
	g.lastSwitch = time.Now().Add(-time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)
	time.Sleep(700 * time.Millisecond)
	if g.Current() != "a" {
		t.Fatalf("switched for a small gain")
	}
	f.set("b", 100*time.Millisecond)
	waitFor(t, "upgrade", func() bool { return g.Current() == "b" })
}

func TestChoosePinsAndReturns(t *testing.T) {
	f := newFake()
	f.set("a", 100*time.Millisecond)
	f.set("b", 100*time.Millisecond)
	c := testConfig()
	c.UpgradeMinGap = 100 * time.Millisecond
	g := New(c, []string{"a", "b"}, "a", f.probe, f.selectTag, f.directOK, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)
	if err := g.Choose("b"); err != nil {
		t.Fatal(err)
	}
	f.down("b")
	waitFor(t, "failover from the chosen server", func() bool { return g.Current() == "a" })
	f.set("b", 100*time.Millisecond)
	waitFor(t, "return to the chosen server", func() bool { return g.Current() == "b" })
}
