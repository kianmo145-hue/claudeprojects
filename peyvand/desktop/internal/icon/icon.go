// Package icon draws the tray icon and the small status dots in the menu. Everything is drawn in
// code, so there are no image files to ship and every size is sharp.
package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

// State is what the tray icon shows.
type State int

const (
	Off     State = iota // grey: paused
	Single               // amber: on, but only one connection has internet
	Merging              // green to teal: two or more connections merged
)

type rgb struct{ r, g, b float64 }

// Two-colour vertical gradients per state (top, bottom).
var palette = map[State][2]rgb{
	Off:     {{148, 163, 184}, {100, 116, 139}},
	Single:  {{251, 191, 36}, {217, 119, 6}},
	Merging: {{52, 211, 153}, {13, 148, 136}},
}

// Draw renders the badge at size x size: a rounded square with the "peyvand" glyph, three
// strands joining into one. With phone a small phone badge sits in the corner (booster active).
func Draw(st State, phone bool, size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	s := float64(size) / 32 // the design is on a 32-unit grid
	top, bot := palette[st][0], palette[st][1]
	strands := [][4]float64{{8, 7, 16, 17}, {16, 6, 16, 17}, {24, 7, 16, 17}, {16, 17, 16, 26}}
	stroke := 1.6
	if size <= 16 {
		strands = [][4]float64{{8, 7, 16, 17}, {24, 7, 16, 17}, {16, 17, 16, 26}}
		stroke = 2.0
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px, py := (float64(x)+0.5)/s, (float64(y)+0.5)/s
			shape := clamp01(0.5 - roundRectDist(px, py, 1, 1, 31, 31, 8)*s)
			if shape == 0 {
				continue
			}
			t := py / 32
			c := rgb{top.r + (bot.r-top.r)*t, top.g + (bot.g-top.g)*t, top.b + (bot.b-top.b)*t}
			glyph := 0.0
			for _, g := range strands {
				glyph = math.Max(glyph, clamp01((stroke-distToSeg(px, py, g[0], g[1], g[2], g[3]))*s+0.5))
			}
			glyph = math.Max(glyph, clamp01((2.6-math.Hypot(px-16, py-17))*s+0.5)) // the knot
			out := rgb{c.r*(1-glyph) + 255*glyph, c.g*(1-glyph) + 255*glyph, c.b*(1-glyph) + 255*glyph}
			alpha := shape
			if phone {
				// cut-out ring, then a dark rounded phone in the bottom-right corner
				ring := clamp01(0.5 - roundRectDist(px, py, 19, 15, 31.5, 31.5, 3.5)*s)
				body := clamp01(0.5 - roundRectDist(px, py, 21, 17, 29.5, 30, 2)*s)
				if ring > 0 {
					alpha = shape * (1 - ring)
					out = rgb{30, 41, 59}
					if body > 0 {
						alpha = math.Max(alpha, body)
						screen := clamp01(0.5 - roundRectDist(px, py, 22.5, 18.5, 28, 27, 0.8)*s)
						out = rgb{30 + (c.r-30)*screen, 41 + (c.g-41)*screen, 59 + (c.b-59)*screen}
					}
				}
			}
			img.SetNRGBA(x, y, color.NRGBA{uint8(out.r), uint8(out.g), uint8(out.b), uint8(alpha * 255)})
		}
	}
	return img
}

// Dot is a small round status light for menu rows.
type Dot int

const (
	DotUp      Dot = iota // green: has internet
	DotPending            // amber: checking
	DotDown               // red: no internet
	DotIdle               // grey: not in use
)

var dotColors = map[Dot]rgb{DotUp: {34, 197, 94}, DotPending: {245, 158, 11}, DotDown: {239, 68, 68}, DotIdle: {148, 163, 184}}

func drawDot(d Dot, size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	c := dotColors[d]
	r := float64(size) * 0.3
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			a := clamp01(r - math.Hypot(float64(x)+0.5-float64(size)/2, float64(y)+0.5-float64(size)/2) + 0.5)
			if a > 0 {
				img.SetNRGBA(x, y, color.NRGBA{uint8(c.r), uint8(c.g), uint8(c.b), uint8(a * 255)})
			}
		}
	}
	return img
}

// PNG is the tray badge as a 32 px PNG (Linux tray, and the source for the .ico).
func PNG(st State, phone bool) []byte { return encode(Draw(st, phone, 32)) }

// ICO holds the badge at 16, 20, 24, 32, 48 and 256 px, so Windows picks a sharp one for any
// display scaling.
func ICO(st State, phone bool) []byte {
	var imgs [][]byte
	sizes := []int{16, 20, 24, 32, 48, 256}
	for _, n := range sizes {
		imgs = append(imgs, encode(Draw(st, phone, n)))
	}
	return packICO(sizes, imgs)
}

// DotICO is a menu status light (16 px).
func DotICO(d Dot) []byte { return packICO([]int{16}, [][]byte{encode(drawDot(d, 16))}) }

// DotPNG is the same for trays that want PNG.
func DotPNG(d Dot) []byte { return encode(drawDot(d, 16)) }

func encode(img image.Image) []byte {
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func packICO(sizes []int, pngs [][]byte) []byte {
	var ico bytes.Buffer
	binary.Write(&ico, binary.LittleEndian, [3]uint16{0, 1, uint16(len(pngs))})
	off := 6 + 16*len(pngs)
	for i, p := range pngs {
		dim := byte(sizes[i])
		if sizes[i] >= 256 {
			dim = 0 // 0 means 256 in an ICONDIRENTRY
		}
		ico.Write([]byte{dim, dim, 0, 0})
		binary.Write(&ico, binary.LittleEndian, [2]uint16{1, 32})
		binary.Write(&ico, binary.LittleEndian, uint32(len(p)))
		binary.Write(&ico, binary.LittleEndian, uint32(off))
		off += len(p)
	}
	for _, p := range pngs {
		ico.Write(p)
	}
	return ico.Bytes()
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func distToSeg(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	t := clamp01(((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy))
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}

// roundRectDist is the signed distance from (px, py) to a rounded rectangle (negative inside).
func roundRectDist(px, py, x0, y0, x1, y1, r float64) float64 {
	cx, cy := (x0+x1)/2, (y0+y1)/2
	hx, hy := (x1-x0)/2-r, (y1-y0)/2-r
	qx, qy := math.Abs(px-cx)-hx, math.Abs(py-cy)-hy
	outside := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
	return outside + math.Min(math.Max(qx, qy), 0) - r
}
