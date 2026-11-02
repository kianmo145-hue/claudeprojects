package braid

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"braid-golib/internal/nics"
	"braid/boost"
)

// Booster mode: the phone shares its mobile data with paired computers on the same Wi-Fi. The
// Kotlin side (BoostService) keeps Wi-Fi and mobile data up, tells this side their network handles
// and the Wi-Fi address, stores the paired computers, and shows the pairing prompt.

// BoostListener receives booster events on the Kotlin side. Calls come from Go goroutines.
type BoostListener interface {
	// OnPairRequest asks the user to compare code with the computer's screen; answer with
	// BoostAnswer. Unanswered requests are declined after a minute.
	OnPairRequest(pcName, code string)
	// OnPaired hands over a newly paired computer for storage (keyHex is its secret).
	OnPaired(pcID, pcName, keyHex string)
	// OnPairEnded tells the UI the request is over (answered, cancelled on the computer, timed out).
	OnPairEnded()
}

var boostState struct {
	mu       sync.Mutex
	srv      *boost.Server
	ln       net.Listener
	cancel   context.CancelFunc
	cell     uint64 // mobile data network handle, 0 = none
	wifi     uint64 // Wi-Fi network handle
	wifiNet  *net.IPNet
	peers    map[string][]byte
	answer   chan bool
	phoneID  string
	phone    string
	listener BoostListener
}

type peerJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

// BoostSetPeers replaces the list of paired computers: a JSON array of {id, name, key (hex)}.
func BoostSetPeers(js string) error {
	var in []peerJSON
	if err := json.Unmarshal([]byte(js), &in); err != nil {
		return err
	}
	peers := map[string][]byte{}
	for _, p := range in {
		k, err := hex.DecodeString(p.Key)
		if err != nil || len(k) != 32 {
			continue
		}
		peers[p.ID] = k
	}
	b := &boostState
	b.mu.Lock()
	b.peers = peers
	srv := b.srv
	b.mu.Unlock()
	if srv != nil {
		for _, p := range in {
			srv.SetClientName(p.ID, p.Name)
		}
	}
	return nil
}

// BoostSetNetworks updates the handles of mobile data and Wi-Fi ("" when absent, decimal
// otherwise) and the Wi-Fi IPv4 address with its prefix length. Called whenever they change.
func BoostSetNetworks(cellHandle, wifiHandle, wifiIP string, prefix int) {
	b := &boostState
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cell, _ = strconv.ParseUint(cellHandle, 10, 64)
	b.wifi, _ = strconv.ParseUint(wifiHandle, 10, 64)
	b.wifiNet = nil
	if ip := net.ParseIP(wifiIP).To4(); ip != nil && prefix > 0 && prefix <= 32 {
		b.wifiNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(prefix, 32)} // IP stays the phone's own (for the broadcast address)
	}
}

