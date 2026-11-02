//go:build !android

package nics

import "syscall"

// ForceEgress is a no-op off Android, so the engine and its tests build on a desktop.
func ForceEgress(int, bool) func(network, address string, c syscall.RawConn) error { return nil }

// PinToHandle is a no-op off Android.
func PinToHandle(uint64) func(network, address string, c syscall.RawConn) error { return nil }
