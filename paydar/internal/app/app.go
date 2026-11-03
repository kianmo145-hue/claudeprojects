// Package app is everything the UI talks to: servers, subscriptions, settings, connect and status.
// The desktop and Android front ends both drive it through the HTTP API (package api).
package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"paydar/internal/build"
	"paydar/internal/engine"
	"paydar/internal/guard"
	"paydar/internal/link"
	"paydar/internal/logx"
	"paydar/internal/store"
)

// Hooks are what the platform provides.
type Hooks struct {
	// Mode returns how to connect; Android always uses build.ModeLocal.
	Mode func(s store.Settings) build.Mode
	// DirectDNS returns the local network's DNS servers ("local" or nil = system resolver).
	DirectDNS func() []string
	// Changed is called after every state change (status, current server), for tray/notification.
	Changed func(Status)
}

type State string

const (
	Disconnected State = "disconnected"
	Connecting   State = "connecting"
	Connected    State = "connected"
)

type Status struct {
	State      State         `json:"state"`
	Mode       string        `json:"mode,omitempty"`
	Current    string        `json:"current,omitempty"` // server id
	ServerName string        `json:"serverName,omitempty"`
	Since      int64         `json:"since,omitempty"`
	Offline    bool          `json:"offline"`
	Error      string        `json:"error,omitempty"`
	Events     []guard.Event `json:"events,omitempty"`
	Testing    bool          `json:"testing"`
	MixedPort  int           `json:"mixedPort,omitempty"`
}

type ServerView struct {
	store.Server
	Health  *guard.Health `json:"health,omitempty"`
	Current bool          `json:"current"`
	Chosen  bool          `json:"chosen"`
	SubName string        `json:"subName,omitempty"`
}

type App struct {
	hooks Hooks
	st    *store.Store
	Logs  *logx.Ring

	mu      sync.Mutex
	s       store.State
	eng     *engine.Engine
	state   State
	err     string
	mode    build.Mode
	testing bool
	tested  map[string]guard.Health
	// opMu serialises connect/disconnect/restart.
	opMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
}

