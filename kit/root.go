package kit

import (
	"image"

	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/pointer"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
)

// rootTag is the window's area, registered by Window.Begin. Every pointer
// event over the window reaches it (it is the ancestor of every area), in
// window coordinates: popups compare those with their anchor's view of the
// same event to learn where the anchor is, so they can flip and shift at the
// window's edges. Without a kit.Window there is no root, and popups neither
// flip nor shift.
var rootTag = new(int)

// pointerKinds are the pointer events the root and popup anchors watch:
// hover only, since asking for presses would make the areas clickable.
const pointerKinds = pointer.Move | pointer.Enter

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

// anchorOffset learns where an area is in the window: it reads the area's
// pointer events (tag, local coordinates) and matches one with the root's
// view of the same event (window coordinates). ok is false when no pointer
// event reached the area this frame; keep the last good offset.
func anchorOffset(gtx layout.Context, tag event.Tag) (off image.Point, ok bool) {
	f := frameOf(gtx)
	for {
		e, more := gtx.Event(pointer.Filter{Target: tag, Kinds: pointerKinds})
		if !more {
			break
		}
		pe, isPointer := e.(pointer.Event)
		if !isPointer || f == nil {
			continue
		}
		for _, r := range f.pointers {
			if r.at == pe.Time {
				off, ok = r.pos.Sub(pe.Position.Round()), true
			}
		}
	}
	return off, ok
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
