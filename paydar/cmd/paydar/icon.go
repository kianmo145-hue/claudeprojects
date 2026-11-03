package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"runtime"

	"paydar/internal/app"
)

var (
	colGrey  = color.RGBA{0x8b, 0x9b, 0xab, 0xff}
	colAmber = color.RGBA{0xff, 0xb5, 0x47, 0xff}
	colGreen = color.RGBA{0x2e, 0xe6, 0xa6, 0xff}
	colDark  = color.RGBA{0x0f, 0x17, 0x20, 0xff}
)

// trayIcon draws the shield in the state's colour: grey off, amber connecting/offline, green on
// (with a check mark). Windows wants ICO bytes; other systems take the PNG.
func trayIcon(state app.State, offline bool) []byte {
	c, check := colGrey, false
	switch {
	case state == app.Connecting || state == app.Connected && offline:
		c = colAmber
	case state == app.Connected:
		c, check = colGreen, true
	}
	p := shieldPNG(32, c, check)
	if runtime.GOOS != "windows" {
		return p
	}
	return icoOf(p, 32)
}

// writeAppIcon writes the installer/exe icon (256 px, PNG in ICO).
func writeAppIcon(path string) error {
	return os.WriteFile(path, icoOf(shieldPNG(256, colGreen, true), 256), 0o644)
}

func shieldPNG(n int, c color.RGBA, check bool) []byte {
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	// 4x4 supersampling for smooth edges.
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			hit, mark := 0, 0
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					fx, fy := (float64(x)+(float64(sx)+0.5)/4)/float64(n), (float64(y)+(float64(sy)+0.5)/4)/float64(n)
					if inShield(fx, fy) {
						hit++
						if check && onCheck(fx, fy) {
							mark++
						}
					}
				}
			}
			if hit == 0 {
				continue
			}
			col := c
			if mark*2 > hit {
				col = colDark
			}
			col.A = uint8(255 * hit / 16)
			img.SetRGBA(x, y, col)
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func icoOf(pngBytes []byte, n int) []byte {
	var ico bytes.Buffer
	binary.Write(&ico, binary.LittleEndian, []uint16{0, 1, 1})
	dim := byte(n)
	if n >= 256 {
		dim = 0
	}
	ico.Write([]byte{dim, dim, 0, 0})
	binary.Write(&ico, binary.LittleEndian, []uint16{1, 32})
	binary.Write(&ico, binary.LittleEndian, []uint32{uint32(len(pngBytes)), 22})
	ico.Write(pngBytes)
	return ico.Bytes()
}

func inShield(x, y float64) bool {
	d := math.Abs(x - 0.5)
	if y < 0.55 {
		return d <= 0.36 && y >= 0.08+0.07*d/0.36
	}
	// the lower half narrows to a point at the bottom
	return y <= 0.95 && d <= 0.36*math.Sqrt(math.Max(0, 1-(y-0.55)/0.40))
}

// onCheck: within the stroke of a check mark (two segments).
func onCheck(x, y float64) bool {
	const w = 0.065
	return segDist(x, y, 0.30, 0.50, 0.45, 0.65) < w || segDist(x, y, 0.45, 0.65, 0.71, 0.36) < w
}

func segDist(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	t := math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/(dx*dx+dy*dy)))
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}
