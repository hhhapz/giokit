package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/hhhapz/giokit/kit"
)

// The cost model's layout half: building one frame's ops (input handling,
// layout, recording; no rendering) for a whole page on the UI goroutine.
// Rendering is the GPU's (see internal/bench). Run:
//
//	go test ./examples/gallery -run '^$' -bench 'Layout|Frame' -benchmem
func benchLayout(b *testing.B, page string) {
	g := newGallery()
	g.page = page
	var win kit.Window
	var ops op.Ops
	start := time.Unix(0, 0)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		ops.Reset()
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 2, PxPerSp: 2},
			Constraints: layout.Exact(image.Pt(2200, 1440)),
			Now:         start.Add(time.Duration(i) * 16 * time.Millisecond),
		}
		gtx = win.Begin(gtx)
		g.Frame(gtx)
		win.End(gtx)
	}
}

// BenchmarkLayout is the layout half of BenchmarkFrame (which also renders
// and routes input through kittest), page by page.
func BenchmarkLayout(b *testing.B) {
	for _, p := range pages {
		b.Run(p.id, func(b *testing.B) { benchLayout(b, p.id) })
	}
}
