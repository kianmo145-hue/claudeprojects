//go:build !windows

package icon

// Tray is the icon in the format the tray library expects on this OS: its Linux backend decodes
// PNG but not ICO.
func Tray(st State, phone bool) []byte { return PNG(st, phone) }

// MenuDot is a menu status light in the format the tray library expects on this OS.
func MenuDot(d Dot) []byte { return DotPNG(d) }
