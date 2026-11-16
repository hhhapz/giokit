package kittest

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
)

// ExpectGolden compares the frame to testdata/golden/<name>.png. Set
// KITTEST_UPDATE=1 to (re)write goldens. Tolerates GPU rounding: fails only if
// more than 0.1% of pixels differ by more than 8 per channel. Goldens are
// machine/driver specific; keep them for layout-heavy screens, not every test.
func (h *Harness) ExpectGolden(name string) {
	h.t()
	path := filepath.Join("testdata", "golden", name+".png")
	got := h.Image()
	if os.Getenv("KITTEST_UPDATE") != "" {
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := h.Save(path); err != nil {
			h.tb.Fatal(err)
		}
		return
	}
	f, err := os.Open(path)
	if err != nil {
		h.tb.Fatalf("golden %s missing (run with KITTEST_UPDATE=1): %v", path, err)
	}
	defer f.Close()
	want, err := png.Decode(f)
	if err != nil {
		h.tb.Fatal(err)
	}
	if want.Bounds() != got.Bounds() {
		h.tb.Fatalf("golden %s is %v, frame is %v", name, want.Bounds(), got.Bounds())
	}
	bad, diff := 0, image.Rectangle{}
	b := got.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r1, g1, b1, _ := want.At(x, y).RGBA()
			c := got.RGBAAt(x, y)
			if absd(r1>>8, c.R) > 8 || absd(g1>>8, c.G) > 8 || absd(b1>>8, c.B) > 8 {
				bad++
				diff = diff.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if bad*1000 > b.Dx()*b.Dy() {
		out := filepath.Join(os.TempDir(), "kittest-"+name+".png")
		h.Save(out)
		h.tb.Errorf("golden %s: %d pixels differ in %v (actual saved to %s)", name, bad, diff, out)
	}
}

func absd(a uint32, b uint8) uint32 {
	if a > uint32(b) {
		return a - uint32(b)
	}
	return uint32(b) - a
}
