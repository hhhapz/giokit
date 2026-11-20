package ui

import (
	"fmt"
	"image"
	"math"

	"github.com/hhhapz/giokit/gio/gesture"
	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/io/pointer"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/unit"

	"github.com/hhhapz/giokit/kit"
)

// ReorderList is a scrollable queue whose rows can be reordered: drag a
// row's handle (ID/<item>/drag; Up/Down while it is focused) or use its
// buttons (ID/<item>/up, ID/<item>/down). Rows have a fixed height, so a
// drag maps to an index without measuring offscreen content. Apply the
// moves Update reports to your items (Move does it for a slice).
type ReorderList struct {
	ID string // the list's id; see the type doc for the row parts
	// Items are the rows, in order. Their IDs must be unique and non-empty.
	Items []Choice
	// Item, if set, draws an item's content: title, artwork, metadata. It
	// gets the row's fixed height. Without it the row shows the label.
	Item func(gtx layout.Context, c Choice) layout.Dimensions
}

// ReorderListState is a ReorderList's scroll, drag and button state. The
// zero value is ready; it must not be copied after first use.
type ReorderListState struct {
	// List is the scroll position, anchored to the first visible item's
	// ID so a move does not make the view jump.
	List     kit.AnchoredList
	handles  kit.Pool[reorderHandle]
	up, down kit.Pool[kit.Clickable]
	ids      []string // the item IDs as last laid out
	rowPx    int      // the row height last laid out, in pixels
}

// reorderMove is a request to move the item at index from to index to.
type reorderMove struct {
	from, to int
}

// reorderHandle is a row's drag handle: a vertical drag gesture and a key
// focus target.
type reorderHandle struct {
	drag  gesture.Drag
	start float32 // the pointer's y at the press, in pixels
}

const (
	// reorderRow is a queue row's height: a title over a line of metadata.
	reorderRow = unit.Dp(56)
	// reorderHandleWidth is the width of a row's drag handle.
	reorderHandleWidth = unit.Dp(34)
)

// Update reports a move requested since the last call: the item at index
// from moves to index to, shifting the items between. Apply it to your
// items in the handle pass, before Layout.
//
// A button or arrow key moves an item one place. A drag moves it by the
// rows the pointer travelled between press and release, rounded to the
// nearest row and clamped to the list: past half a row it moves.
//
// One move a frame: every request is computed against the order as last
// laid out, so any other request in the same frame is dropped.
func (s *ReorderListState) Update(gtx layout.Context) (from, to int, ok bool) {
	var move reorderMove
	moved := false
	request := func(m reorderMove) {
		if !moved {
			move, moved = m, true
		}
	}
	n := len(s.ids)
	for i, id := range s.ids {
		if s.up.Get(id).Clicked(gtx) && i > 0 {
			request(reorderMove{from: i, to: i - 1})
		}
		if s.down.Get(id).Clicked(gtx) && i < n-1 {
			request(reorderMove{from: i, to: i + 1})
		}
		h := s.handles.Get(id)
		if to, ok := s.dragged(gtx, h, i); ok {
			request(reorderMove{from: i, to: to})
		}
		if to, ok := s.keyed(gtx, h, i); ok {
			request(reorderMove{from: i, to: to})
		}
	}
	if !moved {
		return 0, 0, false
	}
	kit.Notify(gtx)
	return move.from, move.to, true
}

// dragged runs handle h's drag events for the item at index i and reports
// where a released drag moves it.
func (s *ReorderListState) dragged(gtx layout.Context, h *reorderHandle, i int) (to int, ok bool) {
	for {
		e, more := h.drag.Update(gtx.Metric, gtx.Source, gesture.Vertical)
		if !more {
			return to, ok
		}
		switch e.Kind {
		case pointer.Press:
			h.start = e.Position.Y
		case pointer.Release:
			if s.rowPx <= 0 {
				continue
			}
			rows := int(math.Round(float64((e.Position.Y - h.start) / float32(s.rowPx))))
			if t := max(0, min(len(s.ids)-1, i+rows)); t != i {
				to, ok = t, true
			}
		}
	}
}

// keyed runs handle h's Up and Down keys for the item at index i and
// reports where they move it.
func (s *ReorderListState) keyed(gtx layout.Context, h *reorderHandle, i int) (to int, ok bool) {
	for {
		e, more := gtx.Event(
			key.FocusFilter{Target: h},
			key.Filter{Focus: h, Name: key.NameUpArrow},
			key.Filter{Focus: h, Name: key.NameDownArrow},
		)
		if !more {
			return to, ok
		}
		ke, isKey := e.(key.Event)
		if !isKey || ke.State != key.Press {
			continue
		}
		t := i - 1
		if ke.Name == key.NameDownArrow {
			t = i + 1
		}
		if t >= 0 && t < len(s.ids) {
			to, ok = t, true
		}
	}
}

