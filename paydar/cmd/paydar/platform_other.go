//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

func singleInstance() (func(), bool) { return func() {}, true }

func canTUN() bool { return os.Geteuid() == 0 }

func openUI(url string) {
	for _, c := range []string{"xdg-open", "open"} {
		if exec.Command(c, url).Start() == nil {
			return
		}
	}
	fmt.Println(url)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "paydar:", err)
	os.Exit(1)
}

func clearStaleSystemProxy(int) {}