func New(dataDir string, hooks Hooks) (*App, error) {
	st, s, err := store.Open(dataDir)
	if err != nil {
		return nil, err
	}
	if hooks.Mode == nil {
		hooks.Mode = func(s store.Settings) build.Mode {
			if s.Mode == "proxy" {
				return build.ModeSystemProxy
			}
			return build.ModeTUN
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{hooks: hooks, st: st, s: s, Logs: logx.NewRing(400), state: Disconnected, tested: map[string]guard.Health{}, ctx: ctx, cancel: cancel}
	go a.subLoop()
	return a, nil
}

func (a *App) save() {
	if err := a.st.Save(a.s); err != nil {
		a.Logs.Errorf("save: %v", err)
	}
}

func (a *App) changed() {
	if a.hooks.Changed != nil {
		a.hooks.Changed(a.Status())
	}
}

// ---- servers ----

func profileOf(s store.Server) (*link.Profile, error) {
	p, err := link.Restore(s.Link)
	if err != nil {
		return nil, err
	}
	p.ID, p.Name = s.ID, s.Name
	return p, nil
}

func (a *App) profiles() []*link.Profile {
	var out []*link.Profile
	for _, s := range a.s.Servers {
		p, err := profileOf(s)
		if err != nil {
			a.Logs.Warnf("skip %s: %v", s.Name, err)
			continue
		}
		out = append(out, p)
	}
	return out
}

func (a *App) Servers() []ServerView {
	a.mu.Lock()
	defer a.mu.Unlock()
	live := map[string]guard.Health{}
	cur := ""
	if a.eng != nil {
		for _, h := range a.eng.Guard().Snapshot() {
			if p := a.eng.Profile(h.Tag); p != nil {
				live[p.ID] = h
			}
		}
		if p := a.eng.Profile(a.eng.Guard().Current()); p != nil {
			cur = p.ID
		}
	}
	subNames := map[string]string{}
	for _, s := range a.s.Subs {
		subNames[s.ID] = s.Name
	}
	out := make([]ServerView, 0, len(a.s.Servers))
	for _, s := range a.s.Servers {
		v := ServerView{Server: s, Current: s.ID == cur, Chosen: s.ID == a.s.Selected, SubName: subNames[s.Sub]}
		if h, ok := live[s.ID]; ok && h.Samples > 0 {
			v.Health = &h
		} else if h, ok := a.tested[s.ID]; ok {
			v.Health = &h
		}
		v.Link = "" // links hold credentials; the UI does not need them
		out = append(out, v)
	}
	return out
}

// ServerLink returns one server's share link (for "copy link").
func (a *App) ServerLink(id string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, s := range a.s.Servers {
		if s.ID == id {
			return s.Link, nil
		}
	}
	return "", errors.New("not found")
}

type AddResult struct {
	Added   int      `json:"added"`
	Skipped []string `json:"skipped,omitempty"`
	Sub     bool     `json:"sub"`
}

// Add takes pasted text: share links, base64 lists, sing-box JSON, or a subscription URL.
func (a *App) Add(text string) (AddResult, error) {
	text = strings.TrimSpace(text)
	if u, err := url.Parse(text); err == nil && (u.Scheme == "http" || u.Scheme == "https") && !strings.ContainsAny(text, " \n") {
		n, err := a.AddSub(text, "")
		return AddResult{Added: n, Sub: true}, err
	}
	ps, skipped := link.ParseText(text)
	if len(ps) == 0 {
		if len(skipped) > 0 {
			return AddResult{Skipped: skipped}, fmt.Errorf("no supported config found: %s", skipped[0])
		}
		return AddResult{}, errors.New("no config found in the text")
	}
	a.mu.Lock()
	have := map[string]bool{}
	for _, s := range a.s.Servers {
		have[s.ID] = true
	}
	added := 0
	for _, p := range ps {
		if have[p.ID] {
			continue
		}
		a.s.Servers = append(a.s.Servers, serverOf(p, ""))
		added++
	}
	a.save()
	a.mu.Unlock()
	if added > 0 {
		a.restartIfConnected("servers added")
	}
	return AddResult{Added: added, Skipped: skipped}, nil
}

func serverOf(p *link.Profile, sub string) store.Server {
	return store.Server{ID: p.ID, Name: p.Name, Type: p.Type, Server: p.Server, Port: p.Port, Link: p.Link, Sub: sub}
}

func (a *App) Remove(ids []string) {
	del := map[string]bool{}
	for _, id := range ids {
		del[id] = true
	}
	a.mu.Lock()
	kept := a.s.Servers[:0]
	for _, s := range a.s.Servers {
		if !del[s.ID] {
			kept = append(kept, s)
		}
	}
	a.s.Servers = kept
	a.save()
	a.mu.Unlock()
	a.restartIfConnected("servers removed")
}

// RemoveDead deletes servers that failed their last test.
func (a *App) RemoveDead() int {
	var ids []string
	for _, v := range a.Servers() {
		if v.Health != nil && v.Health.Samples > 0 && v.Health.Success == 0 {
			ids = append(ids, v.ID)
		}
	}
	if len(ids) > 0 {
		a.Remove(ids)
	}
	return len(ids)
}

// ---- subscriptions ----

func (a *App) Subs() []store.Subscription {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]store.Subscription(nil), a.s.Subs...)
}

func (a *App) AddSub(rawURL, name string) (int, error) {
	id := subID(rawURL)
	a.mu.Lock()
	exists := false
	for _, s := range a.s.Subs {
		if s.ID == id {
			exists = true
		}
	}
	if !exists {
		a.s.Subs = append(a.s.Subs, store.Subscription{ID: id, Name: name, URL: strings.TrimSpace(rawURL)})
		a.save()
	}
	a.mu.Unlock()
	return a.UpdateSub(id)
}

func (a *App) RemoveSub(id string) {
	a.mu.Lock()
	subs := a.s.Subs[:0]
	for _, s := range a.s.Subs {
		if s.ID != id {
			subs = append(subs, s)
		}
	}
	a.s.Subs = subs
	servers := a.s.Servers[:0]
	for _, s := range a.s.Servers {
		if s.Sub != id {
			servers = append(servers, s)
		}
	}
	a.s.Servers = servers
	a.save()
	a.mu.Unlock()
	a.restartIfConnected("subscription removed")
}

