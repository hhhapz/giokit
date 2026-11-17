package kit

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/op"
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

// Layer draws w above everything else in the window, offset from the
// current origin: tooltips, toasts and other floating content go through it,
// never through op.Defer directly, so the host can later show a layer as an
// OS popup window without the component changing (docs/design/phase1.md).
// w is laid out with the current constraints; it gets no input priority
// beyond being drawn last.
func Layer(gtx C, offset image.Point, w W) {
	m := op.Record(gtx.Ops)
	op.Offset(offset).Add(gtx.Ops)
	w(gtx)
	op.Defer(gtx.Ops, m.Stop())
}
