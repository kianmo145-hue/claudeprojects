package engine

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"

	"braid/internal/nics"
)

func TestRemoteSlotDialsThroughBooster(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	var broken, good atomic.Int32
	var nd net.Dialer
	s := NewScheduler([]nics.Nic{
		{Name: "dead phone", IP: net.IPv4(127, 0, 0, 1), Weight: 1000, Dial: func(ctx context.Context, addr string) (net.Conn, error) {
			broken.Add(1)
			return nil, &nics.LinkError{Err: errors.New("phone gone")}
		}},
		{Name: "phone", IP: net.IPv4(127, 0, 0, 1), Weight: 1, Dial: func(ctx context.Context, addr string) (net.Conn, error) {
			good.Add(1)
			return nd.DialContext(ctx, "tcp4", addr)
		}},
	})
	b := NewBalancer(s)
	c, sl, f, err := b.DialTCP(context.Background(), "127.0.0.1", ln.Addr().(*net.TCPAddr).Port)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	s.ReleaseFor(sl, f)
	if sl.Name != "phone" || good.Load() != 1 {
		t.Fatalf("flow went to %q (good dials %d, broken %d)", sl.Name, good.Load(), broken.Load())
	}
	// The broken link got its chance first (weight 1000), failed as a link error and was skipped.
	if broken.Load() != 1 {
		t.Fatalf("broken dials = %d, want 1", broken.Load())
	}
	for _, st := range s.Snapshot() {
		if !st.Remote {
			t.Fatalf("%s should be marked remote", st.Name)
		}
	}
	if _, err := b.ListenUDP(context.Background(), Fam4); err == nil {
		t.Fatal("UDP must not be sent through a booster phone")
	}
}
