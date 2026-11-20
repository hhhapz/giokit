package kit

import (
	"image"
	"image/color"
	"testing"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"
)

// TestDrawCacheRecordsOncePerKey: the drawing runs only when the key
// changes or the cache is invalidated; every other frame replays it.
func TestDrawCacheRecordsOncePerKey(t *testing.T) {
	var c DrawCache[int]
	runs := 0
	draw := func(gtx layout.Context) {
		runs++
		paint.FillShape(gtx.Ops, color.NRGBA{A: 255}, clip.Rect{Max: image.Pt(10, 10)}.Op())
	}
	frame := func(key int) {
		gtx := layout.Context{Ops: new(op.Ops)}
		c.Draw(gtx, key, draw)
	}
	for _, step := range []struct {
		key  int
		want int
	}{{1, 1}, {1, 1}, {1, 1}, {2, 2}, {2, 2}, {1, 3}} {
		frame(step.key)
		if runs != step.want {
			t.Fatalf("after key %d: drew %d times, want %d", step.key, runs, step.want)
		}
	}
	c.Invalidate()
	frame(1)
	if runs != 4 {
		t.Fatalf("after Invalidate: drew %d times, want 4", runs)
	}
}

// TestDrawCacheReplayAllocs: replaying an unchanged recording allocates
// nothing beyond the frame's own op list.
func TestDrawCacheReplayAllocs(t *testing.T) {
	var c DrawCache[int]
	ops := new(op.Ops)
	draw := func(gtx layout.Context) {
		paint.FillShape(gtx.Ops, color.NRGBA{A: 255}, clip.Rect{Max: image.Pt(10, 10)}.Op())
	}
	frame := func() {
		ops.Reset()
		c.Draw(layout.Context{Ops: ops}, 1, draw)
	}
	frame()
	if n := testing.AllocsPerRun(50, frame); n > 0 {
		t.Fatalf("replay allocates %.0f times per frame, want 0", n)
	}
}
