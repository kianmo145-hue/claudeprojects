// Package booster is the computer's side of booster mode: it hears phones announcing themselves
// on the Wi-Fi, pairs with them, and turns each paired phone into a link the scheduler can use.
package booster

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"braid/boost"
	"braid/internal/logx"
	"braid/internal/nics"
)

// A phone is shown as online while its announcements keep coming; it announces every 2 s.
const onlineFor = 8 * time.Second

// Saved is a paired phone as stored on disk (the key is protected, see protect_*.go).
type Saved struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Key     []byte `json:"-"`
	KeyBlob string `json:"key"` // protect(Key), hex
	Enabled bool   `json:"enabled"`
}

type file struct {
	PCID   string  `json:"pcId"`
	Phones []Saved `json:"phones"`
}

// Manager keeps the paired phones and the discovery listener.
type Manager struct {
	path   string
	pcName string
	disc   boost.Discovery

	mu     sync.Mutex
	pcID   string
	phones map[string]*Saved
	ln     net.PacketConn
}

// PhoneInfo is one row of the tray's phone section.
type PhoneInfo struct {
	ID, Name string
	Paired   bool
	Enabled  bool
	Online   bool // heard on the network just now
	Cell     bool // has mobile data right now
}

// New loads the saved phones from dir.
func New(dir string) *Manager {
	host, _ := os.Hostname()
	if host == "" {
		host = "PC"
	}
	m := &Manager{path: filepath.Join(dir, "phones.json"), pcName: host, phones: map[string]*Saved{}}
	m.load()
	return m
}

func (m *Manager) load() {
	b, err := os.ReadFile(m.path)
	if err == nil {
		var f file
		if json.Unmarshal(b, &f) == nil {
			m.pcID = f.PCID
			for _, p := range f.Phones {
				raw, err := hex.DecodeString(p.KeyBlob)
				if err != nil {
					continue
				}
				key, err := unprotect(raw)
				if err != nil || len(key) != 32 {
					logx.Printf("booster: the saved key for %s can't be read; pair it again\n", p.Name)
					continue
				}
				p.Key = key
				m.phones[p.ID] = &p
			}
		}
	}
	if m.pcID == "" {
		id := make([]byte, 12)
		rand.Read(id)
		m.pcID = hex.EncodeToString(id)
		m.save()
	}
}

// save writes the list; the caller holds mu or is the only user.
func (m *Manager) save() error {
	f := file{PCID: m.pcID}
	for _, p := range m.phones {
		blob, err := protect(p.Key)
		if err != nil {
			return err
		}
		s := *p
		s.KeyBlob = hex.EncodeToString(blob)
		f.Phones = append(f.Phones, s)
	}
	sort.Slice(f.Phones, func(i, j int) bool { return f.Phones[i].Name < f.Phones[j].Name })
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

// Start listens for phone announcements. Failing to listen is logged, not fatal: merging the
// local adapters still works.
func (m *Manager) Start() {
	allowDiscovery()
	ln, err := net.ListenPacket("udp4", net.JoinHostPort("0.0.0.0", itoa(boost.AnnouncePort)))
	if err != nil {
		logx.Printf("booster: can't listen for phones on UDP %d: %v\n", boost.AnnouncePort, err)
		return
	}
	m.mu.Lock()
	m.ln = ln
	m.mu.Unlock()
	go m.disc.Listen(ln)
}

// Close stops listening.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ln != nil {
		m.ln.Close()
		m.ln = nil
	}
}

