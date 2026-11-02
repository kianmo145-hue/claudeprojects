package braid

import (
	"encoding/json"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"braid/boost"
)

type fakeUI struct {
	codes chan string
	keys  chan string
}

func (f *fakeUI) OnPairRequest(pcName, code string) { f.codes <- code }
func (f *fakeUI) OnPaired(pcID, pcName, keyHex string) {
	f.keys <- pcID
}
func (f *fakeUI) OnPairEnded() {}

func TestBoostPairAndTunnel(t *testing.T) {
	ui := &fakeUI{codes: make(chan string, 1), keys: make(chan string, 1)}
	// Loopback stands in for the Wi-Fi subnet; handle 0 skips pinning off Android.
	BoostSetNetworks("", "", "127.0.0.1", 8)
	boostState.cell = 1 // pretend mobile data is up (pinning is a no-op off Android)
	if err := BoostStart("phone", "Test phone", ui); err != nil {
		t.Skipf("can't listen on %d: %v", boost.ServicePort, err)
	}
	defer BoostStop()
	addr := "127.0.0.1:" + strconv.Itoa(boost.ServicePort)

	go func() { code := <-ui.codes; BoostAnswer(len(code) == 6) }()
	c, err := net.Dial("tcp4", addr)
	if err != nil {
		t.Fatal(err)
	}
	peer, err := boost.PairClient(c, "pc", "Test PC", func(string, string) bool { return true })
	c.Close()
	if err != nil {
		t.Fatal(err)
	}
	if id := <-ui.keys; id != "pc" {
		t.Fatal(id)
	}

	echo, _ := net.Listen("tcp4", "127.0.0.1:0")
	defer echo.Close()
	go func() {
		c, err := echo.Accept()
		if err == nil {
			io.Copy(c, c)
			c.Close()
		}
	}()
	d := &boost.Dialer{PhoneAddr: addr, PCID: "pc", Key: peer.Key}
	conn, err := d.DialContext(t.Context(), echo.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.Write([]byte("ok"))
	buf := make([]byte, 2)
	io.ReadFull(conn, buf)
	conn.Close()
	time.Sleep(50 * time.Millisecond)
	var st []clientJSON
	json.Unmarshal([]byte(BoostStats()), &st)
	if string(buf) != "ok" || len(st) != 1 || st[0].BytesIn != 2 || st[0].Name != "Test PC" {
		t.Fatalf("%q %+v", buf, st)
	}
}
