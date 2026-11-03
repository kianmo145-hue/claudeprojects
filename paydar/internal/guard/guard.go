// Package guard keeps the connection alive: it watches the active server, keeps a few standbys
// warm, and moves traffic to a working server as soon as the active one stops answering.
//
// Three loops, all driven by real probes through each server (an HTTP 204 fetch):
//
//   - active:  the selected server, every few seconds. Two misses in a row (the second taken one
//     second after the first) count as down, and traffic fails over at once.
//   - standby: the best few other servers, so a failover target is known to work right now.
//   - sweep:   every server, rarely, to keep scores fresh and find better ones.
//
// A failover tries the warm standbys first, then races every server and takes the first that
// answers. When nothing answers, a direct check tells "the local network is down" apart from
// "the servers are down", so a lost Wi-Fi does not mark every server as broken.
//
// Switching to a merely faster server (not a failover) needs a large, sustained margin and a
// minimum time since the last switch, since every switch resets open connections.
package guard

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"time"
)

// Prober measures one round trip through the outbound with the given tag.
type Prober func(ctx context.Context, tag string) (time.Duration, error)

// Selector moves traffic to the outbound with the given tag.
type Selector func(tag string) error

// DirectCheck reports whether the local network reaches the internet without any proxy.
type DirectCheck func(ctx context.Context) bool

type Config struct {
	ActiveInterval  time.Duration
	ConfirmDelay    time.Duration
	StandbyInterval time.Duration
	SweepInterval   time.Duration
	ProbeTimeout    time.Duration
	FailThreshold   int
	Standbys        int
	Concurrency     int
	// AutoSwitch off: only monitor; the user's choice is never changed.
	AutoSwitch bool
	// PreferFaster: also move to a clearly faster server while the current one works.
	PreferFaster  bool
	UpgradeRatio  float64
	UpgradeMinGap time.Duration
	OfflineRetry  time.Duration
}

func DefaultConfig() Config {
	return Config{
		ActiveInterval:  5 * time.Second,
		ConfirmDelay:    time.Second,
		StandbyInterval: 30 * time.Second,
		SweepInterval:   4 * time.Minute,
		ProbeTimeout:    6 * time.Second,
		FailThreshold:   2,
		Standbys:        3,
		Concurrency:     8,
		AutoSwitch:      true,
		PreferFaster:    true,
		UpgradeRatio:    0.5,
		UpgradeMinGap:   10 * time.Minute,
		OfflineRetry:    8 * time.Second,
	}
}

const window = 8

type health struct {
	ewma, jitter float64 // ms
	results      [window]bool
	n, pos       int
	fails        int
	okStreak     int
	lastOK       time.Time
	lastProbe    time.Time
	lastRTT      float64
	lastErr      string
	cooldown     time.Time
	probing      bool
}

func (h *health) rate() float64 {
	if h.n == 0 {
		return 0
	}
	ok := 0
	for i := 0; i < h.n; i++ {
		if h.results[i] {
			ok++
		}
	}
	return float64(ok) / float64(h.n)
}

// score: lower is better; +Inf for never-tested.
func (h *health) score() float64 {
	if h.n == 0 || h.ewma == 0 && h.fails > 0 {
		return math.Inf(1)
	}
	s := h.ewma + 0.5*h.jitter
	s *= 1 + 3*(1-h.rate())
	s += 2000 * float64(h.fails)
	return s
}

func (h *health) record(now time.Time, rtt time.Duration, err error) {
	h.lastProbe = now
	h.results[h.pos] = err == nil
	h.pos = (h.pos + 1) % window
	if h.n < window {
		h.n++
	}
	if err != nil {
		h.fails++
		h.okStreak = 0
		h.lastErr = err.Error()
		backoff := 30 * time.Second << min(h.fails-1, 5)
		h.cooldown = now.Add(min(backoff, 10*time.Minute))
		return
	}
	ms := float64(rtt) / float64(time.Millisecond)
	if h.ewma == 0 {
		h.ewma = ms
	} else {
		h.jitter = 0.7*h.jitter + 0.3*math.Abs(ms-h.ewma)
		h.ewma = 0.7*h.ewma + 0.3*ms
	}
	h.lastRTT = ms
	h.fails = 0
	h.okStreak++
	h.lastOK = now
	h.lastErr = ""
	h.cooldown = time.Time{}
}

