// Package web holds the UI shared by the Windows and Android apps.
package web

import "embed"

//go:embed index.html app.js style.css icon.svg fonts/*.ttf
var Files embed.FS
