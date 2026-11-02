// Package tray is the notification-area app: a coloured badge for the state (grey paused, amber
// one connection, green merging, with a phone badge while a booster phone is in use), live speeds
// on hover, and a menu with every link, the booster phones and the settings, in Persian or English.
package tray

import (
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf16"

	"fyne.io/systray"

	"peyvand/internal/booster"
	"peyvand/internal/engine"
	. "peyvand/internal/i18n"
	"peyvand/internal/icon"
	"peyvand/internal/logx"
	"peyvand/internal/platform"
	"peyvand/internal/state"
	"peyvand/internal/tun"
)

// ProjectURL is opened by "About"; HelpURL explains booster mode.
const (
	ProjectURL = "https://github.com/kianmo145-hue/claudeprojects/tree/main/peyvand"
	HelpURL    = ProjectURL + "/docs/BOOSTER.md"
)

const (
	trayRows  = 6 // links shown in the menu; extra rows stay hidden
	phoneRows = 4 // phones shown in the menu
)

// Run shows the tray icon and blocks until Quit, and closes ready once the icon exists. With
// startOn it merges as soon as it can (at once, or, with autoPause, once two connections work).
// boost may be nil (booster mode unavailable).
func Run(t *tun.Tun, sched *engine.Scheduler, bal *engine.Balancer, boost *booster.Manager, stateDir, version string, startOn, autoPause bool, ready chan struct{}) {
	ui := &trayUI{tun: t, sched: sched, bal: bal, boost: boost, stateDir: stateDir, version: version, startOn: startOn, ready: ready}
	ui.ap.enabled = autoPause
	exit := func() { t.Close(); state.Clear(stateDir) }
	systray.Run(ui.onReady, exit)
	exit()
}

// event is a click, funnelled into the menu goroutine so all state changes happen in one place.
type event struct {
	kind  string // "limit", "pair", "use", "forget"
	row   int
	value float64
}

type phoneRow struct {
	item, pair, use, forget *systray.MenuItem
	id, name                string
	paired                  bool
	last                    string
}

type linkRow struct {
	item   *systray.MenuItem
	unlim  *systray.MenuItem
	preset []*systray.MenuItem
	name   string
	last   string
	dot    icon.Dot
}

type trayUI struct {
	tun      *tun.Tun
	bal      *engine.Balancer
	sched    *engine.Scheduler
	boost    *booster.Manager
	stateDir string
	version  string
	startOn  bool
	ready    chan struct{}

	ap      autoPauser
	events  chan event
	pairing atomic.Bool

	header, total, toggleItem                     *systray.MenuItem
	phonesHeader, phonesNone, phonesHelp          *systray.MenuItem
	settings, v6Item, autoItem, signInItem        *systray.MenuItem
	langItem, langFA, langEN, aboutItem, quitItem *systray.MenuItem

	links  [trayRows]*linkRow
	phones [phoneRows]*phoneRow

	lastIcon   string
	lastOn     *bool
	lastHeader string
	lastTotal  string
	lastTip    string
	lastNone   *bool
	lastErr    string
}