// UpdateSub downloads one subscription and replaces its servers. A failed download keeps the old
// list, so a filtered subscription host never leaves the user with nothing.
func (a *App) UpdateSub(id string) (int, error) {
	a.mu.Lock()
	var sub *store.Subscription
	for i := range a.s.Subs {
		if a.s.Subs[i].ID == id {
			sub = &a.s.Subs[i]
		}
	}
	if sub == nil {
		a.mu.Unlock()
		return 0, errors.New("unknown subscription")
	}
	u := sub.URL
	port := 0
	if a.eng != nil {
		port = a.s.Settings.MixedPort
	}
	a.mu.Unlock()

	ctx, cancel := context.WithTimeout(a.ctx, 50*time.Second)
	defer cancel()
	f, err := fetchSub(ctx, u, port)

	a.mu.Lock()
	sub = nil
	for i := range a.s.Subs {
		if a.s.Subs[i].ID == id {
			sub = &a.s.Subs[i]
		}
	}
	if sub == nil {
		a.mu.Unlock()
		return 0, errors.New("subscription removed")
	}
	sub.UpdatedAt = time.Now().UnixMilli()
	if err != nil {
		sub.LastError = err.Error()
		a.save()
		a.mu.Unlock()
		a.Logs.Warnf("subscription %s: %v", sub.Name, err)
		return 0, err
	}
	sub.LastError = ""
	if sub.Name == "" {
		sub.Name = f.title
	}
	if sub.Name == "" {
		if pu, err := url.Parse(u); err == nil {
			sub.Name = pu.Hostname()
		}
	}
	sub.Upload, sub.Download, sub.Total, sub.Expire = f.info.Upload, f.info.Download, f.info.Total, f.info.Expire
	if f.info.IntervalHours > 0 {
		sub.IntervalHours = f.info.IntervalHours
	}
	sub.Count = len(f.profiles)
	old := map[string]store.Server{}
	var others []store.Server
	for _, s := range a.s.Servers {
		if s.Sub == id {
			old[s.ID] = s
		} else {
			others = append(others, s)
		}
	}
	changed := len(old) != len(f.profiles)
	var fresh []store.Server
	for _, p := range f.profiles {
		s := serverOf(p, id)
		if o, ok := old[p.ID]; ok {
			s.LastRTT, s.LastTested, s.LastOK = o.LastRTT, o.LastTested, o.LastOK
		} else {
			changed = true
		}
		fresh = append(fresh, s)
	}
	a.s.Servers = append(others, fresh...)
	a.save()
	a.mu.Unlock()
	if changed {
		a.restartIfConnected("subscription updated")
	}
	return len(f.profiles), nil
}

func (a *App) UpdateAllSubs() {
	for _, s := range a.Subs() {
		a.UpdateSub(s.ID)
	}
}

func (a *App) subLoop() {
	t := time.NewTicker(20 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
		}
		a.mu.Lock()
		hours := a.s.Settings.SubRefreshHours
		var due []string
		for _, s := range a.s.Subs {
			h := hours
			if h == 0 {
				h = s.IntervalHours
			}
			if h == 0 {
				h = 12
			}
			if time.Since(time.UnixMilli(s.UpdatedAt)) > time.Duration(h)*time.Hour {
				due = append(due, s.ID)
			}
		}
		a.mu.Unlock()
		for _, id := range due {
			a.UpdateSub(id)
		}
	}
}

// ---- settings ----

func (a *App) Settings() store.Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.s.Settings
}

func (a *App) SetSettings(s store.Settings) {
	a.mu.Lock()
	old := a.s.Settings
	if s.MixedPort <= 0 || s.MixedPort > 65535 {
		s.MixedPort = old.MixedPort
	}
	a.s.Settings = s
	a.save()
	eng := a.eng
	a.mu.Unlock()
	needRestart := old.Mode != s.Mode || old.BypassIran != s.BypassIran || old.IPv6 != s.IPv6 ||
		old.RemoteDNS != s.RemoteDNS || old.MixedPort != s.MixedPort
	if needRestart {
		a.restartIfConnected("settings changed")
	} else if eng != nil {
		eng.Guard().SetConfig(a.guardConfig())
	}
}

