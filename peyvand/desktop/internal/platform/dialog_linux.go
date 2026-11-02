package platform

import (
	"os"
	"os/exec"
	"strings"
)

// Confirm asks with zenity when it is installed; without it the answer is yes, because pairing
// is still approved on the phone.
func Confirm(title, msg string, rtl bool) bool {
	if p, err := exec.LookPath("zenity"); err == nil {
		return exec.Command(p, "--question", "--title="+title, "--text="+msg).Run() == nil
	}
	return true
}

// Info shows a message (zenity, else a notification).
func Info(title, msg string, rtl bool) {
	if p, err := exec.LookPath("zenity"); err == nil {
		exec.Command(p, "--info", "--title="+title, "--text="+msg).Run()
		return
	}
	if p, err := exec.LookPath("notify-send"); err == nil {
		exec.Command(p, "--app-name=peyvand", title, msg).Run()
	}
}

// UILanguage returns "fa" for a Persian locale, otherwise "en".
func UILanguage() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			if strings.HasPrefix(v, "fa") {
				return "fa"
			}
			return "en"
		}
	}
	return "en"
}