func (u *trayUI) onReady() {
	if u.ready != nil {
		close(u.ready)
	}
	u.events = make(chan event, 16)
	systray.SetIcon(icon.Tray(icon.Off, false))
	systray.SetTitle("Peyvand")
	systray.SetTooltip(T("app"))

	u.header = systray.AddMenuItem("", "")
	u.header.Disable()
	u.total = systray.AddMenuItem("", "")
	u.total.Disable()
	systray.AddSeparator()

	for i := range u.links {
		r := &linkRow{item: systray.AddMenuItem("", "")}
		add := func(mbit float64, title string) *systray.MenuItem {
			it := r.item.AddSubMenuItemCheckbox(title, "", false)
			go func() {
				for range it.ClickedCh {
					u.events <- event{kind: "limit", row: i, value: mbit}
				}
			}()
			return it
		}
		r.unlim = add(0, "")
		for _, m := range limitPresets {
			r.preset = append(r.preset, add(m, ""))
		}
		// Add the children while the row is still in the menu: on Windows a hidden item is removed
		// from the menu, and turning it into a submenu afterwards fails.
		r.item.Hide()
		r.dot = -1
		u.links[i] = r
	}
	systray.AddSeparator()

	u.phonesHeader = systray.AddMenuItem("", "")
	u.phonesHeader.Disable()
	for i := range u.phones {
		p := &phoneRow{item: systray.AddMenuItem("", "")}
		p.pair = p.item.AddSubMenuItem("", "")
		p.use = p.item.AddSubMenuItemCheckbox("", "", false)
		p.forget = p.item.AddSubMenuItem("", "")
		for kind, it := range map[string]*systray.MenuItem{"pair": p.pair, "use": p.use, "forget": p.forget} {
			go func() {
				for range it.ClickedCh {
					u.events <- event{kind: kind, row: i}
				}
			}()
		}
		p.item.Hide()
		u.phones[i] = p
	}
	u.phonesNone = systray.AddMenuItem("", "")
	u.phonesNone.Disable()
	u.phonesHelp = systray.AddMenuItem("", "")
	if u.boost == nil {
		u.phonesHeader.Hide()
		u.phonesNone.Hide()
		u.phonesHelp.Hide()
	}
	systray.AddSeparator()

	u.toggleItem = systray.AddMenuItem("", "")
	u.settings = systray.AddMenuItem("", "")
	u.v6Item = u.settings.AddSubMenuItemCheckbox("", "", u.bal.V6Mode() == engine.V6Auto)
	u.autoItem = u.settings.AddSubMenuItemCheckbox("", "", u.ap.enabled)
	u.signInItem = u.settings.AddSubMenuItemCheckbox("", "", platform.AutostartEnabled())
	u.langItem = u.settings.AddSubMenuItem("", "")
	u.langFA = u.langItem.AddSubMenuItemCheckbox("فارسی", "", Current() == FA)
	u.langEN = u.langItem.AddSubMenuItemCheckbox("English", "", Current() == EN)
	u.aboutItem = systray.AddMenuItem("", "")
	u.quitItem = systray.AddMenuItem("", "")
	u.applyTexts()

	go u.loop()
}

// applyTexts sets every fixed title in the current language and makes refresh redraw the rest.
func (u *trayUI) applyTexts() {
	for _, r := range u.links {
		r.item.SetTooltip(T("limit.tip"))
		r.unlim.SetTitle(T("limit.none"))
		for i, it := range r.preset {
			it.SetTitle(Local(limitText(limitPresets[i])))
		}
		r.last = ""
	}
	u.phonesHeader.SetTitle(T("phones"))
	u.phonesNone.SetTitle(T("phones.none"))
	u.phonesHelp.SetTitle(T("phone.help"))
	for _, p := range u.phones {
		p.pair.SetTitle(T("phone.pair"))
		p.use.SetTitle(T("phone.use"))
		p.forget.SetTitle(T("phone.forget"))
		p.last = ""
	}
	u.settings.SetTitle(T("settings"))
	u.v6Item.SetTitle(T("v6"))
	u.v6Item.SetTooltip(T("v6.tip"))
	u.autoItem.SetTitle(T("autopause"))
	u.signInItem.SetTitle(T("signin"))
	u.langItem.SetTitle(T("lang"))
	u.aboutItem.SetTitle(T("about", versionLabel(u.version)))
	u.quitItem.SetTitle(T("quit"))
	u.lastOn, u.lastHeader, u.lastTotal, u.lastTip, u.lastNone = nil, "", "", "", nil
}

