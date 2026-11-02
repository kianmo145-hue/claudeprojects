package boost

import (
	"context"
	"crypto/hmac"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Server is the phone's side: it accepts pairing requests and tunnels from paired computers on the
// local network and connects tunnels out through mobile data.
type Server struct {
	PhoneID   string
	PhoneName string

	// Lookup returns the key of a paired computer.
	Lookup func(pcID string) (key []byte, ok bool)
	// Approve shows a pairing request (computer name and code) and blocks until the user decides.
	Approve func(pcName, code string) bool
	// Paired stores a newly paired computer.
	Paired func(p Peer)
	// Dial connects to a target, pinned to mobile data. nil uses a plain dialer (tests).
	Dial func(ctx context.Context, network, addr string) (net.Conn, error)
	// Allow filters incoming connections by remote address (e.g. only the Wi-Fi subnet).
	// nil allows any.
	Allow func(remote net.Addr) bool

	ConnectTimeout time.Duration

	pairing atomic.Bool // one pairing prompt at a time
	mu      sync.Mutex
	clients map[string]*ClientStats
	conns   map[net.Conn]struct{}
	closed  bool
}

// ClientStats are the live numbers for one paired computer.
type ClientStats struct {
	ID       string
	Name     string
	Active   atomic.Int64
	BytesIn  atomic.Int64 // from the internet to the computer
	BytesOut atomic.Int64 // from the computer to the internet
	LastSeen atomic.Int64 // unix seconds
}

// Serve accepts connections until ln is closed.
func (s *Server) Serve(ln net.Listener) error {
	for {
		c, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		if s.Allow != nil && !s.Allow(c.RemoteAddr()) {
			c.Close()
			continue
		}
		go s.handle(c)
	}
}

// Close drops every open tunnel. The listener is closed by its owner.
func (s *Server) Close() {
	s.mu.Lock()
	s.closed = true
	for c := range s.conns {
		c.Close()
	}
	s.mu.Unlock()
}

func (s *Server) track(c net.Conn, on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conns == nil {
		s.conns = map[net.Conn]struct{}{}
	}
	if on {
		s.conns[c] = struct{}{}
	} else {
		delete(s.conns, c)
	}
}

// Stats returns a copy of the per-computer numbers.
func (s *Server) Stats() []ClientSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ClientSnapshot, 0, len(s.clients))
	for _, c := range s.clients {
		out = append(out, ClientSnapshot{ID: c.ID, Name: c.Name, Active: c.Active.Load(),
			BytesIn: c.BytesIn.Load(), BytesOut: c.BytesOut.Load(), LastSeen: c.LastSeen.Load()})
	}
	return out
}

// ClientSnapshot is a plain copy of ClientStats.
type ClientSnapshot struct {
	ID, Name                  string
	Active, BytesIn, BytesOut int64
	LastSeen                  int64
}

func (s *Server) client(id string) *ClientStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clients == nil {
		s.clients = map[string]*ClientStats{}
	}
	c := s.clients[id]
	if c == nil {
		c = &ClientStats{ID: id}
		s.clients[id] = c
	}
	return c
}

// SetClientName records a display name (from pairing or the app's saved list).
func (s *Server) SetClientName(id, name string) { s.client(id).Name = name }

func (s *Server) handle(c net.Conn) {
	s.track(c, true)
	defer s.track(c, false)
	c.SetDeadline(time.Now().Add(15 * time.Second))
	op := make([]byte, 1)
	if _, err := io.ReadFull(c, op); err != nil {
		c.Close()
		return
	}
	switch op[0] {
	case 'J':
		s.handlePair(c)
	case 'T':
		s.handleTunnel(c)
	default:
		c.Close()
	}
}

func (s *Server) handlePair(c net.Conn) {
	defer c.Close()
	if !s.pairing.CompareAndSwap(false, true) {
		writeJSON(c, pairReply{Err: "another pairing is in progress"})
		return
	}
	defer s.pairing.Store(false)
	c.SetDeadline(time.Now().Add(90 * time.Second)) // the user needs time to compare codes
	p, err := pairServer(c, s.PhoneID, s.PhoneName, s.Approve)
	if err != nil {
		return
	}
	s.SetClientName(p.ID, p.Name)
	if s.Paired != nil {
		s.Paired(p)
	}
}

func (s *Server) handleTunnel(c net.Conn) {
	chal, err := newChallenge()
	if err != nil {
		c.Close()
		return
	}
	if _, err := c.Write(chal); err != nil {
		c.Close()
		return
	}
	var req tunnelReq
	if err := readJSON(c, &req); err != nil {
		c.Close()
		return
	}
	key, ok := []byte(nil), false
	if s.Lookup != nil {
		key, ok = s.Lookup(req.ID)
	}
	mac, _ := hex.DecodeString(req.MAC)
	if !ok || !hmac.Equal(mac, tunnelMAC(key, chal, req.Net, req.Addr)) {
		writeJSON(c, tunnelRep{Err: "unauthorized"})
		c.Close()
		return
	}
	if req.Net != "tcp" {
		writeJSON(c, tunnelRep{Err: "only tcp is supported"})
		c.Close()
		return
	}
	st := s.client(req.ID)
	st.LastSeen.Store(time.Now().Unix())
	timeout := s.ConnectTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	dial := s.Dial
	if dial == nil {
		var nd net.Dialer
		dial = nd.DialContext
	}
	up, err := dial(ctx, "tcp", req.Addr)
	cancel()
	if err != nil {
		writeJSON(c, tunnelRep{Err: err.Error()})
		c.Close()
		return
	}
	if err := writeJSON(c, tunnelRep{OK: true}); err != nil {
		up.Close()
		c.Close()
		return
	}
	c.SetDeadline(time.Time{})
	st.Active.Add(1)
	defer st.Active.Add(-1)
	s.track(up, true)
	defer s.track(up, false)
	relay(c, up, &st.BytesOut, &st.BytesIn)
}

// relay copies both ways, honouring half-close, then closes both.
func relay(a, b net.Conn, aToB, bToA *atomic.Int64) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pipe(b, a, aToB)
	}()
	pipe(a, b, bToA)
	wg.Wait()
	a.Close()
	b.Close()
}

func pipe(dst, src net.Conn, n *atomic.Int64) {
	buf := make([]byte, 64<<10)
	for {
		k, err := src.Read(buf)
		if k > 0 {
			n.Add(int64(k))
			if _, werr := dst.Write(buf[:k]); werr != nil {
				dst.Close()
				src.Close()
				return
			}
		}
		if err != nil {
			if err == io.EOF {
				if cw, ok := dst.(interface{ CloseWrite() error }); ok {
					cw.CloseWrite()
					return
				}
			}
			dst.Close()
			src.Close()
			return
		}
	}
}
