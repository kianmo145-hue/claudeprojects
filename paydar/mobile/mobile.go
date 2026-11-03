// Package mobile is the Android side of the core, bound with gomobile.
//
// The app runs the same UI as Windows (a WebView on the local API). Connecting goes through
// Android: the UI asks the platform for the VPN, the VpnService creates the TUN and passes its file
// descriptor to StartTunnel, and tun2socks feeds every packet into sing-box's local proxy. The app
// itself is excluded from the VPN, so sing-box's own connections leave the phone directly.
package mobile

import (
	"errors"
	"strings"
	"sync"

	"paydar/internal/api"
	"paydar/internal/app"
	"paydar/internal/build"
	"paydar/internal/store"
)

// Platform is implemented in Kotlin.
type Platform interface {
	// RequestConnect: the UI's connect button; the app asks for VPN permission and starts the service.
	RequestConnect()
	// RequestDisconnect: the UI's disconnect button; the app stops the service.
	RequestDisconnect()
	// OnStatus: for the notification and tile. state is disconnected, connecting or connected.
	OnStatus(state, server string, offline bool)
}

var (
	mu       sync.Mutex
	a        *app.App
	srv      *api.Server
	plat     Platform
	dns      []string
	tunnelUp bool
	tun      tunStack
)

// Start initialises the core once per process and returns the UI address.
func Start(dataDir, version string, p Platform) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	plat = p
	if a != nil {
		return srv.URL(), nil
	}
	var err error
	a, err = app.New(dataDir, app.Hooks{
		Mode: func(store.Settings) build.Mode { return build.ModeLocal },
		DirectDNS: func() []string {
			mu.Lock()
			defer mu.Unlock()
			return append([]string(nil), dns...)
		},
		Changed: func(st app.Status) {
			mu.Lock()
			pl := plat
			mu.Unlock()
			if pl != nil {
				pl.OnStatus(string(st.State), st.ServerName, st.Offline)
			}
		},
	})
	if err != nil {
		return "", err
	}
	srv = &api.Server{
		App: a, Token: api.NewToken(), Platform: "android", Version: version,
		OnConnect: func() error {
			mu.Lock()
			pl := plat
			mu.Unlock()
			if pl == nil {
				return errors.New("app not ready")
			}
			pl.RequestConnect()
			return nil
		},
		OnDisconnect: func() {
			mu.Lock()
			pl := plat
			mu.Unlock()
			if pl != nil {
				pl.RequestDisconnect()
			}
		},
	}
	if err := srv.Listen(0); err != nil {
		a.Close()
		a = nil
		return "", err
	}
	return srv.URL(), nil
}

func parseDNS(list string) []string {
	var out []string
	for _, s := range strings.Split(list, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		out = []string{"1.1.1.1"}
	}
	return out
}

// StartTunnel connects and attaches the VPN's TUN (fd is owned by Go from here). dnsList is the
// current network's DNS servers, comma-separated, used to resolve server addresses.
func StartTunnel(fd, mtu int64, dnsList string) error {
	mu.Lock()
	if a == nil {
		mu.Unlock()
		return errors.New("core not started")
	}
	dns = parseDNS(dnsList)
	ap := a
	mu.Unlock()

	if err := ap.Connect(); err != nil {
		return err
	}
	if err := tun.start(int(fd), int(mtu), ap.Settings().MixedPort); err != nil {
		ap.Disconnect()
		return err
	}
	mu.Lock()
	tunnelUp = true
	mu.Unlock()
	return nil
}

// StopTunnel disconnects and releases the TUN.
func StopTunnel() {
	mu.Lock()
	up := tunnelUp
	tunnelUp = false
	ap := a
	mu.Unlock()
	if up {
		tun.stop()
	}
	if ap != nil {
		ap.Disconnect()
	}
}

// NetworkChanged: the default network changed (Wi-Fi ⇄ mobile). A new DNS list means the old
// resolver may be unreachable, so sing-box restarts with the new one; otherwise the guard just
// re-checks at once.
func NetworkChanged(dnsList string) {
	mu.Lock()
	ap := a
	next := parseDNS(dnsList)
	same := strings.Join(next, ",") == strings.Join(dns, ",")
	dns = next
	up := tunnelUp
	mu.Unlock()
	if ap == nil || !up {
		return
	}
	if same {
		ap.NetworkChanged()
	} else {
		ap.Reconnect("network changed")
	}
}

func IsConnected() bool {
	mu.Lock()
	defer mu.Unlock()
	return tunnelUp
}
