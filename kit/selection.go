package kit

// selection is the shared single-select behaviour of lists, trees and
// tables: one Tab stop (a FocusGroup), click to select, Up/Down/Home/End
// move the selection (selection follows focus) and scroll it into view.
// Multi-select and type-ahead will grow here, once, for all three.
type selection struct {
	group FocusGroup
	items Pool[Clickable]
	cur   int // index of the selected id this frame, -1 if none
}

// handle runs in the handle pass: rows join the group and clicks select.
// It returns each row's clickable, in ids order.
func (s *selection) handle(gtx C, ids []string, selected *string, changed *bool) []*Clickable {
	clicks := make([]*Clickable, len(ids))
	s.cur = -1
	for i, id := range ids {
		c := s.items.Get(id)
		clicks[i] = c
		s.group.Join(c, id == *selected)
		if id == *selected {
			s.cur = i
		}
		if c.Clicked(gtx) && *selected != id {
			*selected, *changed = id, true
			s.cur = i
		}
	}
	s.items.Sweep()
	s.group.Active = *selected
	return clicks
}

// layout lays out w as the group, in a KeyContext named kind, with the
// movement actions declared. extra declares more actions (tree expand).
func (s *selection) layout(gtx C, kind, id string, l *List, ids []string, selected *string, changed *bool, extra func(gtx C), w W) D {
	sel := func(to int) {
		if len(ids) == 0 {
			return
		}
		to = max(0, min(len(ids)-1, to))
		if *selected != ids[to] {
			*selected, *changed = ids[to], true
		}
		s.group.Active = *selected
		l.ScrollIntoView(to)
	}
	return KeyContext(gtx, kind, func(gtx C) D {
		cur := s.cur
		OnAction(gtx, ActMoveUp, func() { sel(max(cur-1, 0)) })
		OnAction(gtx, ActMoveDown, func() { sel(cur + 1) })
		OnAction(gtx, ActFirst, func() { sel(0) })
		OnAction(gtx, ActLast, func() { sel(len(ids) - 1) })
		if extra != nil {
			extra(gtx)
		}
		return s.group.Layout(gtx, id, w)
	})
}

// ScrollIntoView scrolls the least distance that shows item i whole, using
// last frame's layout (rows of similar height). Call it from a handler; the
// next frame shows the result.
func (l *List) ScrollIntoView(i int) {
	p := &l.Position
	l.pending = 0
	if p.Count == 0 {
		p.First, p.Offset = i, 0
		return
	}
	last := p.First + p.Count - 1
	switch {
	case i < p.First || (i == p.First && p.Offset > 0):
		p.First, p.Offset = i, 0
	case i > last || (i == last && p.OffsetLast < 0):
		p.First += i - last
		p.Offset -= p.OffsetLast // OffsetLast < 0: the hidden part of the last row
	default:
		return
	}
	p.BeforeEnd = true
}
