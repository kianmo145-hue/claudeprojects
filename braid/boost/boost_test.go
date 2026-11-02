package boost

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type keyStore struct {
	mu   sync.Mutex
	keys map[string][]byte
}

func (k *keyStore) lookup(id string) ([]byte, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.keys[id]
	return v, ok
}

func (k *keyStore) put(p Peer) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.keys == nil {
		k.keys = map[string][]byte{}
	}
	k.keys[p.ID] = p.Key
}

func startServer(t *testing.T, approve bool) (*Server, *keyStore, string, *string) {
	t.Helper()
	ks := &keyStore{}
	var phoneCode string
	s := &Server{
		PhoneID: "phone-1", PhoneName: "Test Phone",
		Lookup:  ks.lookup,
		Paired:  ks.put,
		Approve: func(pcName, code string) bool { phoneCode = code; return approve },
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go s.Serve(ln)
	t.Cleanup(func() { ln.Close(); s.Close() })
	return s, ks, ln.Addr().String(), &phoneCode
}

func pair(t *testing.T, addr string) (Peer, string, error) {
	c, err := net.Dial("tcp4", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var pcCode string
	p, err := PairClient(c, "pc-1", "Test PC", func(code, phone string) bool { pcCode = code; return true })
	return p, pcCode, err
}

func echoServer(t *testing.T) string {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { io.Copy(c, c); c.Close() }()
		}
	}()
	return ln.Addr().String()
}

func TestPairAndTunnel(t *testing.T) {
	srv, ks, addr, phoneCode := startServer(t, true)
	peer, pcCode, err := pair(t, addr)
	if err != nil {
		t.Fatal(err)
	}
	if pcCode == "" || pcCode != *phoneCode || len(pcCode) != 6 {
		t.Fatalf("codes differ: pc %q phone %q", pcCode, *phoneCode)
	}
	if peer.ID != "phone-1" || peer.Name != "Test Phone" {
		t.Fatalf("peer = %+v", peer)
	}
	time.Sleep(50 * time.Millisecond) // Paired runs after the result is written
	key, ok := ks.lookup("pc-1")
	if !ok || string(key) != string(peer.Key) {
		t.Fatal("both sides must derive the same key")
	}

	echo := echoServer(t)
	d := &Dialer{PhoneAddr: addr, PCID: "pc-1", Key: peer.Key}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := d.DialContext(ctx, echo)
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("hello through the phone")
	c.Write(msg)
	got := make([]byte, len(msg))
	if _, err := io.ReadFull(c, got); err != nil || string(got) != string(msg) {
		t.Fatalf("echo: %q %v", got, err)
	}
	c.Close()
	time.Sleep(50 * time.Millisecond)
	st := srv.Stats()
	if len(st) != 1 || st[0].BytesOut != int64(len(msg)) || st[0].BytesIn != int64(len(msg)) || st[0].Name != "Test PC" {
		t.Fatalf("stats = %+v", st)
	}
}

func TestDeclined(t *testing.T) {
	_, ks, addr, _ := startServer(t, false)
	if _, _, err := pair(t, addr); err != ErrDeclined {
		t.Fatalf("err = %v, want ErrDeclined", err)
	}
	if _, ok := ks.lookup("pc-1"); ok {
		t.Fatal("a declined computer must not be stored")
	}
}

func TestWrongKeyRejected(t *testing.T) {
	_, ks, addr, _ := startServer(t, true)
	ks.put(Peer{ID: "pc-1", Key: make([]byte, 32)})
	bad := make([]byte, 32)
	bad[0] = 1
	d := &Dialer{PhoneAddr: addr, PCID: "pc-1", Key: bad}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := d.DialContext(ctx, echoServer(t)); err != ErrUnauthorized {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	d.PCID = "unknown"
	if _, err := d.DialContext(ctx, echoServer(t)); err != ErrUnauthorized {
		t.Fatalf("unknown pc: err = %v", err)
	}
}

func TestRemoteDialError(t *testing.T) {
	_, ks, addr, _ := startServer(t, true)
	key := make([]byte, 32)
	ks.put(Peer{ID: "pc-1", Key: key})
	ln, _ := net.Listen("tcp4", "127.0.0.1:0")
	dead := ln.Addr().String()
	ln.Close()
	d := &Dialer{PhoneAddr: addr, PCID: "pc-1", Key: key}
	_, err := d.DialContext(context.Background(), dead)
	if _, ok := err.(*RemoteError); !ok {
		t.Fatalf("err = %v (%T), want RemoteError", err, err)
	}
}

func TestCodeCommitment(t *testing.T) {
	// A computer that reveals a key other than the one it committed to is refused.
	_, _, addr, _ := startServer(t, true)
	c, _ := net.Dial("tcp4", addr)
	defer c.Close()
	c.Write([]byte{'J'})
	writeJSON(c, pairHello{Op: "pair", ID: "x", Name: "x", Commit: "00000000000000000000000000000000000000000000000000000000000000ff"})
	var rep pairReply
	readJSON(c, &rep)
	writeJSON(c, pairReveal{Pub: rep.Pub})
	var res pairResult
	if err := readJSON(c, &res); err != nil || res.OK || res.Err == "" {
		t.Fatalf("res = %+v err %v", res, err)
	}
}

func TestDiscovery(t *testing.T) {
	ln, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var d Discovery
	go d.Listen(ln)
	defer ln.Close()
	snd, _ := net.ListenPacket("udp4", "127.0.0.1:0")
	defer snd.Close()
	ctx, cancel := context.WithCancel(context.Background())
	go Announce(ctx, snd, []*net.UDPAddr{ln.LocalAddr().(*net.UDPAddr)}, 20*time.Millisecond,
		func() Announcement { return Announcement{ID: "p1", Name: "Phone", Port: 47801, Cell: true} })
	defer cancel()
	snd.WriteTo([]byte(`{"svc":"other","id":"x","port":1}`), ln.LocalAddr())
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ps := d.Phones(time.Minute); len(ps) == 1 {
			if ps[0].Addr() != "127.0.0.1:47801" || !ps[0].Cell {
				t.Fatalf("phone = %+v", ps[0])
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("announcement not heard")
}

func TestBroadcastAddr(t *testing.T) {
	if got := BroadcastAddr(net.ParseIP("192.168.1.23"), 24).String(); got != "192.168.1.255" {
		t.Fatal(got)
	}
	if got := BroadcastAddr(net.ParseIP("10.0.5.1"), 16).String(); got != "10.0.255.255" {
		t.Fatal(got)
	}
	var _ = json.Marshal
}