// BoostStart starts sharing: the service on TCP 47801 and announcements every 2 s.
func BoostStart(phoneID, phoneName string, l BoostListener) error {
	b := &boostState
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.srv != nil {
		return nil
	}
	b.phoneID, b.phone, b.listener = phoneID, phoneName, l
	ln, err := net.Listen("tcp4", ":"+strconv.Itoa(boost.ServicePort))
	if err != nil {
		return err
	}
	srv := &boost.Server{
		PhoneID: phoneID, PhoneName: phoneName,
		Lookup: func(id string) ([]byte, bool) {
			b.mu.Lock()
			defer b.mu.Unlock()
			k, ok := b.peers[id]
			return k, ok
		},
		Approve: approve,
		Paired: func(p boost.Peer) {
			b.mu.Lock()
			if b.peers == nil {
				b.peers = map[string][]byte{}
			}
			b.peers[p.ID] = p.Key
			b.mu.Unlock()
			if l != nil {
				l.OnPaired(p.ID, p.Name, hex.EncodeToString(p.Key))
			}
		},
		Dial:  dialCell,
		Allow: allowLAN,
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.srv, b.ln, b.cancel = srv, ln, cancel
	go srv.Serve(ln)
	go announce(ctx)
	return nil
}

// BoostStop stops sharing and drops every open tunnel.
func BoostStop() {
	b := &boostState
	b.mu.Lock()
	srv, ln, cancel := b.srv, b.ln, b.cancel
	b.srv, b.ln, b.cancel = nil, nil, nil
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if ln != nil {
		ln.Close()
	}
	if srv != nil {
		srv.Close()
	}
}

// BoostRunning reports whether sharing is on.
func BoostRunning() bool {
	b := &boostState
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.srv != nil
}

// BoostAnswer answers the pending pairing request.
func BoostAnswer(approve bool) {
	b := &boostState
	b.mu.Lock()
	ch := b.answer
	b.mu.Unlock()
	if ch != nil {
		select {
		case ch <- approve:
		default:
		}
	}
}

func approve(pcName, code string) bool {
	b := &boostState
	ch := make(chan bool, 1)
	b.mu.Lock()
	b.answer = ch
	l := b.listener
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.answer = nil
		b.mu.Unlock()
		if l != nil {
			l.OnPairEnded()
		}
	}()
	if l == nil {
		return false
	}
	l.OnPairRequest(pcName, code)
	select {
	case ok := <-ch:
		return ok
	case <-time.After(time.Minute):
		return false
	}
}

var errNoCell = errors.New("mobile data is not available")

// dialCell connects out through mobile data only: never Wi-Fi, which the computer already has.
func dialCell(ctx context.Context, network, addr string) (net.Conn, error) {
	b := &boostState
	b.mu.Lock()
	h := b.cell
	b.mu.Unlock()
	if h == 0 {
		return nil, errNoCell
	}
	d := net.Dialer{Control: nics.PinToHandle(h), KeepAlive: 30 * time.Second}
	return d.DialContext(ctx, network, addr)
}

// allowLAN accepts only computers on the phone's Wi-Fi subnet.
func allowLAN(a net.Addr) bool {
	ta, ok := a.(*net.TCPAddr)
	if !ok {
		return false
	}
	b := &boostState
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.wifiNet != nil && b.wifiNet.Contains(ta.IP)
}

func announce(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	var pc net.PacketConn
	var pcWifi uint64
	defer func() {
		if pc != nil {
			pc.Close()
		}
	}()
	for {
		b := &boostState
		b.mu.Lock()
		wifi, wnet, cell := b.wifi, b.wifiNet, b.cell
		a := boost.Announcement{Svc: boost.Service, V: boost.Version, ID: b.phoneID, Name: b.phone, Port: boost.ServicePort, Cell: cell != 0}
		b.mu.Unlock()
		if wnet != nil {
			if pc == nil || pcWifi != wifi {
				if pc != nil {
					pc.Close()
				}
				lc := net.ListenConfig{Control: nics.PinToHandle(wifi)}
				pc, _ = lc.ListenPacket(ctx, "udp4", ":0")
				pcWifi = wifi
			}
			if pc != nil {
				if msg, err := json.Marshal(a); err == nil {
					ones, _ := wnet.Mask.Size()
					dst := &net.UDPAddr{IP: boost.BroadcastAddr(wnet.IP, ones), Port: boost.AnnouncePort}
					pc.WriteTo(msg, dst)
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

type clientJSON struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Active   int64  `json:"active"`
	BytesIn  int64  `json:"bytesIn"`
	BytesOut int64  `json:"bytesOut"`
	LastSeen int64  `json:"lastSeen"`
}

// BoostStats returns a JSON array with one entry per computer that used the phone.
func BoostStats() string {
	b := &boostState
	b.mu.Lock()
	srv := b.srv
	b.mu.Unlock()
	out := []clientJSON{}
	if srv != nil {
		for _, c := range srv.Stats() {
			out = append(out, clientJSON{c.ID, c.Name, c.Active, c.BytesIn, c.BytesOut, c.LastSeen})
		}
	}
	js, _ := json.Marshal(out)
	return string(js)
}
