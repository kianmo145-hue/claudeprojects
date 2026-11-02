package booster

import (
	"context"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"braid/boost"
	"braid/internal/nics"
)

func TestPairThenUsePhoneAsLink(t *testing.T) {
	keys := map[string][]byte{}
	srv := &boost.Server{PhoneID: "ph1", PhoneName: "Pixel",
		Lookup:  func(id string) ([]byte, bool) { k, ok := keys[id]; return k, ok },
		Paired:  func(p boost.Peer) { keys[p.ID] = p.Key },
		Approve: func(string, string) bool { return true }}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go srv.Serve(ln)

	echo, _ := net.Listen("tcp4", "127.0.0.1:0")
	defer echo.Close()
	go func() {
		for {
			c, err := echo.Accept()
			if err != nil {
				return
			}
			go func() { io.Copy(c, c); c.Close() }()
		}
	}()

	dir := t.TempDir()
	m := New(dir)
	port := ln.Addr().(*net.TCPAddr).Port
	m.disc.Add(boost.Phone{Announcement: boost.Announcement{ID: "ph1", Name: "Pixel", Port: port, Cell: true}, IP: net.IPv4(127, 0, 0, 1)})
	if got := m.Phones(); len(got) != 1 || got[0].Paired {
		t.Fatalf("before pairing: %+v", got)
	}
	var code string
	if err := m.Pair("ph1", func(c, name string) bool { code = c; return name == "Pixel" }); err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 {
		t.Fatalf("code %q", code)
	}
	time.Sleep(50 * time.Millisecond)

	// The pairing survives a restart.
	m2 := New(dir)
	m2.disc.Add(boost.Phone{Announcement: boost.Announcement{ID: "ph1", Name: "Pixel", Port: port, Cell: true}, IP: net.IPv4(127, 0, 0, 1)})
	if got := m2.Phones(); len(got) != 1 || !got[0].Paired || !got[0].Enabled || !got[0].Online {
		t.Fatalf("after reload: %+v", got)
	}

	subnetOf = func(n *nics.Nic) *net.IPNet { return &net.IPNet{IP: net.IPv4(127, 0, 0, 0), Mask: net.CIDRMask(8, 32)} }
	links := m2.Nics([]nics.Nic{{Name: "lo", IP: net.IPv4(127, 0, 0, 1)}})
	if len(links) != 1 || links[0].Name != "📱 Pixel" || links[0].Dial == nil {
		t.Fatalf("links = %+v", links)
	}
	c, err := links[0].Dial(context.Background(), echo.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	c.Write([]byte("hi"))
	b := make([]byte, 2)
	if _, err := io.ReadFull(c, b); err != nil || string(b) != "hi" {
		t.Fatalf("%q %v", b, err)
	}
	c.Close()
	if _, ok := c.(*net.TCPConn); !ok {
		t.Fatalf("the engine needs a *net.TCPConn, got %T", c)
	}

	// A phone that stopped answering is a link failure; a refused target is not.
	ln.Close()
	srv.Close()
	_, err = links[0].Dial(context.Background(), "127.0.0.1:"+strconv.Itoa(port))
	if _, ok := err.(*nics.LinkError); !ok {
		t.Fatalf("err = %v (%T), want LinkError", err, err)
	}

	m2.SetEnabled("ph1", false)
	if len(m2.Nics([]nics.Nic{{Name: "lo", IP: net.IPv4(127, 0, 0, 1)}})) != 0 {
		t.Fatal("a disabled phone must not be used")
	}
	m2.Forget("ph1")
	if got := New(dir).Phones(); len(got) != 0 {
		t.Fatalf("after forget: %+v", got)
	}
}
