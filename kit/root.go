package kit

import (
	"image"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
)

// noEdges is a window so large that nothing flips or shifts against it. It
// only enters placement arithmetic; overlay.go's backdropBounds is the
// (smaller) clip rectangle that covers a window.
var noEdges = image.Rect(-1<<20, -1<<20, 1<<20, 1<<20)

// windowBounds is the window's rectangle, or noEdges without a kit.Window.
func windowBounds(gtx layout.Context) image.Rectangle {
	if f := frameOf(gtx); f != nil && f.size != (image.Point{}) {
		return image.Rectangle{Max: f.size}
	}
	return noEdges
}

// Layer draws w above everything else in the window, offset from the
// current origin: it records w and defers the recording with op.Defer.
// Tooltips, toasts and other floating content use it rather than op.Defer
// directly. w is laid out with the current constraints; it gets no input
// priority beyond being drawn last.
func Layer(gtx layout.Context, offset image.Point, w layout.Widget) {
	m := op.Record(gtx.Ops)
	op.Offset(offset).Add(gtx.Ops)
	w(gtx)
	op.Defer(gtx.Ops, m.Stop())
}
