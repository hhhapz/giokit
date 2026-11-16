package kit

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/op/clip"
)

// rootTag marks the window's area, so popups can find the window edges (via
// the patched Source.Bounds) to flip and shift at them.
var rootTag = new(int)

// Root registers the window's bounds. kit.App and kittest call it at the
// start of every frame; call it yourself first thing if you drive Gio's
// window loop directly. Without it, popups never flip or shift.
func Root(gtx C) {
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, rootTag)
}

// windowBounds is the window's rectangle, or a huge one when Root wasn't
// called (no edges: nothing flips).
func windowBounds(gtx C) image.Rectangle {
	if r, ok := gtx.Source.Bounds(rootTag); ok {
		return r
	}
	return image.Rect(-1<<20, -1<<20, 1<<20, 1<<20)
}