// Move moves s[from] to index to, shifting the items between. It reports
// false (and changes nothing) for an out-of-range or no-op move.
func Move[T any](s []T, from, to int) bool {
	if from < 0 || to < 0 || from >= len(s) || to >= len(s) || from == to {
		return false
	}
	it := s[from]
	if from < to {
		copy(s[from:to], s[from+1:to+1])
	} else {
		copy(s[to+1:from+1], s[to:from])
	}
	s[to] = it
	return true
}

// Layout draws the visible rows at a fixed height, filling the available
// size, each with its drag handle and move buttons. It remembers the order
// and row height for s.Update, which turns drags, keys and button clicks
// into a move; Layout itself never reorders Items.
func (rl ReorderList) Layout(gtx layout.Context, th *kit.Theme, s *ReorderListState) layout.Dimensions {
	rowPx := max(1, gtx.Dp(reorderRow))
	s.rowPx = rowPx
	s.ids = s.ids[:0]
	for _, c := range rl.Items {
		s.ids = append(s.ids, c.ID)
	}
	defer s.up.Sweep()
	defer s.down.Sweep()
	defer s.handles.Sweep()
	return th.AnchoredList(gtx, &s.List, rl.ID, s.ids, func(gtx layout.Context, i int) layout.Dimensions {
		c := rl.Items[i]
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = rowPx, rowPx
		return kit.Row(gtx,
			kit.Rigid(func(gtx layout.Context) layout.Dimensions { return rl.handle(gtx, th, s, c, rowPx) }),
			kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = 0
				return rl.content(gtx, th, c)
			}),
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return th.Disable(gtx, i == 0, func(gtx layout.Context) layout.Dimensions {
					return IconButton{ID: kit.Part(rl.ID, c.ID, "up"), Icon: kit.IconChevronUp, Label: fmt.Sprintf(th.Strings.MoveUp, c.Label)}.Layout(gtx, th, s.up.Get(c.ID))
				})
			}),
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return th.Disable(gtx, i == len(rl.Items)-1, func(gtx layout.Context) layout.Dimensions {
					return IconButton{ID: kit.Part(rl.ID, c.ID, "down"), Icon: kit.IconChevronDown, Label: fmt.Sprintf(th.Strings.MoveDown, c.Label)}.Layout(gtx, th, s.down.Get(c.ID))
				})
			}),
			// Clear the overlay scrollbar's track, which does not scale
			// with density, so it never covers the buttons.
			kit.Rigid(kit.Gap(th.HitSize, 0)),
		)
	})
}

// content draws a row's content: Item, or the label centred vertically.
func (rl ReorderList) content(gtx layout.Context, th *kit.Theme, c Choice) layout.Dimensions {
	if rl.Item != nil {
		return rl.Item(gtx, c)
	}
	return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return th.Text(gtx, c.Label, th.Type.Body, th.Fg, kit.Regular)
	})
}

// handle is a row's drag handle: a focusable button that drags vertically,
// with a focus ring while focused.
func (rl ReorderList) handle(gtx layout.Context, th *kit.Theme, s *ReorderListState, c Choice, rowPx int) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	h := s.handles.Get(c.ID)
	id := kit.Part(rl.ID, c.ID, "drag")
	sz := image.Pt(gtx.Dp(reorderHandleWidth), rowPx)
	defer clip.Rect{Max: sz}.Push(gtx.Ops).Pop()
	kit.TagLabel(gtx, id, fmt.Sprintf(th.Strings.Reorder, c.Label))
	kit.FocusID(gtx, h, id)
	describe(gtx, kit.RoleButton, "") // labelled by TagLabel
	if gtx.Enabled() {
		h.drag.Add(gtx.Ops)
		event.Op(gtx.Ops, h)
		pointer.CursorGrab.Add(gtx.Ops)
	}
	gtx.Constraints.Min, gtx.Constraints.Max = sz, sz
	surf := th.Styled(kit.Surface{Radius: th.Radius})
	if gtx.Focused(h) { // only the keyboard focuses a handle: the ring always shows
		surf.Focus = th.Focus
	}
	return surf.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return kit.IconMenu(gtx, th.IconSizeLg, th.Muted) })
	})
}