// Health is what the UI shows for one server.
type Health struct {
	Tag       string  `json:"tag"`
	State     string  `json:"state"` // unknown, testing, ok, slow, down
	RTT       int     `json:"rtt"`   // smoothed, ms
	LastRTT   int     `json:"lastRtt"`
	Jitter    int     `json:"jitter"`
	Success   float64 `json:"success"` // 0..1 over the last probes
	Samples   int     `json:"samples"`
	Fails     int     `json:"fails"`
	LastError string  `json:"lastError,omitempty"`
	LastOK    int64   `json:"lastOk,omitempty"` // unix ms
	Score     float64 `json:"score"`
}

type Event struct {
	Time   int64  `json:"time"`
	Kind   string `json:"kind"` // switch, down, offline, online
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Reason string `json:"reason"`
}

type Guard struct {
	cfg    Config
	probe  Prober
	sel    Selector
	direct DirectCheck
	notify func(Event)

	mu         sync.Mutex
	tags       []string
	h          map[string]*health
	current    string
	preferred  string
	lastSwitch time.Time
	offline    bool
	events     []Event
	failing    bool
	nextRescue time.Time
	backoff    time.Duration

	kick chan struct{}
	sem  chan struct{}
	now  func() time.Time
}

func New(cfg Config, tags []string, current string, probe Prober, sel Selector, direct DirectCheck, notify func(Event)) *Guard {
	g := &Guard{
		cfg: cfg, probe: probe, sel: sel, direct: direct, notify: notify,
		h: map[string]*health{}, kick: make(chan struct{}, 1),
		sem: make(chan struct{}, max(cfg.Concurrency, 1)), now: time.Now,
	}
	g.tags = append([]string(nil), tags...)
	for _, t := range tags {
		g.h[t] = &health{}
	}
	if _, ok := g.h[current]; !ok && len(tags) > 0 {
		current = tags[0]
	}
	g.current = current
	return g
}

func (g *Guard) Current() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.current
}

func (g *Guard) Offline() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.offline
}

func (g *Guard) SetConfig(c Config) {
	g.mu.Lock()
	g.cfg = c
	g.mu.Unlock()
	g.Kick()
}

// Kick asks for an immediate check (network changed, app resumed).
func (g *Guard) Kick() {
	select {
	case g.kick <- struct{}{}:
	default:
	}
}

// Choose is a user's pick: it becomes current and preferred.
func (g *Guard) Choose(tag string) error {
	g.mu.Lock()
	if _, ok := g.h[tag]; !ok {
		g.mu.Unlock()
		return errors.New("unknown server")
	}
	from := g.current
	g.current, g.preferred = tag, tag
	g.lastSwitch = g.now()
	g.mu.Unlock()
	if err := g.sel(tag); err != nil {
		return err
	}
	g.emit(Event{Kind: "switch", From: from, To: tag, Reason: "manual"})
	g.Kick()
	return nil
}

func (g *Guard) Snapshot() []Health {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]Health, 0, len(g.tags))
	for _, t := range g.tags {
		h := g.h[t]
		s := Health{
			Tag: t, RTT: int(h.ewma), LastRTT: int(h.lastRTT), Jitter: int(h.jitter), Success: h.rate(),
			Samples: h.n, Fails: h.fails, LastError: h.lastErr, Score: h.score(),
		}
		if math.IsInf(s.Score, 1) {
			s.Score = -1
		}
		if !h.lastOK.IsZero() {
			s.LastOK = h.lastOK.UnixMilli()
		}
		switch {
		case h.probing && h.n == 0:
			s.State = "testing"
		case h.n == 0:
			s.State = "unknown"
		case h.fails > 0:
			s.State = "down"
		case h.ewma > 1500 || h.rate() < 0.75:
			s.State = "slow"
		default:
			s.State = "ok"
		}
		out = append(out, s)
	}
	return out
}

func (g *Guard) Events() []Event {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]Event(nil), g.events...)
}

func (g *Guard) emit(e Event) {
	e.Time = g.now().UnixMilli()
	g.mu.Lock()
	g.events = append(g.events, e)
	if len(g.events) > 50 {
		g.events = g.events[len(g.events)-50:]
	}
	g.mu.Unlock()
	if g.notify != nil {
		g.notify(e)
	}
}

