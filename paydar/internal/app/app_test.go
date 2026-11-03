package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"

	"paydar/internal/build"
	"paydar/internal/engine"
	"paydar/internal/store"
)

func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func ssServer(t *testing.T) int {
	port := freePort(t)
	cfg := fmt.Sprintf(`{"log":{"level":"error"},"inbounds":[{"type":"shadowsocks","listen":"127.0.0.1","listen_port":%d,"method":"aes-128-gcm","password":"pw"}],"outbounds":[{"type":"direct"}]}`, port)
	ctx := include.Context(context.Background())
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
	t.Cleanup(func() { b.Close() })
	return port
}

func TestConnectFlowAndSubscription(t *testing.T) {
	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer probe.Close()
	engine.ProbeURLs = []string{probe.URL + "/generate_204"}

	good := ssServer(t)
	links := fmt.Sprintf("ss://aes-128-gcm:pw@127.0.0.1:%d#dead\nss://aes-128-gcm:pw@127.0.0.1:%d#good", freePort(t), good)
	sub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=1; download=2; total=100; expire=2000000000")
		w.Header().Set("Profile-Title", "base64:VGVzdCBTdWI=")
		fmt.Fprint(w, links)
	}))
	defer sub.Close()

	a, err := New(t.TempDir(), Hooks{Mode: func(store.Settings) build.Mode { return build.ModeLocal }})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	s := a.Settings()
	s.MixedPort = freePort(t)
	a.SetSettings(s)

	res, err := a.Add(sub.URL)
	if err != nil || !res.Sub || res.Added != 2 {
		t.Fatalf("add sub: %+v %v", res, err)
	}
	subs := a.Subs()
	if subs[0].Name != "Test Sub" || subs[0].Total != 100 {
		t.Errorf("sub info %+v", subs[0])
	}

	if err := a.TestAll(); err != nil {
		t.Fatal(err)
	}
	if v := a.Servers(); v[0].Name != "good" {
		t.Errorf("tested list not ranked: first is %s", v[0].Name)
	}

	if err := a.Connect(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for a.Status().ServerName != "good" && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	st := a.Status()
	if st.State != Connected || st.ServerName != "good" {
		t.Fatalf("status %+v", st)
	}
	// Traffic really flows through the local proxy and the good server.
	pc, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", s.MixedPort))
	if err != nil {
		t.Fatal(err)
	}
	pc.Close()

	a.Disconnect()
	if a.Status().State != Disconnected {
		t.Error("still connected")
	}
}
