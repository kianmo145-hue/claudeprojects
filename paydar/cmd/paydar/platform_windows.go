package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func singleInstance() (release func(), first bool) {
	name, _ := windows.UTF16PtrFromString(`Local\Paydar-SingleInstance`)
	h, err := windows.CreateMutex(nil, false, name)
	if err == windows.ERROR_ALREADY_EXISTS {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return func() {}, false
	}
	return func() { windows.CloseHandle(h) }, true
}

// canTUN: the TUN adapter and routes need an elevated process.
func canTUN() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// openUI shows the UI as an app window (Edge --app), or in the default browser.
func openUI(url string) {
	for _, base := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LocalAppData")} {
		if base == "" {
			continue
		}
		edge := filepath.Join(base, `Microsoft\Edge\Application\msedge.exe`)
		if _, err := os.Stat(edge); err == nil {
			profile := filepath.Join(dataDir(), "ui-profile")
			cmd := exec.Command(edge, "--app="+url, "--user-data-dir="+profile, "--no-first-run", "--window-size=480,820")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			if cmd.Start() == nil {
				return
			}
		}
	}
	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func fatal(err error) {
	title, _ := windows.UTF16PtrFromString("Paydar")
	msg, _ := windows.UTF16PtrFromString(err.Error())
	windows.MessageBox(0, msg, title, windows.MB_ICONERROR)
	os.Exit(1)
}

var (
	wininet               = windows.NewLazySystemDLL("wininet.dll")
	procInternetSetOption = wininet.NewProc("InternetSetOptionW")
)

// clearStaleSystemProxy turns off a system proxy that points at our own port (left by a crash).
func clearStaleSystemProxy(port int) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	enabled, _, _ := k.GetIntegerValue("ProxyEnable")
	server, _, _ := k.GetStringValue("ProxyServer")
	if enabled == 0 || !strings.Contains(server, "127.0.0.1:"+strconv.Itoa(port)) {
		return
	}
	k.SetDWordValue("ProxyEnable", 0)
	const settingsChanged, refresh = 39, 37
	procInternetSetOption.Call(0, settingsChanged, 0, 0)
	procInternetSetOption.Call(0, refresh, 0, 0)
}