// probeOne runs one probe (bounded by the concurrency limit) and records it.
func (g *Guard) probeOne(ctx context.Context, tag string) (time.Duration, error) {
	select {
	case g.sem <- struct{}{}:
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	defer func() { <-g.sem }()
	g.mu.Lock()
	h := g.h[tag]
	if h == nil {
		g.mu.Unlock()
		return 0, errors.New("unknown server")
	}
	h.probing = true
	timeout := g.cfg.ProbeTimeout
	g.mu.Unlock()

	pctx, cancel := context.WithTimeout(ctx, timeout)
	rtt, err := g.probe(pctx, tag)
	cancel()

	if ctx.Err() != nil {
		// Shutting down: not the server's fault.
		g.mu.Lock()
		h.probing = false
		g.mu.Unlock()
		return 0, ctx.Err()
	}
	g.mu.Lock()
	h.probing = false
	// While the local network is down, failures say nothing about the server.
	if err == nil || !g.offline {
		h.record(g.now(), rtt, err)
	}
	g.mu.Unlock()
	return rtt, err
}

// ProbeAll tests every server now (the UI's "test" button) and waits for the results.
func (g *Guard) ProbeAll(ctx context.Context) {
	g.mu.Lock()
	tags := append([]string(nil), g.tags...)
	g.mu.Unlock()
	var wg sync.WaitGroup
	for _, t := range tags {
		wg.Add(1)
		go func() { defer wg.Done(); g.probeOne(ctx, t) }()
	}
	wg.Wait()
}

// ranked returns the tags sorted by score, best first, excluding skip. Servers in cooldown are left
// out unless includeCooling.
func (g *Guard) ranked(skip string, includeCooling bool) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	var out []string
	for _, t := range g.tags {
		if t == skip {
			continue
		}
		if !includeCooling && g.h[t].cooldown.After(now) {
			continue
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool { return g.h[out[i]].score() < g.h[out[j]].score() })
	return out
}

func (g *Guard) switchTo(tag, reason string) {
	g.mu.Lock()
	from := g.current
	if from == tag {
		g.mu.Unlock()
		return
	}
	g.current = tag
	g.lastSwitch = g.now()
	g.mu.Unlock()
	if err := g.sel(tag); err != nil {
		return
	}
	g.emit(Event{Kind: "switch", From: from, To: tag, Reason: reason})
}

// failover finds a working server and moves to it. It returns false if none answered.
func (g *Guard) failover(ctx context.Context, reason string) bool {
	g.mu.Lock()
	if g.failing {
		g.mu.Unlock()
		return true
	}
	g.failing = true
	cur := g.current
	g.mu.Unlock()
	defer func() { g.mu.Lock(); g.failing = false; g.mu.Unlock() }()

	// 1. Warm standbys, best first: one quick confirmation each.
	tried := map[string]bool{cur: true}
	for i, t := range g.ranked(cur, false) {
		if i >= g.cfg.Standbys {
			break
		}
		g.mu.Lock()
		h := g.h[t]
		fresh := h.fails == 0 && h.n > 0 && g.now().Sub(h.lastOK) < 3*g.cfg.StandbyInterval
		g.mu.Unlock()
		if !fresh {
			continue
		}
		tried[t] = true
		if _, err := g.probeOne(ctx, t); err == nil {
			g.switchTo(t, reason)
			return true
		}
	}

	// 2. Race all the rest (cooling ones too) and take the first that answers.
	rest := []string{}
	for _, t := range g.ranked(cur, true) {
		if !tried[t] {
			rest = append(rest, t)
		}
	}
	if len(rest) > 0 {
		rctx, cancel := context.WithCancel(ctx)
		winner := make(chan string, len(rest))
		var wg sync.WaitGroup
		for _, t := range rest {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := g.probeOne(rctx, t); err == nil {
					winner <- t
				}
			}()
		}
		go func() { wg.Wait(); close(winner) }()
		if t, ok := <-winner; ok {
			cancel()
			g.switchTo(t, reason)
			return true
		}
		cancel()
	}

	// 3. Maybe the current one came back meanwhile.
	if _, err := g.probeOne(ctx, cur); err == nil {
		return true
	}
	return false
}

// Run blocks until ctx is done.
func (g *Guard) Run(ctx context.Context) {
	if len(g.tags) == 0 {
		<-ctx.Done()
		return
	}
	// Startup: check the remembered server, warm everything else in the background.
	go g.ProbeAll(ctx)
	g.checkCurrent(ctx)

	g.mu.Lock()
	cfg := g.cfg
	g.mu.Unlock()
	active := time.NewTicker(cfg.ActiveInterval)
	standby := time.NewTicker(cfg.StandbyInterval)
	sweep := time.NewTicker(cfg.SweepInterval)
	defer active.Stop()
	defer standby.Stop()
	defer sweep.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-g.kick:
			g.mu.Lock()
			for _, h := range g.h {
				h.cooldown = time.Time{} // new network: give everyone a fresh chance
			}
			g.backoff, g.nextRescue = 0, time.Time{}
			g.mu.Unlock()
			g.checkCurrent(ctx)
			go g.ProbeAll(ctx)
		case <-active.C:
			g.checkCurrent(ctx)
		case <-standby.C:
			g.probeStandbys(ctx)
		case <-sweep.C:
			g.ProbeAll(ctx)
			g.maybeUpgrade()
		}
	}
}

