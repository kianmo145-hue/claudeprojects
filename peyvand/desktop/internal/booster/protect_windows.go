package booster

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// protect encrypts a pairing key with DPAPI for this Windows user.
func protect(b []byte) ([]byte, error) {
	in := windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}

func unprotect(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, windows.ERROR_INVALID_DATA
	}
	in := windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}

// allowDiscovery lets the phones' announcements through Windows Firewall (the tray is elevated).
// The rule is per program, so it only opens what this exe listens on.
func allowDiscovery() {
	run := func(args ...string) error {
		cmd := exec.Command("netsh", args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd.Run()
	}
	if run("advfirewall", "firewall", "show", "rule", "name=Peyvand booster") == nil {
		return
	}
	run("advfirewall", "firewall", "add", "rule", "name=Peyvand booster", "dir=in", "action=allow",
		"protocol=UDP", "localport=47800", "profile=private,domain", "program="+exePath())
}
