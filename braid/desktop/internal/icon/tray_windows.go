package icon

// Tray is the icon in the format the tray library expects on this OS.
func Tray(st State, phone bool) []byte { return ICO(st, phone) }

// MenuDot is a menu status light in the format the tray library expects on this OS.
func MenuDot(d Dot) []byte { return DotICO(d) }