func (u *trayUI) loop() {
	u.ap.start(u.startOn)
	switch {
	case !u.startOn:
	case u.ap.enabled:
		if err := u.tun.Warm(); err != nil {
			logx.Printf("warning: %v\n", err)
		}
	default:
		u.turn(true)
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		u.refresh()
		select {
		case <-u.toggleItem.ClickedCh:
			u.manual(!u.tun.IsOn())
		case <-u.autoItem.ClickedCh:
			toggle(u.autoItem)
			u.ap.setEnabled(u.autoItem.Checked(), u.tun.IsOn())
		case <-u.signInItem.ClickedCh:
			on := !u.signInItem.Checked()
			u.lastErr = ""
			if err := platform.SetAutostart(on); err != nil {
				u.lastErr = T("err.signin", err.Error())
				logx.Printf("error: %s\n", u.lastErr)
			} else {
				setCheck(u.signInItem, on)
			}
		case <-u.v6Item.ClickedCh:
			toggle(u.v6Item)
			if u.v6Item.Checked() {
				u.bal.SetV6Mode(engine.V6Auto)
			} else {
				u.bal.SetV6Mode(engine.V6On)
			}
		case <-u.langFA.ClickedCh:
			u.setLang(FA)
		case <-u.langEN.ClickedCh:
			u.setLang(EN)
		case <-u.phonesHelp.ClickedCh:
			u.open(HelpURL)
		case <-u.aboutItem.ClickedCh:
			u.open(ProjectURL)
		case e := <-u.events:
			u.handle(e)
		case <-u.quitItem.ClickedCh:
			systray.Quit()
			return
		case <-tick.C:
			if req, ok := state.TakeLimit(u.stateDir); ok { // `peyvand limit ...` from a terminal
				u.applyLimit(req.Adapter, req.Mbps)
			}
			switch { // `peyvand on|off|quit` from a terminal
			case state.TakeMarker(u.stateDir, "quit"):
				systray.Quit()
				return
			case state.TakeMarker(u.stateDir, "off"):
				u.manual(false)
			case state.TakeMarker(u.stateDir, "on"):
				u.manual(true)
			}
			switch u.ap.tick(time.Now(), u.tun.IsOn(), u.usable(), u.movingFlows()) {
			case autoPause:
				u.turn(false)
			case autoResume:
				if u.turn(true); !u.tun.IsOn() {
					u.ap.resumeFailed()
				}
			}
		}
	}
}

func (u *trayUI) open(url string) {
	if err := platform.OpenURL(url); err != nil {
		logx.Printf("error: opening %s: %v\n", url, err)
	}
}

func (u *trayUI) setLang(l Lang) {
	Set(l)
	setCheck(u.langFA, l == FA)
	setCheck(u.langEN, l == EN)
	s := state.LoadSettings(u.stateDir)
	s.Lang = string(l)
	if err := state.SaveSettings(u.stateDir, s); err != nil {
		logx.Printf("warning: saving settings: %v\n", err)
	}
	u.applyTexts()
}

func (u *trayUI) handle(e event) {
	switch e.kind {
	case "limit":
		u.applyLimit(u.links[e.row].name, e.value)
	case "pair":
		p := u.phones[e.row]
		if p.id == "" || p.paired || !u.pairing.CompareAndSwap(false, true) {
			return
		}
		id, name := p.id, p.name
		go func() { // the dialogs block; the menu keeps working meanwhile
			defer u.pairing.Store(false)
			err := u.boost.Pair(id, func(code, phoneName string) bool {
				return platform.Confirm(T("pair.title", phoneName), T("pair.body", code[:3]+" "+code[3:]), RTL())
			})
			switch {
			case err == nil:
				platform.Info(T("app"), T("pair.ok", name), RTL())
			case strings.Contains(err.Error(), "declined"):
				platform.Info(T("app"), T("pair.declined"), RTL())
			case strings.Contains(err.Error(), "cancelled"):
			default:
				platform.Info(T("app"), T("pair.fail", err.Error()), RTL())
			}
		}()
	case "use":
		p := u.phones[e.row]
		if p.paired {
			if err := u.boost.SetEnabled(p.id, !p.use.Checked()); err != nil {
				logx.Printf("error: %v\n", err)
			}
		}
	case "forget":
		p := u.phones[e.row]
		if p.paired {
			if err := u.boost.Forget(p.id); err != nil {
				logx.Printf("error: %v\n", err)
			}
		}
	}
}

