package media

import (
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"

	"github.com/hhhapz/giokit/kittest"
)

// TestZeroCopyRender decodes on the GPU, exports DMA-BUFs, and draws them
// through paint.Stream in the headless renderer (EGL import, no copy). The
// screen must match the software decode of the same frames: this catches
// sRGB handling, channel order, pitch and tiling-modifier mistakes.
func TestZeroCopyRender(t *testing.T) {
	var ref []*image.RGBA
	decodeFile(t, "h264.mp4", Options{Hardware: HardwareOff}, func(fr Frame) bool {
		ref = append(ref, clone(fr.Image))
		return false
	})

	s := paint.NewStream()
	draw := func(gtx layout.Context) {
		defer clip.Rect{Max: s.Size()}.Push(gtx.Ops).Pop()
		s.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	}
	h, err := kittest.New(640, 360, 1, draw)
	if err != nil {
		t.Skip(err)
	}
	defer h.Release()

	released := 0
	i, checked := 0, 0
	_, d, _ := decodeFile(t, "h264.mp4", Options{Hardware: HardwareVAAPI, DMABuf: true}, func(fr Frame) bool {
		k := i
		i++
		if fr.DMABuf == nil {
			return false // surfaces exhausted: a CPU frame, not under test here
		}
		sf := fr.StreamFrame()
		rel := sf.Release
		sf.Release = func() { released++; rel() }
		s.Submit(sf)
		h.Settle()
		if s.ImportState() != paint.ImportSupported {
			t.Skipf("renderer cannot import DMA-BUFs (state %d)", s.ImportState())
		}
		if k == 0 && os.Getenv("GK_DUMP") != "" {
			dump(t, os.Getenv("GK_DUMP")+"/screen.png", h.Image())
			dump(t, os.Getenv("GK_DUMP")+"/ref.png", ref[0])
		}
		if k%7 == 0 { // a spread of frames, key and predicted
			// Compare away from sharp edges: VPP and swscale upsample chroma
			// differently there (both valid). Colour errors show everywhere.
			if os.Getenv("GK_DIAG") != "" && k > 0 && k+1 < len(ref) {
				img := h.Image()
				t.Logf("frame %d: vs k-1 %.2f, k %.2f, k+1 %.2f; whole-frame mean %.2f", k, flatDiff(ref[k-1], img), flatDiff(ref[k], img), flatDiff(ref[k+1], img), meanDiff(ref[k], img))
			}
			// Measured on an RX 6700 XT: 1.4-2.2 in flat areas, with a
			// +0.5-0.8 per-channel bias from VPP's fixed-point matrix; a wrong
			// matrix, range, sRGB decode or channel order gives 15+.
			// GK_DIAG=1 logs neighbours (a wrong frame on screen); GK_DUMP=dir
			// saves frame 0 as screen.png and ref.png.
			if diff := flatDiff(ref[k], h.Image()); diff > 3 {
				t.Fatalf("frame %d: screen differs from software decode by %.2f per channel in flat areas", k, diff)
			}
			checked++
		}
		return true // the stream owns it now
	})
	if err := d.DMABufError(); err != nil {
		t.Skip(err)
	}
	if checked < 5 {
		t.Fatalf("only %d frames checked", checked)
	}
	// Every frame but the one on screen has been released back to the
	// decoder, which is why the decoder never ran out of surfaces.
	if released != i-1 {
		t.Fatalf("%d of %d frames released, want all but the bound one", released, i)
	}
}

func dump(t *testing.T, path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Error(err)
	}
	if err := f.Close(); err != nil {
		t.Error(err)
	}
}

// flatDiff is the mean per-channel difference over pixels at least 3 pixels
// from any edge in ref (a luma step over 40).
func flatDiff(ref, got *image.RGBA) float64 {
	w, h := ref.Rect.Dx(), ref.Rect.Dy()
	luma := func(x, y int) int {
		i := ref.PixOffset(x, y)
		return (int(ref.Pix[i])*3 + int(ref.Pix[i+1])*6 + int(ref.Pix[i+2])) / 10
	}
	edge := make([]bool, w*h)
	for y := 0; y < h-1; y++ {
		for x := 0; x < w-1; x++ {
			l := luma(x, y)
			if abs(l-luma(x+1, y)) > 40 || abs(l-luma(x, y+1)) > 40 {
				for dy := -3; dy <= 3; dy++ {
					for dx := -3; dx <= 3; dx++ {
						if xx, yy := x+dx, y+dy; xx >= 0 && yy >= 0 && xx < w && yy < h {
							edge[yy*w+xx] = true
						}
					}
				}
			}
		}
	}
	var sum, n int
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if edge[y*w+x] {
				continue
			}
			i, j := ref.PixOffset(x, y), got.PixOffset(x, y)
			for c := 0; c < 3; c++ {
				sum += abs(int(ref.Pix[i+c]) - int(got.Pix[j+c]))
				n++
			}
		}
	}
	return float64(sum) / float64(n)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// The solid red clip through the zero-copy path must be red on screen.
func TestZeroCopyColour(t *testing.T) {
	s := paint.NewStream()
	h, err := kittest.New(320, 240, 1, func(gtx layout.Context) {
		defer clip.Rect{Max: s.Size()}.Push(gtx.Ops).Pop()
		s.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	})
	if err != nil {
		t.Skip(err)
	}
	defer h.Release()
	done := false
	_, d, _ := decodeFile(t, "red.mp4", Options{Hardware: HardwareVAAPI, DMABuf: true}, func(fr Frame) bool {
		if done || fr.DMABuf == nil {
			return false
		}
		s.Submit(fr.StreamFrame())
		h.Settle()
		done = true
		return true
	})
	if d.DMABufError() != nil || s.ImportState() != paint.ImportSupported {
		t.Skip("no zero-copy path here")
	}
	if c := h.Pixel(image.Pt(160, 120)); c.R < 245 || c.G > 10 || c.B > 10 {
		t.Fatalf("red renders as %v", c)
	}
}

// A DMA-BUF the renderer cannot import must not crash or draw garbage: the
// stream reports ImportUnsupported (so the producer switches to CPU frames)
// and the frame is released once replaced.
func TestImportFailureDegrades(t *testing.T) {
	s := paint.NewStream()
	h, err := kittest.New(64, 64, 1, func(gtx layout.Context) {
		defer clip.Rect{Max: s.Size()}.Push(gtx.Ops).Pop()
		s.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	})
	if err != nil {
		t.Skip(err)
	}
	defer h.Release()
	released := false
	s.Submit(paint.StreamFrame{
		DMABuf:  &paint.DMABuf{ID: 1, FD: -1, Pitch: 256, Fourcc: fourcc("AB24"), Modifier: 0x00ffffffffffffff, Width: 64, Height: 64},
		Release: func() { released = true },
	})
	h.Settle()
	if s.ImportState() != paint.ImportUnsupported {
		t.Fatalf("import state %d after a bad buffer, want unsupported", s.ImportState())
	}
	// The producer falls back to CPU frames, which draw.
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	s.Submit(paint.StreamFrame{Image: img})
	h.Settle()
	if !released {
		t.Fatal("the failed frame was never released")
	}
	if c := h.Pixel(image.Pt(32, 32)); c.R != 255 || c.G != 255 || c.B != 255 {
		t.Fatalf("CPU frame after a failed import renders %v", c)
	}
}
