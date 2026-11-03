package engine

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"

	"paydar/internal/build"
	"paydar/internal/guard"
	"paydar/internal/link"
	"paydar/internal/logx"
)

func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// ssServer starts a local shadowsocks server and returns its port and a stop func.
func ssServer(t *testing.T, password string) (int, func()) {
	port := freePort(t)
	cfg := fmt.Sprintf(`{"log":{"level":"error"},"inbounds":[{"type":"shadowsocks","listen":"127.0.0.1","listen_port":%d,"method":"aes-128-gcm","password":%q}],"outbounds":[{"type":"direct"}]}`, port, password)
	ctx, cancel := context.WithCancel(include.Context(context.Background()))
	opts, err := json.UnmarshalExtendedContext[option.Options](ctx, []byte(cfg))
	if err != nil {
		t.Fatal(err)
	}
	b, err := box.New(box.Options{Options: opts, Context: ctx})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Start(); err != nil {
		t.Fatal(err)
	}
	return port, func() { b.Close(); cancel() }
}

func ssLink(port int, password, name string) string {
	return fmt.Sprintf("ss://aes-128-gcm:%s@127.0.0.1:%d#%s", password, port, name)
}

func TestBuildLoadsInSingBox(t *testing.T) {
	ps, _ := link.ParseText(strings.Join([]string{
		"vless://11111111-2222-3333-4444-555555555555@example.com:443?security=reality&sni=a.com&pbk=jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0&sid=ab&fp=chrome&flow=xtls-rprx-vision#r",
		"vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?security=tls&type=ws&host=h.com&path=%2F%3Fed%3D2048#ws",
		"trojan://pw@t.example.com:443?type=grpc&serviceName=g#g",
		"hy2://pw@h.example.com:443,2000-3000?obfs=salamander&obfs-password=x#hy",
		"tuic://11111111-2222-3333-4444-555555555555:pw@u.example.com:443#tu",
		"ss://aes-128-gcm:pw@1.2.3.4:8388#ss",
	}, "\n"))
	if len(ps) != 6 {
		t.Fatalf("parsed %d", len(ps))
	}
	for _, mode := range []build.Mode{build.ModeLocal, build.ModeTest, build.ModeTUN, build.ModeSystemProxy} {
		content, err := build.Build(ps, build.Options{Mode: mode, MixedPort: 2080, BypassIran: true, DirectDNS: []string{"8.8.8.8"}})
		if err != nil {
			t.Fatal(err)
		}
		ctx := include.Context(context.Background())
		opts, err := json.UnmarshalExtendedContext[option.Options](ctx, content)
		if err != nil {
			t.Fatalf("%s: %v\n%s", mode, err, content)
		}
		b, err := box.New(box.Options{Options: opts, Context: ctx})
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		b.Close()
	}
}

func TestFailoverEndToEnd(t *testing.T) {
	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer probe.Close()
	ProbeURLs = []string{probe.URL + "/generate_204"}
	directTargets = []string{strings.TrimPrefix(probe.URL, "http://")}

	portA, stopA := ssServer(t, "pa")
	portB, stopB := ssServer(t, "pb")
	defer stopB()
	ps, _ := link.ParseText(ssLink(portA, "pa", "A") + "\n" + ssLink(portB, "pb", "B"))

	cfg := guard.DefaultConfig()
	cfg.ActiveInterval = 200 * time.Millisecond
	cfg.ConfirmDelay = 50 * time.Millisecond
	cfg.ProbeTimeout = time.Second
	logs := logx.NewRing(100)
	events := make(chan guard.Event, 10)
	e, err := Start(context.Background(), ps, build.Options{Mode: build.ModeLocal, MixedPort: freePort(t), Selected: build.Tag(ps[0])}, cfg, logs, func(ev guard.Event) { events <- ev })
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	time.Sleep(600 * time.Millisecond)
	if cur := e.Guard().Current(); cur != build.Tag(ps[0]) {
		t.Fatalf("current %s, want A", cur)
	}
	stopA()
	deadline := time.After(5 * time.Second)
	for e.Guard().Current() != build.Tag(ps[1]) {
		select {
		case <-deadline:
			t.Fatalf("no failover; snapshot %+v logs %v", e.Guard().Snapshot(), logs.Lines())
		case <-time.After(50 * time.Millisecond):
		}
	}
	ev := <-events
	if ev.Kind != "switch" {
		t.Errorf("event %+v", ev)
	}
}

func TestOfflineServersTest(t *testing.T) {
	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer probe.Close()
	ProbeURLs = []string{probe.URL + "/generate_204"}
	port, stop := ssServer(t, "p")
	defer stop()
	ps, _ := link.ParseText(ssLink(port, "p", "ok") + "\n" + ssLink(freePort(t), "p", "dead"))
	res, err := Test(context.Background(), ps, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res[ps[0].ID].State != "ok" || res[ps[1].ID].State != "down" {
		t.Errorf("results %+v", res)
	}
}