// manual is a switch by the user (menu or `peyvand on|off`), which pausing by itself must respect.
func (u *trayUI) manual(on bool) {
	u.ap.userSet(on, u.usable())
	u.turn(on)
}

// usable is how many connections reach the internet right now.
func (u *trayUI) usable() (n int) {
	for _, s := range u.sched.Snapshot() {
		if s.Present && (s.State == "up" || s.State6 == "up") {
			n++
		}
	}
	return n
}

// movingFlows is how many connections are relayed while data is really moving.
func (u *trayUI) movingFlows() (n int64) {
	var rate float64
	for _, s := range u.sched.Snapshot() {
		if s.Present {
			n += s.Active
			rate += s.RateIn + s.RateOut
		}
	}
	if rate < quietRate {
		return 0
	}
	return n
}

func (u *trayUI) turn(on bool) {
	u.lastErr = ""
	if !on {
		u.tun.Off()
		state.Clear(u.stateDir)
		return
	}
	u.setHeader(T("header", T("state.starting")))
	if err := u.tun.On(); err != nil {
		u.lastErr = err.Error()
		logx.Printf("error: %v\n", err)
	}
}

func (u *trayUI) setHeader(s string) {
	if s != u.lastHeader {
		u.header.SetTitle(s)
		u.lastHeader = s
	}
}

func (u *trayUI) refresh() {
	on := u.tun.IsOn()
	if u.lastOn == nil || *u.lastOn != on {
		if on {
			u.toggleItem.SetTitle("⏸  " + T("toggle.off"))
		} else {
			u.toggleItem.SetTitle("▶  " + T("toggle.on"))
		}
		u.lastOn = &on
	}

	var live []engine.SlotStats
	var down, up float64
	usable, phoneUp := 0, false
	for _, s := range u.sched.Snapshot() {
		if !s.Present {
			continue
		}
		live = append(live, s)
		down += s.LinkIn
		up += s.LinkOut
		if s.State == "up" || s.State6 == "up" {
			usable++
			phoneUp = phoneUp || s.Remote
		}
	}

	st := icon.Off
	switch {
	case on && usable >= 2:
		st = icon.Merging
	case on:
		st = icon.Single
	}
	if key := fmt.Sprint(st, on && phoneUp); key != u.lastIcon {
		systray.SetIcon(icon.Tray(st, on && phoneUp))
		u.lastIcon = key
	}

	word := stateWord(on)
	if !on && u.ap.enabled && u.ap.waiting && !u.ap.manualOff {
		word = T("state.single")
	}
	if u.lastErr != "" {
		u.setHeader(T("error", shorten(u.lastErr, 70)))
	} else {
		u.setHeader(T("header", word))
	}
	if t := T("total", mbps(down), mbps(up)); t != u.lastTotal {
		u.total.SetTitle(t)
		u.lastTotal = t
	}

	for i, r := range u.links {
		if i >= len(live) {
			if r.last != "" {
				r.item.Hide()
				r.last, r.name = "", ""
			}
			continue
		}
		s := live[i]
		r.name = s.Name
		txt := rowText(s, down, on)
		if txt != r.last {
			r.item.SetTitle(txt)
			r.item.Show()
			r.last = txt
		}
		if d := dotFor(s); d != r.dot {
			r.item.SetIcon(icon.MenuDot(d))
			r.dot = d
		}
		u.showLimit(i, s.LimitMbps)
	}

	if u.boost != nil {
		u.refreshPhones()
	}

	tip := trayTooltip(word, down, up, live)
	if tip != u.lastTip {
		systray.SetTooltip(tip)
		u.lastTip = tip
	}
}

