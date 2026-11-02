package platform

import (
	"golang.org/x/sys/windows"
)

const (
	mbRight      = 0x00080000
	mbRTLReading = 0x00100000
	idOK         = 1
)

func box(title, msg string, flags uint32, rtl bool) int32 {
	t, _ := windows.UTF16PtrFromString(msg)
	c, _ := windows.UTF16PtrFromString(title)
	flags |= windows.MB_TOPMOST | windows.MB_SETFOREGROUND
	if rtl {
		flags |= mbRight | mbRTLReading
	}
	r, _ := windows.MessageBox(0, t, c, flags)
	return r
}

// Confirm shows an OK/Cancel question and reports whether OK was clicked.
func Confirm(title, msg string, rtl bool) bool {
	return box(title, msg, windows.MB_OKCANCEL|windows.MB_ICONINFORMATION, rtl) == idOK
}

// Info shows a message with an OK button.
func Info(title, msg string, rtl bool) {
	box(title, msg, windows.MB_OK|windows.MB_ICONINFORMATION, rtl)
}

// UILanguage returns "fa" when Windows' display language is Persian, otherwise "en".
func UILanguage() string {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")
	lang, _, _ := proc.Call()
	if lang&0x3ff == 0x29 { // LANG_PERSIAN
		return "fa"
	}
	return "en"
}
