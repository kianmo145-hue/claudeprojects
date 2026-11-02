package boost

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"time"
)

type tunnelReq struct {
	ID   string `json:"id"`
	Net  string `json:"net"`  // "tcp" (UDP is not carried in this version)
	Addr string `json:"addr"` // host:port
	MAC  string `json:"mac"`
}

type tunnelRep struct {
	OK  bool   `json:"ok"`
	Err string `json:"err,omitempty"`
}

const challengeLen = 16

func tunnelMAC(key, chal []byte, network, addr string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte("braid-tunnel-v1"))
	m.Write(chal)
	m.Write([]byte(network + "\x00" + addr))
	return m.Sum(nil)
}

// ErrUnauthorized means the phone no longer knows this computer (it was removed on the phone).
var ErrUnauthorized = errors.New("boost: not paired with this phone")

// OpenTunnel turns conn, a fresh connection to the phone's service port, into a connection to
// addr through the phone's mobile data. It returns once the phone has connected to addr.
func OpenTunnel(conn net.Conn, pcID string, key []byte, addr string) error {
	if _, err := conn.Write([]byte{'T'}); err != nil {
		return err
	}
	chal := make([]byte, challengeLen)
	if _, err := io.ReadFull(conn, chal); err != nil {
		return err
	}
	mac := tunnelMAC(key, chal, "tcp", addr)
	if err := writeJSON(conn, tunnelReq{ID: pcID, Net: "tcp", Addr: addr, MAC: hex.EncodeToString(mac)}); err != nil {
		return err
	}
	var rep tunnelRep
	if err := readJSON(conn, &rep); err != nil {
		return err
	}
	if !rep.OK {
		if rep.Err == "unauthorized" {
			return ErrUnauthorized
		}
		return &RemoteError{Msg: rep.Err}
	}
	return nil
}

// RemoteError is a dial error reported by the phone (the target refused, timed out, ...).
type RemoteError struct{ Msg string }

func (e *RemoteError) Error() string { return "boost: phone: " + e.Msg }

// Dialer connects to targets through one paired phone.
type Dialer struct {
	PhoneAddr string // ip:port of the phone's service
	PCID      string
	Key       []byte
	// Base dials the phone itself; nil uses a plain net.Dialer. The Windows app passes a dialer
	// pinned to the Wi-Fi adapter so the tunnel never loops back into its own virtual adapter.
	Base func(ctx context.Context, network, addr string) (net.Conn, error)
}

// DialContext opens a TCP connection to addr through the phone.
func (d *Dialer) DialContext(ctx context.Context, addr string) (net.Conn, error) {
	base := d.Base
	if base == nil {
		var nd net.Dialer
		base = nd.DialContext
	}
	c, err := base(ctx, "tcp4", d.PhoneAddr)
	if err != nil {
		return nil, err
	}
	if dl, ok := ctx.Deadline(); ok {
		c.SetDeadline(dl)
	}
	stop := context.AfterFunc(ctx, func() { c.SetDeadline(time.Unix(1, 0)) })
	err = OpenTunnel(c, d.PCID, d.Key, addr)
	stop()
	c.SetDeadline(time.Time{})
	if err != nil {
		c.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	return c, nil
}

func newChallenge() ([]byte, error) {
	chal := make([]byte, challengeLen)
	_, err := rand.Read(chal)
	return chal, err
}
