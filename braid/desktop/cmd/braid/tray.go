package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"braid/internal/booster"
	"braid/internal/i18n"
	"braid/internal/logx"
	"braid/internal/platform"
	"braid/internal/state"
	"braid/internal/tray"
	"braid/internal/tun"
)

func cmdTray(args []string) {
	fs := flag.NewFlagSet("tray", flag.ExitOnError)
	var c common
	c.register(fs)
	hidden := fs.Bool("hidden", true, "hide the console window of the elevated process")
	startOff := fs.Bool("off", false, "start with system-wide mode OFF (default: ON)")
	noBoost := fs.Bool("no-booster", false, "don't listen for booster phones on the Wi-Fi")
	autoPause := fs.Bool("auto-pause", true, "pause by itself when fewer than two connections work, and resume when two do (pausing by hand always wins)")
	fs.Parse(args)

	if !platform.IsAdmin() {
		child := append([]string{"tray"}, args...)
		if c.stateDir != state.DefaultDir() { // the macOS passwordless rule only matches a bare `tray`
			child = append(child, "-state-dir", c.stateDir)
		}
		if err := platform.RelaunchTray(child, *hidden); err != nil {
			die(err)
		}
		fmt.Println("Approve the admin prompt. Look for the braid icon in the system tray (^ overflow arrow) or menu bar.")
		return
	}
	setLang(c.stateDir)
	if !platform.SingleInstance() {
		// The windowless build has no console: say it in a box, or a second launch looks like nothing.
		fmt.Println("braid is already running.")
		if !platform.HasConsole() {
			platform.Info(i18n.T("running.title"), i18n.T("running.already"), i18n.RTL())
		}
		return
	}
	if c.logFile == "" {
		c.stats = 0 // nobody is watching a hidden console
		if !platform.HasConsole() {
			c.logFile = filepath.Join(c.stateDir, "braid.log") // windowless build: keep a log for diagnosis
		}
	}
	if err := c.setup(); err != nil {
		die(err)
	}
	if err := platform.CheckDriver(); err != nil {
		die(err)
	}

	if !*noBoost {
		c.boost = booster.New(c.stateDir)
		c.boost.Start()
		defer c.boost.Close()
	}

	t := tun.New(nil)
	t.MTU = c.mtu
	mode := func() string {
		if t.IsOn() {
			return "system-wide (tray)"
		}
		return ""
	}
	sched, bal, err := c.boot(mode)
	if err != nil {
		die(err)
	}
	t.B = bal

	// Started at sign-in this process can beat the desktop shell by a fraction of a second. With no
	// taskbar the tray library fails to create the icon and never becomes ready, so wait for it.
	if !platform.WaitForTray(3 * time.Minute) {
		logx.Printf("warning: taskbar not found after 3 minutes, trying anyway\n")
	}
	ready := make(chan struct{})
	go welcome(c.stateDir, ready)
	go func() { // safety net: if the icon still can't be created, exit so autostart can retry
		select {
		case <-ready:
		case <-time.After(30 * time.Second):
			logx.Printf("error: tray icon did not initialise within 30s; exiting so autostart can restart it\n")
			os.Exit(4)
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() { // logout, `launchctl bootout`, kill: turn off cleanly instead of leaving routes behind
		<-sig
		t.Close()
		state.Clear(c.stateDir)
		os.Exit(0)
	}()
	tray.Run(t, sched, bal, c.boost, c.stateDir, version, !*startOff, *autoPause, ready)
}

// setLang picks the saved language, else the system's.
func setLang(dir string) {
	if lang := state.LoadSettings(dir).Lang; lang != "" {
		i18n.Set(i18n.Lang(lang))
	} else {
		i18n.Set(i18n.Lang(platform.UILanguage()))
	}
}

// welcome tells a first-time user where the app went: it has no window, and Windows 11 hides new
// tray icons behind the ^ arrow, so without this it looks like nothing started.
func welcome(dir string, ready <-chan struct{}) {
	s := state.LoadSettings(dir)
	if s.Welcomed {
		return
	}
	<-ready
	s.Welcomed = true
	if err := state.SaveSettings(dir, s); err != nil {
		logx.Printf("warning: saving settings: %v\n", err)
	}
	platform.Info(i18n.T("running.title"), i18n.T("running.welcome"), i18n.RTL())
}
