//go:build !windows

package booster

// On Linux (used for development and tests) the file is only readable by its owner (0600).
func protect(b []byte) ([]byte, error)   { return append([]byte(nil), b...), nil }
func unprotect(b []byte) ([]byte, error) { return append([]byte(nil), b...), nil }
func allowDiscovery()                    {}
