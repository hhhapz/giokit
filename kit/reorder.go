package kit

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

type reorderHandle struct {
	drag  gesture.Drag
	start float32
}

// ReorderList keeps stable item IDs while rows move. The caller owns Items and
// can persist their order after Changed. Rows have a fixed height so a drag
// maps to an item index without measuring offscreen content.
type ReorderList struct {
	List     AnchoredList
	Items    []Choice
	handles  Pool[reorderHandle]
	up, down Pool[Clickable]
	changed  bool
	from, to int
}

func (r *ReorderList) Changed() (from, to int, ok bool) {
	from, to, ok = r.from, r.to, r.changed
	r.changed = false
	return
}

func (r *ReorderList) Move(from, to int) bool {
	if from < 0 || to < 0 || from >= len(r.Items) || to >= len(r.Items) || from == to {
		return false
	}
	item := r.Items[from]
	if from < to {
		copy(r.Items[from:to], r.Items[from+1:to+1])
	} else {
		copy(r.Items[to+1:from+1], r.Items[to:from])
	}
	r.Items[to] = item
	r.from, r.to, r.changed = from, to, true
	return true
}

// ReorderList draws a scrollable queue. Drag its handle or use the Up/Down
// buttons (including their normal Tab/Enter keyboard behavior). The item
// widget can contain a title, artwork, and secondary metadata.
func (t *Theme) ReorderList(gtx C, r *ReorderList, id string, item func(C, Choice) D) D {
	const rowDp = unit.Dp(56)
	rowPx := max(1, gtx.Dp(rowDp))
	defer r.up.Sweep()
	defer r.down.Sweep()
	defer r.handles.Sweep()
	for i, c := range r.Items {
		if r.up.Get(c.ID).Clicked(gtx) && r.Move(i, i-1) {
			Notify(gtx)
		}
		if r.down.Get(c.ID).Clicked(gtx) && r.Move(i, i+1) {
			Notify(gtx)
		}
		h := r.handles.Get(c.ID)
		for {
			e, ok := h.drag.Update(gtx.Metric, gtx.Source, gesture.Vertical)
			if !ok {
				break
			}
			switch e.Kind {
			case pointer.Press:
				h.start = e.Position.Y
			case pointer.Release:
				shift := int((e.Position.Y - h.start) / float32(rowPx))
				if shift != 0 && r.Move(i, max(0, min(len(r.Items)-1, i+shift))) {
					Notify(gtx)
				}
			}
		}
		for {
			e, ok := gtx.Event(key.FocusFilter{Target: h}, key.Filter{Focus: h, Name: key.NameUpArrow}, key.Filter{Focus: h, Name: key.NameDownArrow})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				d := -1
				if ke.Name == key.NameDownArrow {
					d = 1
				}
				if r.Move(i, i+d) {
					Notify(gtx)
				}
			}
		}
	}
	ids := make([]string, len(r.Items))
	for i := range r.Items {
		ids[i] = r.Items[i].ID
	}
	return t.AnchoredList(gtx, &r.List, id, ids, func(gtx C, i int) D {
		c := r.Items[i]
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = rowPx, rowPx
		return Row(gtx,
			Rigid(func(gtx C) D {
				gtx.Constraints.Min = image.Point{}
				h := r.handles.Get(c.ID)
				sz := image.Pt(gtx.Dp(34), rowPx)
				defer clip.Rect{Max: sz}.Push(gtx.Ops).Pop()
				TagLabel(gtx, Part(id, c.ID, "drag"), "Reorder "+c.Label+"; use up and down arrows")
				semantic.Button.Add(gtx.Ops)
				if gtx.Enabled() {
					h.drag.Add(gtx.Ops)
					event.Op(gtx.Ops, h)
					pointer.CursorGrab.Add(gtx.Ops)
				}
				return layoutCenter(gtx, sz, func(gtx C) D { return IconMenu(gtx, 16, t.Muted) })
			}),
			Flexed(1, func(gtx C) D { gtx.Constraints.Min.Y = 0; return item(gtx, c) }),
			Rigid(func(gtx C) D {
				return t.Disable(gtx, i == 0, func(gtx C) D {
					return t.IconButtonLabel(gtx, r.up.Get(c.ID), Part(id, c.ID, "up"), "Move "+c.Label+" up", IconChevronUp, false)
				})
			}),
			Rigid(func(gtx C) D {
				return t.Disable(gtx, i == len(r.Items)-1, func(gtx C) D {
					return t.IconButtonLabel(gtx, r.down.Get(c.ID), Part(id, c.ID, "down"), "Move "+c.Label+" down", IconChevronDown, false)
				})
			}),
			Rigid(Gap(6, 0)),
		)
	})
}

func layoutCenter(gtx C, size image.Point, w W) D {
	gtx.Constraints.Min, gtx.Constraints.Max = size, size
	return layout.Center.Layout(gtx, w)
}