func (u *trayUI) refreshPhones() {
	list := u.boost.Phones()
	none := len(list) == 0
	if u.lastNone == nil || *u.lastNone != none {
		if none {
			u.phonesNone.Show()
		} else {
			u.phonesNone.Hide()
		}
		u.lastNone = &none
	}
	for i, p := range u.phones {
		if i >= len(list) {
			if p.last != "" {
				p.item.Hide()
				p.last, p.id = "", ""
			}
			continue
		}
		ph := list[i]
		p.id, p.name, p.paired = ph.ID, ph.Name, ph.Paired
		var status string
		var dot icon.Dot
		switch {
		case !ph.Paired:
			status, dot = T("phone.new"), icon.DotPending
		case !ph.Online:
			status, dot = T("phone.offline"), icon.DotIdle
		case !ph.Cell:
			status, dot = T("phone.nodata"), icon.DotDown
		case !ph.Enabled:
			status, dot = T("phone.online"), icon.DotIdle
		default:
			status, dot = T("phone.online"), icon.DotUp
		}
		txt := fmt.Sprintf("📱 %s  ·  %s", ph.Name, status)
		key := fmt.Sprint(txt, ph.Paired, ph.Enabled, dot)
		if key == p.last {
			continue
		}
		p.item.SetTitle(txt)
		p.item.SetIcon(icon.MenuDot(dot))
		if ph.Paired {
			p.pair.Hide()
			p.use.Show()
			p.forget.Show()
			setCheck(p.use, ph.Enabled)
		} else {
			p.pair.Show()
			p.use.Hide()
			p.forget.Hide()
		}
		p.item.Show()
		p.last = key
	}
}

// rowText is one link's menu line: name, speeds, its share of the total as a bar, and flows.
func rowText(s engine.SlotStats, total float64, on bool) string {
	txt := T("row", s.Name, mbps(s.LinkIn), mbps(s.LinkOut), shareBar(s.LinkIn, total))
	if on {
		txt += T("row.flows", s.Active)
	}
	if s.LimitMbps > 0 {
		txt += T("row.limit", limitText(s.LimitMbps))
	}
	switch {
	case s.State == "up" || s.State6 == "up":
	case s.State == "no internet":
		txt += T("row.nonet")
	default:
		txt += T("row.checking")
	}
	if s.State6 == "up" {
		txt += T("row.v6")
	}
	return txt
}

// shareBar draws part/total as five segments, e.g. ▰▰▰▱▱.
func shareBar(part, total float64) string {
	n := 0
	if total > 0 {
		n = int(math.Round(part / total * 5))
	}
	n = max(0, min(5, n))
	return strings.Repeat("▰", n) + strings.Repeat("▱", 5-n)
}

func dotFor(s engine.SlotStats) icon.Dot {
	switch {
	case s.State == "up" || s.State6 == "up":
		return icon.DotUp
	case s.State == "no internet" && s.State6 != "unknown":
		return icon.DotDown
	}
	return icon.DotPending
}

func toggle(it *systray.MenuItem) { setCheck(it, !it.Checked()) }

func setCheck(it *systray.MenuItem, on bool) {
	if it.Checked() == on {
		return
	}
	if on {
		it.Check()
	} else {
		it.Uncheck()
	}
}

// Windows 11 shows only the first 64 characters of a tray tooltip, so the hover text drops detail
// until it fits.
const tooltipMax = 63

// stateWord is how the tray names the two states.
func stateWord(on bool) string {
	if on {
		return T("state.on")
	}
	return T("state.off")
}

