package kit

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// rootTag is the window's area. Every pointer event over the window reaches
// it (it is the ancestor of every area), in window coordinates: popups
// compare those with their anchor's view of the same event to learn where
// the anchor is, so they can flip and shift at the window's edges.
var rootTag = new(int)

// pointerKinds are the pointer events the root and popup anchors watch:
// hover only, since asking for presses would make the areas clickable.
const pointerKinds = pointer.Move | pointer.Enter

// Root registers the window's area. kit.Window.Begin calls it (kit.App and
// kittest use a Window); call it first thing only if you drive Gio's window
// loop without one. Without it, popups never flip or shift.
func Root(gtx C) {
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, rootTag)
}

// noEdges is a window so large that nothing flips or shifts against it.
var noEdges = image.Rect(-1<<20, -1<<20, 1<<20, 1<<20)

// windowBounds is the window's rectangle, or noEdges without a kit.Window.
func windowBounds(gtx C) image.Rectangle {
	if f := frameOf(gtx); f != nil && f.size != (image.Point{}) {
		return image.Rectangle{Max: f.size}
	}
	return noEdges
}

// anchorOffset learns where an area is in the window: it reads the area's
// pointer events (tag, local coordinates) and matches one with the root's
// view of the same event (window coordinates). ok is false when no pointer
// event reached the area this frame; keep the last good offset.
func anchorOffset(gtx C, tag event.Tag) (off image.Point, ok bool) {
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
// current origin: tooltips, toasts and other floating content go through it,
// never through op.Defer directly, so the host can later show a layer as an
// OS popup window without the component changing.
// w is laid out with the current constraints; it gets no input priority
// beyond being drawn last.
func Layer(gtx C, offset image.Point, w W) {
	m := op.Record(gtx.Ops)
	op.Offset(offset).Add(gtx.Ops)
	w(gtx)
	op.Defer(gtx.Ops, m.Stop())
}