// checkCurrent probes the active server and fails over when it is down.
func (g *Guard) checkCurrent(ctx context.Context) {
	cur := g.Current()
	if _, err := g.probeOne(ctx, cur); err == nil {
		g.setOffline(false)
		g.mu.Lock()
		g.backoff, g.nextRescue = 0, time.Time{}
		g.mu.Unlock()
		g.maybeReturnToPreferred(ctx)
		return
	} else if ctx.Err() != nil {
		return
	}
	// Confirm before acting: one lost probe is common on mobile networks.
	select {
	case <-time.After(g.cfg.ConfirmDelay):
	case <-ctx.Done():
		return
	}
	g.mu.Lock()
	fails := g.h[cur].fails
	auto := g.cfg.AutoSwitch
	g.mu.Unlock()
	if fails < g.cfg.FailThreshold {
		if _, err := g.probeOne(ctx, cur); err == nil {
			return
		}
	}
	// Is it us or them?
	if g.direct != nil && !g.direct(ctx) {
		g.setOffline(true)
		return
	}
	g.setOffline(false)
	g.mu.Lock()
	wait := g.now().Before(g.nextRescue)
	g.mu.Unlock()
	if wait {
		return
	}
	if !auto {
		g.emit(Event{Kind: "down", From: cur, Reason: "server not responding (auto-switch is off)"})
		return
	}
	if g.failover(ctx, "server stopped responding") {
		g.mu.Lock()
		g.backoff, g.nextRescue = 0, time.Time{}
		g.mu.Unlock()
		return
	}
	// Nothing answered: retry the full rescue with a growing pause instead of every tick.
	g.mu.Lock()
	g.backoff = min(max(g.backoff*2, g.cfg.OfflineRetry), time.Minute)
	g.nextRescue = g.now().Add(g.backoff)
	g.mu.Unlock()
	g.emit(Event{Kind: "down", From: cur, Reason: "no server is responding"})
}

func (g *Guard) setOffline(off bool) {
	g.mu.Lock()
	changed := g.offline != off
	g.offline = off
	g.mu.Unlock()
	if !changed {
		return
	}
	if off {
		g.emit(Event{Kind: "offline", Reason: "the local network has no internet"})
	} else {
		g.emit(Event{Kind: "online", Reason: "internet is back"})
	}
}

func (g *Guard) probeStandbys(ctx context.Context) {
	cur := g.Current()
	g.mu.Lock()
	pref := g.preferred
	g.mu.Unlock()
	list := g.ranked(cur, false)
	if len(list) > g.cfg.Standbys {
		list = list[:g.cfg.Standbys]
	}
	if pref != "" && pref != cur {
		list = append(list, pref)
	}
	var wg sync.WaitGroup
	for _, t := range list {
		wg.Add(1)
		go func() { defer wg.Done(); g.probeOne(ctx, t) }()
	}
	wg.Wait()
}

// maybeReturnToPreferred moves back to the user's own pick once it has been healthy for a while.
func (g *Guard) maybeReturnToPreferred(ctx context.Context) {
	g.mu.Lock()
	pref, cur := g.preferred, g.current
	ok := pref != "" && pref != cur && g.cfg.AutoSwitch && g.h[pref].okStreak >= 3 &&
		g.now().Sub(g.lastSwitch) > g.cfg.UpgradeMinGap/2
	g.mu.Unlock()
	if ok {
		g.switchTo(pref, "back to your chosen server")
	}
}

// maybeUpgrade moves to a much better server; never away from the user's pick.
func (g *Guard) maybeUpgrade() {
	g.mu.Lock()
	cur := g.current
	if !g.cfg.AutoSwitch || !g.cfg.PreferFaster || cur == g.preferred && cur != "" ||
		g.now().Sub(g.lastSwitch) < g.cfg.UpgradeMinGap {
		g.mu.Unlock()
		return
	}
	ch := g.h[cur]
	curScore := ch.score()
	g.mu.Unlock()
	best := g.ranked(cur, false)
	if len(best) == 0 {
		return
	}
	g.mu.Lock()
	bh := g.h[best[0]]
	good := bh.n >= 3 && bh.rate() == 1 && bh.okStreak >= 3 &&
		bh.score() < g.cfg.UpgradeRatio*curScore && curScore-bh.score() > 150
	g.mu.Unlock()
	if good {
		g.switchTo(best[0], "a much faster server is available")
	}
}