// versionLabel is shown in "About", so a bug report has the version without a terminal.
func versionLabel(v string) string {
	if v = strings.TrimSpace(v); v == "" {
		v = "dev"
	}
	if v == "dev" || strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// trayTooltip lists each link's speed and a total line, e.g.
//
//	Merging
//	WiFi ↓121 ↑9.0
//	📱 Pixel ↓84 ↑3.1
//	Total ↓205 ↑12 Mbps
//
// It drops detail (the app name, then per-link upload, then the unit) until it fits; every link
// keeps its download speed and the total is always shown.
func trayTooltip(state string, down, up float64, live []engine.SlotStats) string {
	build := func(head string, withUp bool, unit string) string {
		lines := []string{head}
		for _, s := range live {
			l := shortName(s.Name) + " ↓" + mbps(s.LinkIn)
			if withUp {
				l += " ↑" + mbps(s.LinkOut)
			}
			if s.State == "no internet" && s.State6 != "up" {
				l += T("tip.nonet")
			}
			lines = append(lines, l)
		}
		lines = append(lines, T("tip.total", mbps(down), mbps(up))+unit)
		return Local(strings.Join(lines, "\n"))
	}
	unit := T("tip.unit")
	leanest := build(state, false, "")
	for _, t := range []string{
		build(T("app")+" · "+state, true, unit),
		build(state, true, unit),
		build(state, false, unit),
		leanest,
	} {
		if utf16Len(t) <= tooltipMax {
			return t
		}
	}
	return shorten(leanest, tooltipMax)
}

// shortName keeps adapter names short enough for the tooltip ("Ethernet 3" -> "Eth 3").
var shortNamer = strings.NewReplacer("Ethernet", "Eth", "Wi-Fi", "WiFi", "Local Area Connection", "LAN", "Cellular", "Cell")

func shortName(n string) string { return shortNamer.Replace(n) }

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// mbps formats bytes/s as Mbit/s compactly: one decimal below 10, whole numbers above.
func mbps(bytesPerSec float64) string {
	m := bytesPerSec * 8 / 1e6
	if m < 9.95 {
		return fmt.Sprintf("%.1f", m)
	}
	return fmt.Sprintf("%.0f", m)
}

// shorten cuts s to at most n UTF-16 units (what Windows counts), ending in an ellipsis.
func shorten(s string, n int) string {
	if utf16Len(s) <= n {
		return s
	}
	units := 0
	for i, r := range s {
		w := utf16.RuneLen(r)
		if units+w > n-1 {
			return s[:i] + "…"
		}
		units += w
	}
	return s
}

// limitPresets are the caps offered in each link's submenu, in Mbit/s.
var limitPresets = []float64{1, 2, 5, 10, 20, 50, 100}

func limitLabel(mbitPerSec float64) string {
	if mbitPerSec == math.Trunc(mbitPerSec) {
		return fmt.Sprintf("%.0f", mbitPerSec)
	}
	return fmt.Sprintf("%.1f", mbitPerSec)
}

// limitText is a cap as the menu shows it, e.g. "12.5 Mbit/s".
func limitText(mbitPerSec float64) string {
	return limitLabel(mbitPerSec) + " Mbit/s"
}

// showLimit ticks the cap the link shown in row is using.
func (u *trayUI) showLimit(row int, mbitPerSec float64) {
	r := u.links[row]
	setCheck(r.unlim, mbitPerSec <= 0)
	for i, it := range r.preset {
		setCheck(it, math.Abs(limitPresets[i]-mbitPerSec) < 0.01)
	}
}

// applyLimit caps the link with this name; 0 removes the cap.
func (u *trayUI) applyLimit(name string, mbitPerSec float64) {
	if name == "" {
		return
	}
	u.sched.SetLimit(name, mbitPerSec)
	if err := state.SaveLimits(u.stateDir, u.sched.Limits()); err != nil {
		u.lastErr = err.Error()
		logx.Printf("error: %v\n", err)
	}
	for i, r := range u.links {
		if r.name == name {
			u.showLimit(i, mbitPerSec)
		}
	}
}
