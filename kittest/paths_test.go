package kittest

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/hhhapz/giokit/gio/f32"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"

	"github.com/hhhapz/giokit/kit"
)

// TestPathBuilds: shapes drawn straight into the frame are built every
// frame, while the same shapes recorded in a kit.DrawCache and replayed
// under a moving offset (a scroll) are built once.
func TestPathBuilds(t *testing.T) {
	const shapes = 20
	draw := func(gtx layout.Context) {
		for i := range shapes {
			r := clip.RRect{Rect: image.Rect(10, 10+i*12, 110, 20+i*12), SE: 3, SW: 3, NE: 3, NW: 3}
			paint.FillShape(gtx.Ops, color.NRGBA{R: 200, A: 255}, r.Op(gtx.Ops))
		}
	}
	var cached bool
	var cache kit.DrawCache[int]
	scroll := 0
	h := NewT(t, 200, 300, func(gtx layout.Context) {
		defer op.Offset(image.Pt(0, -scroll)).Push(gtx.Ops).Pop()
		if cached {
			cache.Draw(gtx, 0, draw)
		} else {
			draw(gtx)
		}
	})
	if h == nil {
		return
	}
	frames := func() {
		for range 5 {
			scroll++
			h.Wait(16 * time.Millisecond)
		}
	}
	builds, n := h.PathBuildsDuring(frames)
	if n == 0 || builds < shapes*n {
		t.Fatalf("drawn straight: %d builds over %d frames, want %d each", builds, n, shapes)
	}
	cached = true
	frames() // warm up: the first recording builds once
	h.ExpectPathBuilds(0, frames)
}

// TestScaledImagePaths: an image drawn under a scale (a sprite at a
// fractional zoom) needs no clip path, so moving it every frame builds
// nothing (giokit's patch to boundsForTransformedRect).
func TestScaledImagePaths(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	src := paint.NewImageOp(img)
	x := 0
	h := NewT(t, 200, 200, func(gtx layout.Context) {
		x++
		defer op.Affine(f32.AffineId().Scale(f32.Point{}, f32.Pt(2.5, 2.5)).Offset(f32.Pt(float32(x), 10))).Push(gtx.Ops).Pop()
		src.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	})
	if h == nil {
		return
	}
	h.Strict = false // it moves every frame, input or not
	h.Wait(16 * time.Millisecond)
	h.ExpectPathBuilds(0, func() {
		for range 5 {
			h.Wait(16 * time.Millisecond)
		}
	})
	if c := h.Pixel(image.Pt(x+5, 15)); c.A == 0 {
		t.Fatalf("image not drawn: %v", c)
	}
}
