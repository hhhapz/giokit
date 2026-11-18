package kittest

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
)

// Golden comparison tolerances for GPU rounding.
const (
	goldenChannelTolerance = 8 // per-channel difference a pixel may have and still match
	goldenBadPerMille      = 1 // pixels per thousand that may differ before the frame fails
	goldenUpdateEnv        = "KITTEST_UPDATE"
)

// ExpectGolden compares the frame to testdata/golden/<name>.png. Set
// KITTEST_UPDATE=1 to (re)write goldens. Tolerates GPU rounding: fails only if
// more than 0.1% of pixels differ by more than 8 in some colour channel. On a
// mismatch the actual frame is saved to a new temporary file named in the
// failure. Goldens are machine/driver specific; keep them for layout-heavy
// screens, not every test.
func (h *Harness) ExpectGolden(name string) {
	h.tb.Helper()
	h.requireTest("ExpectGolden")
	path := filepath.Join("testdata", "golden", name+".png")
	got := h.Image()
	if os.Getenv(goldenUpdateEnv) != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			h.tb.Fatal(err)
		}
		if err := writePNG(path, got); err != nil {
			h.tb.Fatal(err)
		}
		return
	}
	f, err := os.Open(path)
	if err != nil {
		h.tb.Fatalf("golden %s missing (run with %s=1): %v", path, goldenUpdateEnv, err)
	}
	defer f.Close()
	want, err := png.Decode(f)
	if err != nil {
		h.tb.Fatalf("golden %s: %v", path, err)
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
			if absd(r1>>8, c.R) > goldenChannelTolerance || absd(g1>>8, c.G) > goldenChannelTolerance || absd(b1>>8, c.B) > goldenChannelTolerance {
				bad++
				diff = diff.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if bad*1000 <= goldenBadPerMille*b.Dx()*b.Dy() {
		return
	}
	out, err := saveActual(name, got)
	if err != nil {
		h.tb.Errorf("golden %s: %d pixels differ in %v (saving the actual frame failed: %v)", name, bad, diff, err)
		return
	}
	h.tb.Errorf("golden %s: %d pixels differ in %v (actual saved to %s)", name, bad, diff, out)
}

// saveActual writes a frame that failed its golden comparison to a new,
// uniquely named temporary file and returns its path.
func saveActual(name string, img image.Image) (string, error) {
	f, err := os.CreateTemp("", "kittest-"+ShotName(name)+"-*.png")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		return "", err
	}
	return path, writePNG(path, img)
}

func absd(a uint32, b uint8) uint32 {
	if a > uint32(b) {
		return a - uint32(b)
	}
	return uint32(b) - a
}
