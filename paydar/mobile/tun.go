package mobile

import (
	"net"
	"strconv"
	"sync"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/stack"

	"github.com/xjasonlyu/tun2socks/v2/core"
	"github.com/xjasonlyu/tun2socks/v2/core/device"
	"github.com/xjasonlyu/tun2socks/v2/core/device/fdbased"
	"github.com/xjasonlyu/tun2socks/v2/core/option"
	tlog "github.com/xjasonlyu/tun2socks/v2/log"
	"github.com/xjasonlyu/tun2socks/v2/proxy/reject"
	"github.com/xjasonlyu/tun2socks/v2/proxy/socks5"
	"github.com/xjasonlyu/tun2socks/v2/tunnel"
)

// tunStack terminates the VPN's TUN in gVisor (tun2socks) and sends every flow, TCP and UDP, to
// sing-box's local SOCKS5 port. Errors are returned, never fatal (upstream's engine exits the
// process on errors).
type tunStack struct {
	mu  sync.Mutex
	dev device.Device
	stk *stack.Stack
}

func (t *tunStack) start(fd, mtu, socksPort int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.teardown()
	if lvl, err := tlog.ParseLevel("error"); err == nil {
		tlog.SetLogger(tlog.Must(tlog.NewLeveled(lvl)))
	}
	px, err := socks5.New(net.JoinHostPort("127.0.0.1", strconv.Itoa(socksPort)), "", "")
	if err != nil {
		return err
	}
	tunnel.T().SetUDPTimeout(30 * time.Second)
	tunnel.T().SetProxy(px)
	dev, err := fdbased.Open(strconv.Itoa(fd), uint32(mtu), 0)
	if err != nil {
		return err
	}
	stk, err := core.CreateStack(&core.Config{
		LinkEndpoint:     dev,
		TransportHandler: tunnel.T(),
		Options:          []option.Option{option.WithTCPModerateReceiveBuffer(true)},
	})
	if err != nil {
		dev.Close()
		return err
	}
	t.dev, t.stk = dev, stk
	return nil
}

func (t *tunStack) stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.teardown()
}

func (t *tunStack) teardown() {
	tunnel.T().SetProxy(&reject.Reject{})
	if t.dev != nil {
		t.dev.Close()
	}
	if t.stk != nil {
		done := make(chan struct{})
		stk := t.stk
		go func() { stk.Close(); stk.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
	t.dev, t.stk = nil, nil
}