func (a *App) guardConfig() guard.Config {
	c := guard.DefaultConfig()
	a.mu.Lock()
	c.AutoSwitch = a.s.Settings.AutoSwitch
	c.PreferFaster = a.s.Settings.PreferFaster
	a.mu.Unlock()
	return c
}

// ---- connection ----

func (a *App) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := Status{State: a.state, Error: a.err, Testing: a.testing, Mode: string(a.mode)}
	if a.eng != nil {
		g := a.eng.Guard()
		st.Since = a.eng.Since().UnixMilli()
		st.Offline = g.Offline()
		st.Events = g.Events()
		st.MixedPort = a.s.Settings.MixedPort
		if p := a.eng.Profile(g.Current()); p != nil {
			st.Current, st.ServerName = p.ID, p.Name
		}
	}
	return st
}

// Connect starts the tunnel with every server; the guard picks and keeps a working one.
func (a *App) Connect() error {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	return a.connectLocked()
}

func (a *App) connectLocked() error {
	a.mu.Lock()
	if a.eng != nil {
		a.mu.Unlock()
		return nil
	}
	ps := a.profiles()
	if len(ps) == 0 {
		a.mu.Unlock()
		return errors.New("add a server or subscription first")
	}
	// Start on the user's pick, else the last server that worked, else the best tested one.
	start := a.s.Selected
	if start == "" {
		start = a.s.LastGood
	}
	if start == "" {
		start = a.bestTestedLocked()
	}
	mode := a.hooks.Mode(a.s.Settings)
	opts := build.Options{
		Mode: mode, MixedPort: a.s.Settings.MixedPort, RemoteDNS: a.s.Settings.RemoteDNS,
		BypassIran: a.s.Settings.BypassIran, IPv6: a.s.Settings.IPv6,
	}
	for _, p := range ps {
		if p.ID == start {
			opts.Selected = build.Tag(p)
		}
	}
	a.state, a.err, a.mode = Connecting, "", mode
	a.mu.Unlock()
	a.changed()

	if a.hooks.DirectDNS != nil {
		opts.DirectDNS = a.hooks.DirectDNS()
	}
	eng, err := engine.Start(a.ctx, ps, opts, a.guardConfig(), a.Logs, a.onEvent)
	a.mu.Lock()
	if err != nil {
		a.state, a.err = Disconnected, err.Error()
		a.mu.Unlock()
		a.Logs.Errorf("connect: %v", err)
		a.changed()
		return err
	}
	a.eng, a.state = eng, Connected
	a.mu.Unlock()
	a.Logs.Infof("connected (%s), %d servers", mode, len(ps))
	a.changed()
	return nil
}

func (a *App) bestTestedLocked() string {
	best, bestRTT := "", 1<<30
	for _, s := range a.s.Servers {
		if s.LastOK && s.LastRTT > 0 && s.LastRTT < bestRTT {
			best, bestRTT = s.ID, s.LastRTT
		}
	}
	return best
}

func (a *App) onEvent(e guard.Event) {
	a.mu.Lock()
	if a.eng != nil && e.Kind == "switch" {
		if p := a.eng.Profile(e.To); p != nil {
			a.s.LastGood = p.ID
			a.save()
		}
	}
	name := func(tag string) string {
		if a.eng != nil {
			if p := a.eng.Profile(tag); p != nil {
				return p.Name
			}
		}
		return tag
	}
	from, to := name(e.From), name(e.To)
	a.mu.Unlock()
	switch e.Kind {
	case "switch":
		a.Logs.Infof("switched %s → %s (%s)", from, to, e.Reason)
	default:
		a.Logs.Warnf("%s: %s %s", e.Kind, from, e.Reason)
	}
	a.changed()
}

func (a *App) Disconnect() {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.disconnectLocked()
}

func (a *App) disconnectLocked() {
	a.mu.Lock()
	eng := a.eng
	a.eng = nil
	if eng != nil {
		a.rememberHealthLocked(eng)
	}
	a.state, a.err = Disconnected, ""
	a.mu.Unlock()
	if eng != nil {
		eng.Close()
		a.Logs.Infof("disconnected")
	}
	a.changed()
}

