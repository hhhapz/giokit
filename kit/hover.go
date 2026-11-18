package kit

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// HoverArea tracks whether the pointer is over an area, including its
// children: for a region that reacts to hover without being pressable (a
// table row, a card). Controls get hover from Theme.Press's State instead.
// The zero value is ready to use; it must not be copied after first use.
type HoverArea struct{ g gesture.Hover }

// Layout lays out w as the area, with id on it, passing whether the pointer
// is over it this frame.
func (h *HoverArea) Layout(gtx layout.Context, id string, w func(gtx layout.Context, hovered bool) layout.Dimensions) layout.Dimensions {
	hovered := h.g.Update(gtx.Source) // consume this frame's pointer events first
	m := op.Record(gtx.Ops)
	dims := w(gtx, hovered)
	call := m.Stop()
	area := clip.Rect(image.Rectangle{Max: dims.Size})
	defer area.Push(gtx.Ops).Pop()
	h.g.Add(gtx.Ops)
	// Gio drops semantics from areas whose handler has no click/scroll gesture,
	// so the id goes on a nested, handler-free area of the same size.
	defer area.Push(gtx.Ops).Pop()
	Tag(gtx, id)
	call.Add(gtx.Ops)
	return dims
}
