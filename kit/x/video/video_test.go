package video_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/paint"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/x/video"
	"github.com/hhhapz/giokit/kittest"
)

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}

var (
	red  = color.RGBA{255, 0, 0, 255}
	blue = color.RGBA{0, 0, 255, 255}
)

type videoView struct {
	th     *kit.Theme
	s      *paint.Stream
	st     video.State
	fit    video.Fit
	hidden bool
}

func (v *videoView) Frame(gtx layout.Context) {
	kit.Fill(gtx, v.th.Bg, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		if v.hidden {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
		return video.Video{ID: "screen", Stream: v.s, Fit: v.fit, Label: "Test pattern"}.Layout(gtx, v.th, &v.st)
	})
}

func newVideo(t *testing.T) (*videoView, *kittest.Harness) {
	v := &videoView{th: kit.Dark(), s: paint.NewStream()}
	h := kittest.NewT(t, 200, 200, v.Frame)
	h.Theme = v.th
	return v, h
}

func near(a color.NRGBA, b color.RGBA) bool {
	d := func(x, y uint8) bool { return x > y-8 && x < y+8 || x == y }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B)
}

func TestVideoDrawsNewestFrame(t *testing.T) {
	v, h := newVideo(t)
	h.Settle()
	if _, ok := h.Find("screen"); !ok {
		t.Fatal("video not tagged")
	}
	// 100x50 frame letterboxed into 200x200: rows 50..150.
	released := 0
	v.s.Submit(paint.StreamFrame{Image: solid(100, 50, red), Release: func() { released++ }})
	h.Settle()
	px := h.Size.X / 2
	if c := h.Pixel(image.Pt(px, h.Size.Y/2)); !near(c, red) {
		t.Fatalf("center = %v, want red", c)
	}
	if c := h.Pixel(image.Pt(px, h.Size.Y/8)); near(c, red) {
		t.Fatal("letterbox bar painted red")
	}
	// The texture is reused; the new frame must be uploaded into it.
	v.s.Submit(paint.StreamFrame{Image: solid(100, 50, blue)})
	h.Settle()
	if c := h.Pixel(image.Pt(px, h.Size.Y/2)); !near(c, blue) {
		t.Fatalf("after second frame center = %v, want blue", c)
	}
	if released != 1 {
		t.Fatalf("first frame released %d times, want 1", released)
	}
	// A size change re-creates the texture: 50x100 now fills columns 50..150.
	v.s.Submit(paint.StreamFrame{Image: solid(50, 100, red)})
	h.Settle()
	if c := h.Pixel(image.Pt(h.Size.X/8, h.Size.Y/2)); near(c, red) {
		t.Fatal("pillarbox bar painted red")
	}
	if c := h.Pixel(image.Pt(px, h.Size.Y/8)); !near(c, red) {
		t.Fatalf("top center = %v, want red", c)
	}
	h.ExpectClean()
}

// A stream not drawn for a frame loses its texture to the cache; it must
// come back with the current frame, not a blank.
func TestVideoSurvivesBeingHidden(t *testing.T) {
	v, h := newVideo(t)
	v.s.Submit(paint.StreamFrame{Image: solid(200, 200, blue)})
	h.Settle()
	v.hidden = true
	h.Settle()
	h.Settle()
	v.hidden = false
	h.Settle()
	if c := h.Pixel(image.Pt(h.Size.X/2, h.Size.Y/2)); !near(c, blue) {
		t.Fatalf("after hide/show center = %v, want blue", c)
	}
}

func TestVideoMapping(t *testing.T) {
	v, h := newVideo(t)
	v.s.Submit(paint.StreamFrame{Image: solid(400, 200, red)})
	W, H := float32(h.Size.X), float32(h.Size.Y)
	for _, c := range []struct {
		fit     video.Fit
		content image.Rectangle // in units of the window size / 4
	}{
		{video.FitContain, image.Rect(0, 1, 4, 3)}, // full width, half height, centered
		{video.FitCover, image.Rect(-2, 0, 6, 4)},  // full height, double width, centered
		{video.FitFill, image.Rect(0, 0, 4, 4)},    // the whole area
		{video.FitNone, image.Rect(-2, 0, 6, 4)},   // 400x200 at 1:1 in a 200x200 window, at scale 1
	} {
		v.fit = c.fit
		h.Settle()
		got := v.st.Content()
		if c.fit == video.FitNone && h.Size.X != 200 {
			continue // 1:1 depends on the pixel scale
		}
		want := image.Rect(c.content.Min.X*h.Size.X/4, c.content.Min.Y*h.Size.Y/4, c.content.Max.X*h.Size.X/4, c.content.Max.Y*h.Size.Y/4)
		if got != want {
			t.Errorf("fit %d: content %v, want %v", c.fit, got, want)
		}
		// round trip: the window center is the frame center
		q, ok := v.st.ToFrame(f32.Pt(W/2, H/2))
		if !ok || q != f32.Pt(200, 100) {
			t.Errorf("fit %d: ToFrame(center) = %v %v, want (200,100) true", c.fit, q, ok)
		}
		if p := v.st.FromFrame(q); p != f32.Pt(W/2, H/2) {
			t.Errorf("fit %d: FromFrame = %v", c.fit, p)
		}
	}
	v.fit = video.FitContain
	h.Settle()
	if _, ok := v.st.ToFrame(f32.Pt(W/2, 2)); ok {
		t.Error("a point in the letterbox bar maps into the frame")
	}
}