// rememberHealthLocked copies live results into the saved list and the test cache.
func (a *App) rememberHealthLocked(eng *engine.Engine) {
	for _, h := range eng.Guard().Snapshot() {
		p := eng.Profile(h.Tag)
		if p == nil || h.Samples == 0 {
			continue
		}
		a.tested[p.ID] = h
		for i := range a.s.Servers {
			if a.s.Servers[i].ID == p.ID {
				a.s.Servers[i].LastRTT, a.s.Servers[i].LastOK = h.RTT, h.Fails == 0
				a.s.Servers[i].LastTested = time.Now().UnixMilli()
			}
		}
	}
	a.save()
}

// Reconnect restarts a running tunnel (for example after the local network changed its DNS).
func (a *App) Reconnect(why string) { a.restartIfConnected(why) }

func (a *App) restartIfConnected(why string) {
	a.mu.Lock()
	running := a.eng != nil
	a.mu.Unlock()
	if !running {
		return
	}
	go func() {
		a.opMu.Lock()
		defer a.opMu.Unlock()
		a.Logs.Infof("reconnecting: %s", why)
		a.disconnectLocked()
		a.connectLocked()
	}()
}

// Choose makes id the user's pick (and moves traffic to it when connected).
func (a *App) Choose(id string) error {
	a.mu.Lock()
	a.s.Selected = id
	a.save()
	eng := a.eng
	a.mu.Unlock()
	if eng == nil {
		a.changed()
		return nil
	}
	for _, h := range eng.Guard().Snapshot() {
		if p := eng.Profile(h.Tag); p != nil && p.ID == id {
			return eng.Guard().Choose(h.Tag)
		}
	}
	return errors.New("server not in the running set")
}

// ClearChoice lets the guard pick freely again.
func (a *App) ClearChoice() {
	a.mu.Lock()
	a.s.Selected = ""
	a.save()
	a.mu.Unlock()
	a.changed()
}

// TestAll measures every server (through the running tunnel's outbounds, or a temporary one).
func (a *App) TestAll() error {
	a.mu.Lock()
	if a.testing {
		a.mu.Unlock()
		return nil
	}
	a.testing = true
	eng := a.eng
	ps := a.profiles()
	a.mu.Unlock()
	a.changed()
	defer func() {
		a.mu.Lock()
		a.testing = false
		a.mu.Unlock()
		a.changed()
	}()
	ctx, cancel := context.WithTimeout(a.ctx, 2*time.Minute)
	defer cancel()
	if eng != nil {
		eng.Guard().ProbeAll(ctx)
		a.mu.Lock()
		a.rememberHealthLocked(eng)
		a.mu.Unlock()
		return nil
	}
	if len(ps) == 0 {
		return nil
	}
	var dns []string
	if a.hooks.DirectDNS != nil {
		dns = a.hooks.DirectDNS()
	}
	res, err := engine.Test(ctx, ps, dns, a.Logs)
	if err != nil {
		return err
	}
	a.mu.Lock()
	now := time.Now().UnixMilli()
	for i := range a.s.Servers {
		if h, ok := res[a.s.Servers[i].ID]; ok {
			a.tested[a.s.Servers[i].ID] = h
			a.s.Servers[i].LastRTT, a.s.Servers[i].LastOK, a.s.Servers[i].LastTested = h.RTT, h.Fails == 0, now
		}
	}
	// Fastest first, so the list reads as a ranking.
	sort.SliceStable(a.s.Servers, func(i, j int) bool {
		si, sj := a.s.Servers[i], a.s.Servers[j]
		if si.LastOK != sj.LastOK {
			return si.LastOK
		}
		return si.LastOK && si.LastRTT < sj.LastRTT
	})
	a.save()
	a.mu.Unlock()
	return nil
}

// NetworkChanged: Wi-Fi/mobile switch or resume; check right away instead of waiting.
func (a *App) NetworkChanged() {
	a.mu.Lock()
	eng := a.eng
	a.mu.Unlock()
	if eng != nil {
		eng.Guard().Kick()
	}
}

// AutoConnect is called at start-up.
func (a *App) AutoConnect() {
	if a.Settings().AutoConnect {
		go a.Connect()
	}
}

func (a *App) Close() {
	a.Disconnect()
	a.cancel()
}
