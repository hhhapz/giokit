// Package video draws streaming images (video, camera, remote screens) with
// Video. It needs giokit's Gio fork (third_party/gioui.org) for paint.Stream
// until that lands upstream; kit and kit/ui build on stock Gio without it.
package video

import (
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/hhhapz/giokit/kit"
)

// Fit is how a video frame fills its area.
type Fit uint8

const (
	FitContain Fit = iota // whole frame, aspect kept, bars where it does not fill
	FitCover              // fill the area, aspect kept, edges cropped
	FitFill               // fill the area, aspect ignored
	FitNone               // 1:1 pixels, centered, cropped if larger
)

// Video draws a paint.Stream: video, a camera, a remote screen. It fills its
// constraints and draws the newest frame scaled by Fit; it handles no
// input. Wrap it to add some, and use the state's mapping to convert
// between window and frame coordinates.
//
//	video.Video{ID: "screen", Stream: s, Label: "Tablet screen"}.Layout(gtx, th, &v.screen)
//
// Before the first frame it draws nothing. To redraw on new frames, set
// the stream's OnFrame to invalidate the window (kit.App.Invalidate).
type Video struct {
	ID     string
	Stream *paint.Stream
	Fit    Fit
	Label  string // what the video shows, for screen readers; default th.Strings.Video
}

// VideoState records where the last layout put the frame.
type VideoState struct {
	dst   rectF       // frame's rectangle in widget pixels (may exceed the widget)
	frame image.Point // frame size in pixels
	size  image.Point // widget size
}

// Layout draws the stream's current frame.
func (v Video) Layout(gtx C, th *kit.Theme, st *VideoState) D {
	size := gtx.Constraints.Max
	st.size = size
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	label := v.Label
	if label == "" {
		label = th.Strings.Video
	}
	kit.TagImage(gtx, v.ID, label)
	if v.Stream == nil {
		st.frame, st.dst = image.Point{}, rectF{}
		return D{Size: size}
	}
	fs := v.Stream.Size()
	st.frame = fs
	if fs.X <= 0 || fs.Y <= 0 || size.X <= 0 || size.Y <= 0 {
		st.dst = rectF{}
		return D{Size: size}
	}
	st.dst = place(v.Fit, fs, size)
	scale := f32.Pt(st.dst.Dx()/float32(fs.X), st.dst.Dy()/float32(fs.Y))
	tr := f32.Affine2D{}.Scale(f32.Point{}, scale).Offset(st.dst.Min)
	defer op.Affine(tr).Push(gtx.Ops).Pop()
	defer clip.Rect{Max: fs}.Push(gtx.Ops).Pop()
	v.Stream.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return D{Size: size}
}

// place returns the frame's rectangle within an area of the given size.
func place(fit Fit, frame, area image.Point) rectF {
	fw, fh := float32(frame.X), float32(frame.Y)
	aw, ah := float32(area.X), float32(area.Y)
	var w, h float32
	switch fit {
	case FitFill:
		w, h = aw, ah
	case FitNone:
		w, h = fw, fh
	default:
		s := min(aw/fw, ah/fh)
		if fit == FitCover {
			s = max(aw/fw, ah/fh)
		}
		w, h = fw*s, fh*s
	}
	x, y := (aw-w)/2, (ah-h)/2
	return rectF{f32.Pt(x, y), f32.Pt(x+w, y+h)}
}

// Frame is the frame size at the last layout; zero before the first frame.
func (st *VideoState) Frame() image.Point { return st.frame }

// Content is where the frame was drawn, in the widget's pixel coordinates.
// It extends past the widget for FitCover and an oversized FitNone.
func (st *VideoState) Content() image.Rectangle {
	r := func(v float32) int { return int(math.Round(float64(v))) }
	return image.Rect(r(st.dst.Min.X), r(st.dst.Min.Y), r(st.dst.Max.X), r(st.dst.Max.Y))
}

// ToFrame converts a point in the widget (pixels, as pointer events report
// it) to frame pixels. ok is false outside the visible frame, such as in
// FitContain's bars.
func (st *VideoState) ToFrame(p f32.Point) (f32.Point, bool) {
	if st.dst.Empty() {
		return f32.Point{}, false
	}
	ok := p.X >= max(st.dst.Min.X, 0) && p.X < min(st.dst.Max.X, float32(st.size.X)) &&
		p.Y >= max(st.dst.Min.Y, 0) && p.Y < min(st.dst.Max.Y, float32(st.size.Y))
	q := f32.Pt(
		(p.X-st.dst.Min.X)*float32(st.frame.X)/st.dst.Dx(),
		(p.Y-st.dst.Min.Y)*float32(st.frame.Y)/st.dst.Dy(),
	)
	return q, ok
}

// FromFrame converts frame pixels to widget pixels: where to draw an overlay
// that should sit on a spot in the video.
func (st *VideoState) FromFrame(q f32.Point) f32.Point {
	if st.frame.X == 0 || st.frame.Y == 0 {
		return f32.Point{}
	}
	return f32.Pt(
		st.dst.Min.X+q.X*st.dst.Dx()/float32(st.frame.X),
		st.dst.Min.Y+q.Y*st.dst.Dy()/float32(st.frame.Y),
	)
}

// rectF is a float rectangle (gioui.org/f32 has none in its public API).
type rectF struct{ Min, Max f32.Point }

func (r rectF) Dx() float32 { return r.Max.X - r.Min.X }
func (r rectF) Dy() float32 { return r.Max.Y - r.Min.Y }
func (r rectF) Empty() bool { return r.Dx() <= 0 || r.Dy() <= 0 }

type (
	C = kit.C
	D = kit.D
)
