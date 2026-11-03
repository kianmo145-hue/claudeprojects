package main

import (
	"sync"

	"fyne.io/systray"

	"paydar/internal/app"
)

type trayUI struct {
	a      *app.App
	url    string
	onQuit func()

	mu        sync.Mutex
	ready     bool
	mStatus   *systray.MenuItem
	mToggle   *systray.MenuItem
	lastState app.State
	quitOnce  sync.Once
}

func newTray(a *app.App, url string, onQuit func()) *trayUI {
	return &trayUI{a: a, url: url, onQuit: onQuit}
}

func (t *trayUI) run() {
	systray.Run(t.onReady, func() {})
}

func (t *trayUI) quit() { t.quitOnce.Do(systray.Quit) }

func (t *trayUI) onReady() {
	systray.SetIcon(trayIcon(app.Disconnected, false))
	systray.SetTitle("Paydar")
	systray.SetTooltip("Paydar")
	mOpen := systray.AddMenuItem("Open Paydar / باز کردن", "")
	systray.AddSeparator()
	t.mStatus = systray.AddMenuItem("Disconnected", "")
	t.mStatus.Disable()
	t.mToggle = systray.AddMenuItem("Connect / اتصال", "")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit / خروج", "")
	systray.SetOnTapped(func() { openUI(t.url) })
	t.mu.Lock()
	t.ready = true
	t.mu.Unlock()
	t.update(t.a.Status())
	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				openUI(t.url)
			case <-t.mToggle.ClickedCh:
				if t.a.Status().State == app.Disconnected {
					go t.a.Connect()
				} else {
					go t.a.Disconnect()
				}
			case <-mQuit.ClickedCh:
				t.onQuit()
				return
			}
		}
	}()
}

func (t *trayUI) update(st app.Status) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ready {
		return
	}
	systray.SetIcon(trayIcon(st.State, st.Offline))
	label := "Disconnected / قطع"
	switch st.State {
	case app.Connecting:
		label = "Connecting… / در حال اتصال"
	case app.Connected:
		label = "Connected / متصل: " + st.ServerName
		if st.Offline {
			label = "No internet / اینترنت قطع است"
		}
	}
	t.mStatus.SetTitle(label)
	systray.SetTooltip("Paydar — " + label)
	if st.State == app.Disconnected {
		t.mToggle.SetTitle("Connect / اتصال")
	} else {
		t.mToggle.SetTitle("Disconnect / قطع اتصال")
	}
}
