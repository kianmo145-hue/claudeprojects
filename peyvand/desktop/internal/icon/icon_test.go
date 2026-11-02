package icon

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"os"
	"testing"
)

func TestICOHasEverySize(t *testing.T) {
	b := ICO(Merging, true)
	if n := int(b[4]) | int(b[5])<<8; n != 6 {
		t.Fatalf("%d images, want 6", n)
	}
	if _, err := png.Decode(bytes.NewReader(PNG(Off, false))); err != nil {
		t.Fatal(err)
	}
}

// PEYVAND_ICON_PREVIEW=file.png writes every state at 16/32/64 px side by side.
func TestPreview(t *testing.T) {
	out := os.Getenv("PEYVAND_ICON_PREVIEW")
	if out == "" {
		t.Skip()
	}
	sizes := []int{16, 32, 64, 128}
	canvas := image.NewNRGBA(image.Rect(0, 0, 6*140, 4*140))
	col := 0
	for _, st := range []State{Off, Single, Merging} {
		for _, ph := range []bool{false, true} {
			for row, n := range sizes {
				img := Draw(st, ph, n)
				r := image.Rect(col*140+10, row*140+10, col*140+10+n, row*140+10+n)
				draw.Draw(canvas, r, img, image.Point{}, draw.Over)
			}
			col++
		}
	}
	f, _ := os.Create(out)
	defer f.Close()
	png.Encode(f, canvas)
}
