package boost

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"
)

// Announcement is what a phone broadcasts every couple of seconds while booster mode is on.
type Announcement struct {
	Svc  string `json:"svc"`
	V    int    `json:"v"`
	ID   string `json:"id"`
	Name string `json:"name"`
	Port int    `json:"port"`
	Cell bool   `json:"cell"` // mobile data is available right now
}

// Announce sends a to each destination every interval until ctx ends. pc must be allowed to
// broadcast (Go sets SO_BROADCAST on UDP sockets).
func Announce(ctx context.Context, pc net.PacketConn, dst []*net.UDPAddr, interval time.Duration, a func() Announcement) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		ann := a()
		ann.Svc, ann.V = Service, Version
		if b, err := json.Marshal(ann); err == nil {
			for _, d := range dst {
				pc.WriteTo(b, d)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// BroadcastAddr is the directed broadcast address of ip/prefix (e.g. 192.168.1.255).
func BroadcastAddr(ip net.IP, prefix int) net.IP {
	ip4 := ip.To4()
	if ip4 == nil || prefix < 0 || prefix > 32 {
		return nil
	}
	mask := net.CIDRMask(prefix, 32)
	out := make(net.IP, 4)
	for i := range 4 {
		out[i] = ip4[i] | ^mask[i]
	}
	return out
}

// Phone is a phone seen on the network.
type Phone struct {
	Announcement
	IP   net.IP
	Seen time.Time
}

// Addr is the phone's service address.
func (p Phone) Addr() string {
	return net.JoinHostPort(p.IP.String(), itoa(p.Port))
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// Discovery collects announcements heard on a UDP socket.
type Discovery struct {
	mu     sync.Mutex
	phones map[string]Phone
	now    func() time.Time
}

// Listen reads announcements from pc until it is closed.
func (d *Discovery) Listen(pc net.PacketConn) {
	buf := make([]byte, 2048)
	for {
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		var a Announcement
		if json.Unmarshal(buf[:n], &a) != nil || a.Svc != Service || a.ID == "" || a.Port <= 0 || a.Port > 65535 {
			continue
		}
		ua, ok := from.(*net.UDPAddr)
		if !ok {
			continue
		}
		d.Add(Phone{Announcement: a, IP: ua.IP})
	}
}

// Add records a phone (Listen calls it; tests and manual "add by IP" can too).
func (d *Discovery) Add(p Phone) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.phones == nil {
		d.phones = map[string]Phone{}
	}
	if d.now == nil {
		d.now = time.Now
	}
	p.Seen = d.now()
	d.phones[p.ID] = p
}

// Phones returns the phones heard within maxAge.
func (d *Discovery) Phones(maxAge time.Duration) []Phone {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.now == nil {
		d.now = time.Now
	}
	var out []Phone
	for _, p := range d.phones {
		if d.now().Sub(p.Seen) <= maxAge {
			out = append(out, p)
		}
	}
	return out
}

// Get returns one phone if it was heard within maxAge.
func (d *Discovery) Get(id string, maxAge time.Duration) (Phone, bool) {
	for _, p := range d.Phones(maxAge) {
		if p.ID == id {
			return p, true
		}
	}
	return Phone{}, false
}
