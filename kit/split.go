package kit

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Split is a resizable two-pane layout. Drag the divider, or focus it (Tab)
// and use the arrow keys. Ratio persists in the struct: save it with your app
// state if you like.
type Split struct {
	Axis  layout.Axis // Horizontal: panes side by side; Vertical: stacked
	Ratio float32     // first pane's share, 0 → 0.5
	Min   unit.Dp     // minimum pane size, 0 → 80dp

	drag  gesture.Drag
	hover gesture.Hover
	grab  float32 // pointer offset inside the handle at press
	start int     // handle position at press (px)
	pos   int     // handle position last frame (px)
	hov   Tween
}

func (t *Theme) Split(gtx C, s *Split, id string, a, b W) D {
	if s.Ratio == 0 {
		s.Ratio = 0.5
	}
	total := gtx.Constraints.Max.X
	if s.Axis == layout.Vertical {
		total = gtx.Constraints.Max.Y
	}
	minPx := gtx.Dp(s.Min)
	if s.Min == 0 {
		minPx = gtx.Dp(80)
	}
	hit := gtx.Dp(16)                      // a usable target; the visible line stays 1dp
	along := func(p image.Point) float32 { // coordinate along the split axis
		if s.Axis == layout.Vertical {
			return float32(p.Y)
		}
		return float32(p.X)
	}
	set := func(px float32) {
		if total <= 0 {
			return
		}
		lo, hi := float32(minPx), float32(total-minPx)
		if hi < lo {
			lo, hi = float32(total)/2, float32(total)/2
		}
		s.Ratio = max(lo, min(hi, px)) / float32(total)
		Notify(gtx)
	}

	// ---- handle input (before drawing) ----
	axis := gesture.Horizontal
	if s.Axis == layout.Vertical {
		axis = gesture.Vertical
	}
	for {
		e, ok := s.drag.Update(gtx.Metric, gtx.Source, axis)
		if !ok {
			break
		}
		// Gio reports a drag relative to the handle as it was at press time
		// (the grab), not as it is now: anchor to the press position.
		p := along(e.Position.Round())
		switch e.Kind {
		case pointer.Press:
			s.grab, s.start = p, s.pos
		case pointer.Drag:
			set(float32(s.start) + p - s.grab + float32(hit)/2)
		}
	}
	for {
		e, ok := gtx.Event(
			key.FocusFilter{Target: s},
			key.Filter{Focus: s, Name: key.NameLeftArrow}, key.Filter{Focus: s, Name: key.NameRightArrow},
			key.Filter{Focus: s, Name: key.NameUpArrow}, key.Filter{Focus: s, Name: key.NameDownArrow},
		)
		if !ok {
			break
		}
		if ke, ok := e.(key.Event); ok && ke.State == key.Press {
			step := float32(total) * 0.05
			switch ke.Name {
			case key.NameLeftArrow, key.NameUpArrow:
				set(s.Ratio*float32(total) - step)
			case key.NameRightArrow, key.NameDownArrow:
				set(s.Ratio*float32(total) + step)
			}
		}
	}
	hovered := s.hover.Update(gtx.Source)

	// ---- layout ----
	first := int(s.Ratio * float32(total))
	first = max(min(first, total-minPx), min(minPx, total/2))
	s.pos = first - hit/2

	pane := func(w W, off, size int) {
		gtx := gtx
		var o image.Point
		if s.Axis == layout.Vertical {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, size))
			o = image.Pt(0, off)
		} else {
			gtx.Constraints = layout.Exact(image.Pt(size, gtx.Constraints.Max.Y))
			o = image.Pt(off, 0)
		}
		st := op.Offset(o).Push(gtx.Ops)
		cl := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
		w(gtx)
		cl.Pop()
		st.Pop()
	}
	line := gtx.Dp(1)
	pane(a, 0, first)
	pane(b, first+line, total-first-line)

	// divider line + highlight while hovered, dragged or focused
	focused := gtx.Focused(s)
	active := float32(0)
	if hovered || s.drag.Dragging() || focused {
		active = 1
	}
	ha := s.hov.To(gtx, active, t.Dur(DurFast))
	lineRect, hitRect := image.Rect(first, 0, first+line, gtx.Constraints.Max.Y), image.Rect(s.pos, 0, s.pos+hit, gtx.Constraints.Max.Y)
	glow := image.Rect(first-1, 0, first+2, gtx.Constraints.Max.Y)
	cursor := pointer.CursorColResize
	if s.Axis == layout.Vertical {
		lineRect, hitRect = image.Rect(0, first, gtx.Constraints.Max.X, first+line), image.Rect(0, s.pos, gtx.Constraints.Max.X, s.pos+hit)
		glow = image.Rect(0, first-1, gtx.Constraints.Max.X, first+2)
		cursor = pointer.CursorRowResize
	}
	paint.FillShape(gtx.Ops, t.Border, clip.Rect(lineRect).Op())
	if ha > 0 {
		c := t.Accent
		if focused {
			c = t.Focus
		}
		paint.FillShape(gtx.Ops, WithAlpha(c, uint8(255*ha)), clip.Rect(glow).Op())
	}
	area := clip.Rect(hitRect).Push(gtx.Ops)
	cursor.Add(gtx.Ops)
	s.drag.Add(gtx.Ops)
	s.hover.Add(gtx.Ops)
	event.Op(gtx.Ops, s)
	Tag(gtx, id) // the drag listens for presses, so this area keeps its semantics
	area.Pop()
	return D{Size: gtx.Constraints.Max}
}
