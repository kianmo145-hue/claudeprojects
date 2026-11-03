// Command paydar is the desktop app: the Go core, the UI server, and a tray icon.
//
//	paydar             tray app (the normal way to run it)
//	paydar -headless   no tray; prints the UI address (development, servers)
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"paydar/internal/api"
	"paydar/internal/app"
	"paydar/internal/build"
	"paydar/internal/store"
)

var version = "dev"

func dataDir() string {
	if d := os.Getenv("PAYDAR_DATA"); d != "" {
		return d
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "Paydar")
}

func main() {
	headless := flag.Bool("headless", false, "run without a tray icon and print the UI address")
	port := flag.Int("port", 0, "UI port on 127.0.0.1 (0 = any free port)")
	iconOut := flag.String("write-icon", "", "write the app icon (.ico) to this path and exit (build step)")
	flag.Parse()
	if *iconOut != "" {
		if err := writeAppIcon(*iconOut); err != nil {
			fatal(err)
		}
		return
	}

	dir := dataDir()
	urlFile := filepath.Join(dir, "ui-url")
	if !*headless {
		release, first := singleInstance()
		if !first {
			// Already running: just show its window.
			if b, err := os.ReadFile(urlFile); err == nil {
				openUI(string(b))
			}
			return
		}
		defer release()
	}

	var tray *trayUI
	a, err := app.New(dir, app.Hooks{
		Mode: func(s store.Settings) build.Mode {
			if s.Mode == "proxy" || !canTUN() {
				return build.ModeSystemProxy
			}
			return build.ModeTUN
		},
		Changed: func(st app.Status) {
			if tray != nil {
				tray.update(st)
			}
		},
	})
	if err != nil {
		fatal(err)
	}
	// A crash while in system-proxy mode would leave Windows pointing at a dead proxy.
	clearStaleSystemProxy(a.Settings().MixedPort)

	srv := &api.Server{App: a, Token: api.NewToken(), Platform: runtime.GOOS, Version: version}
	quit := make(chan struct{})
	srv.OnQuit = func() { close(quit) }
	if err := srv.Listen(*port); err != nil {
		fatal(err)
	}
	os.WriteFile(urlFile, []byte(srv.URL()), 0o600)
	a.Logs.Infof("Paydar %s, data in %s", version, dir)
	if runtime.GOOS == "windows" && !canTUN() {
		a.Logs.Warnf("not running as administrator: using system-proxy mode")
	}
	a.AutoConnect()

	shutdown := func() {
		a.Close()
		srv.Close()
		os.Remove(urlFile)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	if *headless {
		fmt.Println(srv.URL())
		select {
		case <-sig:
		case <-quit:
		}
		shutdown()
		return
	}
	tray = newTray(a, srv.URL(), func() { close(quit) })
	go func() {
		select {
		case <-sig:
		case <-quit:
		}
		tray.quit()
	}()
	openUI(srv.URL())
	tray.run() // blocks until quit
	shutdown()
}