// Phones lists paired phones first, then phones on the network that are not paired yet.
func (m *Manager) Phones() []PhoneInfo {
	seen := map[string]boost.Phone{}
	for _, p := range m.disc.Phones(onlineFor) {
		seen[p.ID] = p
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []PhoneInfo
	for _, p := range m.phones {
		a, online := seen[p.ID]
		out = append(out, PhoneInfo{ID: p.ID, Name: p.Name, Paired: true, Enabled: p.Enabled, Online: online, Cell: online && a.Cell})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	var fresh []PhoneInfo
	for id, a := range seen {
		if _, ok := m.phones[id]; !ok {
			fresh = append(fresh, PhoneInfo{ID: id, Name: a.Name, Online: true, Cell: a.Cell})
		}
	}
	sort.Slice(fresh, func(i, j int) bool { return fresh[i].Name < fresh[j].Name })
	return append(out, fresh...)
}

// Pair pairs with a phone that is on the network. confirm shows the code on the computer; it
// returns false if the user cancels there.
func (m *Manager) Pair(id string, confirm func(code, phoneName string) bool) error {
	ph, ok := m.disc.Get(id, onlineFor)
	if !ok {
		return errors.New("the phone is not on this network any more")
	}
	local := m.localFor(ph.IP)
	d := net.Dialer{Timeout: 5 * time.Second}
	if local != nil {
		d.LocalAddr = &net.TCPAddr{IP: local.IP}
		d.Control = nics.ForceEgress(local.Index, false)
	}
	c, err := d.Dial("tcp4", ph.Addr())
	if err != nil {
		return err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(2 * time.Minute))
	peer, err := boost.PairClient(c, m.pcID, m.pcName, confirm)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.phones[peer.ID] = &Saved{ID: peer.ID, Name: peer.Name, Key: peer.Key, Enabled: true}
	return m.save()
}

// SetEnabled turns a paired phone's link on or off without forgetting it.
func (m *Manager) SetEnabled(id string, on bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.phones[id]
	if p == nil {
		return errors.New("not paired")
	}
	p.Enabled = on
	return m.save()
}

// Forget removes a paired phone.
func (m *Manager) Forget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.phones, id)
	return m.save()
}

// LinkName is how a phone appears among the adapters (and is keyed in the scheduler).
func LinkName(phoneName string) string { return "📱 " + phoneName }

// Nics turns the enabled, online phones that have mobile data into scheduler links, reached
// through whichever local adapter shares the phone's subnet.
func (m *Manager) Nics(local []nics.Nic) []nics.Nic {
	seen := m.disc.Phones(onlineFor)
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []nics.Nic
	for _, ph := range seen {
		p := m.phones[ph.ID]
		if p == nil || !p.Enabled || !ph.Cell {
			continue
		}
		via := pickLocal(local, ph.IP)
		if via == nil {
			continue
		}
		key := append([]byte(nil), p.Key...)
		viaIP, viaIdx := via.IP, via.Index
		bd := &boost.Dialer{PhoneAddr: ph.Addr(), PCID: m.pcID, Key: key,
			Base: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := net.Dialer{LocalAddr: &net.TCPAddr{IP: viaIP}, Control: nics.ForceEgress(viaIdx, false), KeepAlive: 30 * time.Second}
				return d.DialContext(ctx, network, addr)
			}}
		out = append(out, nics.Nic{Name: LinkName(p.Name), IP: viaIP, Weight: 1,
			Dial: func(ctx context.Context, addr string) (net.Conn, error) {
				c, err := bd.DialContext(ctx, addr)
				if err == nil {
					return c, nil
				}
				var re *boost.RemoteError
				if errors.As(err, &re) || ctx.Err() != nil && !errors.Is(err, context.DeadlineExceeded) {
					return nil, err // the target failed, not the phone
				}
				return nil, &nics.LinkError{Err: err}
			}})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// localFor finds the local adapter on the phone's subnet, from a fresh scan.
func (m *Manager) localFor(ip net.IP) *nics.Nic {
	ns, err := nics.DiscoverNics(nil, nil)
	if err != nil {
		return nil
	}
	return pickLocal(ns, ip)
}

// pickLocal returns the adapter whose IPv4 subnet contains ip.
func pickLocal(local []nics.Nic, ip net.IP) *nics.Nic {
	for i := range local {
		n := &local[i]
		if n.Dial != nil || n.IP == nil {
			continue
		}
		if sameSubnet(n, ip) {
			return n
		}
	}
	return nil
}

var subnetOf = func(n *nics.Nic) *net.IPNet {
	ifc, err := net.InterfaceByIndex(n.Index)
	if err != nil {
		return nil
	}
	addrs, _ := ifc.Addrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.Equal(n.IP) {
			return ipn
		}
	}
	return nil
}

func sameSubnet(n *nics.Nic, ip net.IP) bool {
	ipn := subnetOf(n)
	return ipn != nil && ipn.Contains(ip)
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }
