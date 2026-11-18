package ui

import (
	"slices"
	"strings"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/hhhapz/giokit/kit"
)

// Choice is one stable item of a list: ID never changes, Label is shown
// and spoken.
type Choice struct {
	ID    string // unique within its list, non-empty and stable across frames
	Label string // the visible and spoken text; may repeat or be translated
}

// SelectionEvent is a change reported by the Update of a SelectableList,
// Tree or DataTable: a click or key chose a new item, or (with Multiple) a
// new chosen set. Apply it to the props you pass next frame.
type SelectionEvent struct {
	// Selected is the selected item; with Multiple, the focused one. It is
	// set on every event, changed or not.
	Selected string
	// Chosen is a Multiple control's chosen set after the change: a new map
	// the control does not keep, or the props' own Chosen when only Selected
	// changed. It is nil for single selection.
	Chosen map[string]bool
}

// selection is the shared selection behaviour of lists, trees and tables:
// one Tab stop (a FocusGroup), click to select, Up/Down/Home/End move the
// selection (selection follows focus) and scroll it into view, typing
// jumps to a matching label, and optionally a multiple selection.
//
// It is controlled: the selected id and chosen set come in through props
// at each layout, and what clicks and keys choose waits in change until
// Update reports it. Row ids must be non-empty.
type selection struct {
	group kit.FocusGroup
	items kit.Pool[kit.Clickable] // row clickables, by row id

	// The rows and props as last laid out. Layout refills the rows each
	// frame; the next frame's Update reads them for clicks.
	ids      []string
	labels   []string // spoken names, parallel to ids, for type-ahead
	selected string
	multi    bool
	chosen   map[string]bool

	// cursor is the index of the row key moves start from: the selected row
	// at layout, then wherever each key of the frame's batch moved it.
	cursor int
	// anchor is the row a Shift range grows from.
	anchor string
	change selectionChange

	typed   string         // type-ahead prefix
	typedAt time.Time      // when the prefix last grew
	keys    []event.Filter // type-ahead keys on the group's focus, built once
}

// selectionChange is what clicks and keys chose since the last Update.
type selectionChange struct {
	selected string          // the row to select; "" when none was chosen
	chosen   map[string]bool // the new chosen set; nil when unchanged
}

// selectionView is the per-frame input of selection.layout, besides the
// rows (setRows).
type selectionView struct {
	context  string    // the KeyContext name: "list", "tree", "table"
	id       string    // the focus group's id
	list     *kit.List // scrolled to keep the cursor in view
	selected string
	multi    bool
	chosen   map[string]bool
	// actions, if set, declares more actions in the group's KeyContext
	// (a tree's expand and collapse).
	actions func(gtx layout.Context)
}

// setRows replaces the rows with n rows; row(i) returns row i's id and
// label. The buffers are reused from frame to frame. The cursor keeps its
// index, so a tree that expands the cursor's node mid-batch can refill the
// rows without moving it.
func (s *selection) setRows(n int, row func(i int) (id, label string)) {
	s.ids, s.labels = s.ids[:0], s.labels[:0]
	for i := range n {
		id, label := row(i)
		s.ids = append(s.ids, id)
		s.labels = append(s.labels, label)
	}
}

// update runs last frame's row clicks and reports the change chosen since
// the last call (handle pass).
//
// Rows join the focus group here, before Clicked, so a click focuses the
// group rather than the row; layout joins them again with the new
// selection as the active row (FocusGroup clears membership each frame).
func (s *selection) update(gtx layout.Context) (SelectionEvent, bool) {
	for _, id := range s.ids {
		c := s.items.Get(id)
		s.group.Join(c, id == s.selected)
		if !c.Clicked(gtx) {
			continue
		}
		if s.multi {
			s.choose(id, c.Modifiers())
		}
		s.change.selected = id
	}
	return s.report()
}

// report returns and clears the pending change. A move that ended where it
// started (Down then Up in one frame) is no change.
func (s *selection) report() (SelectionEvent, bool) {
	c := s.change
	s.change = selectionChange{}
	moved := c.selected != "" && c.selected != s.selected
	if !moved && c.chosen == nil {
		return SelectionEvent{}, false
	}
	if moved {
		s.selected = c.selected
	}
	if c.chosen != nil {
		s.chosen = c.chosen
	}
	e := SelectionEvent{Selected: s.selected}
	if s.multi {
		e.Chosen = s.chosen
	}
	return e, true
}

// choose updates the multiple selection for a click on id: alone, it
// selects just id; with Ctrl/Cmd it toggles id; with Shift it selects the
// range from the anchor.
func (s *selection) choose(id string, mods key.Modifiers) {
	set := map[string]bool{}
	switch {
	case mods.Contain(key.ModShift) && s.anchor != "":
		s.addRange(set, s.anchor, id)
	case mods.Contain(key.ModShortcut):
		for k, v := range s.chosen {
			set[k] = v
		}
		set[id] = !set[id]
		if !set[id] {
			delete(set, id)
		}
		s.anchor = id
	default:
		set[id] = true
		s.anchor = id
	}
	s.change.chosen = set
}

// addRange adds the ids from a to b (either order) to set.
func (s *selection) addRange(set map[string]bool, a, b string) {
	i, j := slices.Index(s.ids, a), slices.Index(s.ids, b)
	if i < 0 || j < 0 {
		return
	}
	for k := min(i, j); k <= max(i, j); k++ {
		set[s.ids[k]] = true
	}
}

// focusRow moves the cursor and the selection to row i, clamped to the
// rows, and scrolls it into view. A multiple selection's chosen set is
// left alone.
func (s *selection) focusRow(l *kit.List, i int) {
	if len(s.ids) == 0 {
		return
	}
	s.cursor = max(0, min(len(s.ids)-1, i))
	s.change.selected = s.ids[s.cursor]
	s.group.Active = s.ids[s.cursor]
	l.ScrollIntoView(s.cursor)
}

// moveTo is a plain move to row i: with multiple selection it also
// chooses that row alone and anchors ranges there.
func (s *selection) moveTo(l *kit.List, i int) {
	s.focusRow(l, i)
	if !s.multi || len(s.ids) == 0 {
		return
	}
	s.anchor = s.ids[s.cursor]
	s.change.chosen = map[string]bool{s.anchor: true}
}

// extendTo moves to row i and chooses the range from the anchor to it
// (from the row it left, when there is no anchor yet).
func (s *selection) extendTo(l *kit.List, i int) {
	if len(s.ids) == 0 {
		return
	}
	if s.anchor == "" && s.cursor >= 0 {
		s.anchor = s.ids[s.cursor]
	}
	s.focusRow(l, i)
	if s.anchor == "" {
		s.anchor = s.ids[s.cursor]
	}
	set := map[string]bool{}
	s.addRange(set, s.anchor, s.ids[s.cursor])
	s.change.chosen = set
}

// chooseAll chooses every row.
func (s *selection) chooseAll() {
	set := make(map[string]bool, len(s.ids))
	for _, id := range s.ids {
		set[id] = true
	}
	s.change.chosen = set
}

// typeAheadReset is how long a pause starts a new type-ahead prefix.
const typeAheadReset = time.Second

// typeAheadKeys are the keys type-ahead reads. Gio delivers other
// characters only as text input to an editor, which a list does not
// request, so type-ahead matches ASCII letters and digits only.
const typeAheadKeys = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// typeAhead moves the selection to the next row whose label starts with
// the letters typed in quick succession (a pause starts over); a repeated
// first letter cycles through the rows starting with it. Matching is
// case-insensitive.
func (s *selection) typeAhead(gtx layout.Context, l *kit.List) {
	if s.keys == nil {
		for _, r := range typeAheadKeys {
			s.keys = append(s.keys, key.Filter{Focus: s.group.FocusHandle(), Name: key.Name(string(r)), Optional: key.ModShift})
		}
	}
	for {
		e, ok := gtx.Event(s.keys...)
		if !ok {
			return
		}
		ke, ok := e.(key.Event)
		if !ok || ke.State != key.Press || !gtx.Enabled() || len(s.labels) == 0 {
			continue
		}
		if gtx.Now.Sub(s.typedAt) > typeAheadReset {
			s.typed = ""
		}
		s.typedAt = gtx.Now
		c := strings.ToLower(string(ke.Name))
		start := s.cursor // a longer prefix may still match the current row
		if s.typed == "" || s.typed == c || strings.Repeat(c, len(s.typed)) == s.typed {
			s.typed = c // first or repeated letter: cycle to the next match
			start = s.cursor + 1
		} else {
			s.typed += c
		}
		for i := range s.labels {
			j := (max(start, 0) + i) % len(s.labels)
			if strings.HasPrefix(strings.ToLower(s.labels[j]), s.typed) {
				s.moveTo(l, j)
				kit.Notify(gtx)
				break
			}
		}
	}
}

// layout lays out w, which draws the rows, as the focus group in a
// KeyContext with the movement actions declared. Call setRows first.
//
// The action handlers run after layout, in Window.End, possibly several in
// one frame: each moves from the cursor the previous one left.
func (s *selection) layout(gtx layout.Context, v selectionView, w layout.Widget) layout.Dimensions {
	s.selected, s.multi, s.chosen = v.selected, v.multi, v.chosen
	s.cursor = slices.Index(s.ids, v.selected)
	for _, id := range s.ids {
		s.group.Join(s.items.Get(id), id == v.selected)
	}
	s.items.Sweep()
	s.group.Active = v.selected
	s.typeAhead(gtx, v.list)
	return kit.KeyContext(gtx, v.context, func(gtx layout.Context) layout.Dimensions {
		kit.OnAction(gtx, kit.ActMoveUp, func() { s.moveTo(v.list, max(s.cursor-1, 0)) })
		kit.OnAction(gtx, kit.ActMoveDown, func() { s.moveTo(v.list, s.cursor+1) })
		kit.OnAction(gtx, kit.ActFirst, func() { s.moveTo(v.list, 0) })
		kit.OnAction(gtx, kit.ActLast, func() { s.moveTo(v.list, len(s.ids)-1) })
		if s.multi {
			kit.OnAction(gtx, kit.ActExtendUp, func() { s.extendTo(v.list, max(s.cursor-1, 0)) })
			kit.OnAction(gtx, kit.ActExtendDown, func() { s.extendTo(v.list, s.cursor+1) })
			kit.OnAction(gtx, kit.ActSelectAll, s.chooseAll)
		}
		if v.actions != nil {
			v.actions(gtx)
		}
		return s.group.Layout(gtx, v.id, w)
	})
}

// isOn reports whether row id draws as selected: chosen with Multiple,
// otherwise the selected row.
func (s *selection) isOn(id string) bool {
	if s.multi {
		return s.chosen[id]
	}
	return id == s.selected
}

// rowSpec describes one selectable row for drawing.
type rowSpec struct {
	id       string // the row's full id
	label    string // spoken name
	role     kit.Role
	selected bool
	padX     unit.Dp // horizontal padding inside the row
}

// row draws one selectable row: accent when selected, Hover on hover,
// ring while focused, the content padded like a control.
func row(gtx layout.Context, th *kit.Theme, c *kit.Clickable, r rowSpec, content layout.Widget) layout.Dimensions {
	return th.Press(gtx, c, r.id, func(gtx layout.Context, st kit.State) layout.Dimensions {
		describe(gtx, r.id, r.role, semantic.Button, r.label)
		semantic.SelectedOp(r.selected).Add(gtx.Ops)
		bg := th.Surface
		if r.selected {
			bg = th.AccentBg
		} else if st.Hovered {
			bg = th.Hover // the row-hover token: differs from Panel, so it shows on cards
		}
		surf := th.Styled(kit.Surface{Bg: bg, Radius: th.Radius})
		if st.Focused {
			surf.Focus = th.Focus
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return surf.Layout(gtx, kit.Pad(th.FieldPad(kit.ControlDefault), th.Space(r.padX), content)) // a field's padding: a row is one control high
	})
}
