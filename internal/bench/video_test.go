package bench

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/hhhapz/giokit/kittest"
)

// Video baselines: a full-window video frame per UI frame.
//
//	go test -run x -bench Video -benchtime 200x ./internal/bench
//
// StreamNew uploads into one reused texture (paint.Stream); ImageOpNew is
// the old way (an ImageOp per frame: new texture, upload, mipmaps);
// StreamSame redraws without a new frame (the cost of the redraw alone).

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

func runVideo(b *testing.B, w, h int, mode string) {
	fs := frames(w, h)
	s := paint.NewStream()
	n := 0
	draw := func(gtx layout.Context) {
		img := fs[n%2]
		switch mode {
		case "stream":
			s.Submit(paint.StreamFrame{Image: img})
			fallthrough
		case "same":
			defer clip.Rect{Max: img.Rect.Size()}.Push(gtx.Ops).Pop()
			s.Add(gtx.Ops)
		case "imageop":
			defer clip.Rect{Max: img.Rect.Size()}.Push(gtx.Ops).Pop()
			paint.NewImageOp(img).Add(gtx.Ops)
		}
		paint.PaintOp{}.Add(gtx.Ops)
		n++
	}
	if mode == "same" {
		s.Submit(paint.StreamFrame{Image: fs[0]})
	}
	hn, err := kittest.New(w, h, 1, draw)
	if err != nil {
		b.Skip(err)
	}
	defer hn.Release()
	hn.Settle()
	b.ResetTimer()
	t0 := time.Now()
	for i := 0; i < b.N; i++ {
		hn.Settle()
	}
	_ = hn.Pixel(image.Pt(1, 1)) // wait for the GPU
	b.ReportMetric(float64(time.Since(t0).Microseconds())/float64(b.N)/1000, "ms/frame")
}

func BenchmarkVideo1080StreamNew(b *testing.B)  { runVideo(b, 1920, 1080, "stream") }
func BenchmarkVideo1080ImageOpNew(b *testing.B) { runVideo(b, 1920, 1080, "imageop") }
func BenchmarkVideo1080StreamSame(b *testing.B) { runVideo(b, 1920, 1080, "same") }
func BenchmarkVideoTabStreamNew(b *testing.B)   { runVideo(b, 2960, 1848, "stream") }
func BenchmarkVideoTabImageOpNew(b *testing.B)  { runVideo(b, 2960, 1848, "imageop") }
func BenchmarkVideoTabStreamSame(b *testing.B)  { runVideo(b, 2960, 1848, "same") }
