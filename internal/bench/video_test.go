package bench

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/hhhapz/giokit/kittest"
)

// videoMode is how a video benchmark puts a frame on screen.
type videoMode int

const (
	streamNew  videoMode = iota // submit a new frame to a paint.Stream each UI frame
	streamSame                  // redraw the stream's current frame
	imageOpNew                  // a new paint.ImageOp each UI frame
)

// frames makes two distinct w×h frames to alternate between.
func frames(w, h int) [2]*image.RGBA {
	var fs [2]*image.RGBA
	for k := range fs {
		fs[k] = image.NewRGBA(image.Rect(0, 0, w, h))
		for i := range fs[k].Pix {
			fs[k].Pix[i] = byte(i*7 + k*91)
		}
	}
	return fs
}

// runVideo draws a full-window w×h video frame each UI frame in mode: the
// video baselines.
//
//	go test -run x -bench Video -benchtime 200x ./internal/bench
//
// StreamNew uploads into one reused texture (paint.Stream); ImageOpNew is
// the old way (an ImageOp per frame: new texture, upload, mipmaps);
// StreamSame redraws without a new frame (the cost of the redraw alone).
func runVideo(b *testing.B, w, h int, mode videoMode) {
	fs := frames(w, h)
	s := paint.NewStream()
	n := 0
	draw := func(gtx layout.Context) {
		img := fs[n%2]
		switch mode {
		case streamNew:
			s.Submit(paint.StreamFrame{Image: img})
			fallthrough
		case streamSame:
			defer clip.Rect{Max: img.Rect.Size()}.Push(gtx.Ops).Pop()
			s.Add(gtx.Ops)
		case imageOpNew:
			defer clip.Rect{Max: img.Rect.Size()}.Push(gtx.Ops).Pop()
			paint.NewImageOp(img).Add(gtx.Ops)
		}
		paint.PaintOp{}.Add(gtx.Ops)
		n++
	}
	if mode == streamSame {
		s.Submit(paint.StreamFrame{Image: fs[0]})
	}
	hn, err := kittest.New(w, h, 1, draw)
	if err != nil {
		b.Skip(err)
	}
	defer hn.Release()
	hn.Settle()
	measureFrames(b, hn)
}

func BenchmarkVideo1080StreamNew(b *testing.B)  { runVideo(b, 1920, 1080, streamNew) }
func BenchmarkVideo1080ImageOpNew(b *testing.B) { runVideo(b, 1920, 1080, imageOpNew) }
func BenchmarkVideo1080StreamSame(b *testing.B) { runVideo(b, 1920, 1080, streamSame) }
func BenchmarkVideoTabStreamNew(b *testing.B)   { runVideo(b, 2960, 1848, streamNew) }
func BenchmarkVideoTabImageOpNew(b *testing.B)  { runVideo(b, 2960, 1848, imageOpNew) }
func BenchmarkVideoTabStreamSame(b *testing.B)  { runVideo(b, 2960, 1848, streamSame) }
