package ui

import (
	"github.com/hhhapz/giokit/gio/layout"

	"github.com/hhhapz/giokit/kit"
)

// SelectableList is a scrolling list with one selected item, or several
// with Multiple. It is one Tab stop; Up/Down/Home/End move the selection
// and typing a label's first letters jumps to it. Rows are ID/<choice ID>,
// with role option in a listbox.
//
//	if e, ok := v.list.Update(gtx); ok { v.sel = e.Selected }        // handle pass
//	ui.SelectableList{ID: "files", Items: files, Selected: v.sel}.Layout(gtx, th, &v.list)
type SelectableList struct {
	ID string // the listbox's id; rows are ID/<choice ID>
	// Items are the rows, in order. Their IDs must be unique and non-empty.
	Items []Choice
	// Selected is the selected item's ID (controlled); "" selects nothing.
	Selected string
	// Item, if set, draws item i's content inside the row (its label is
	// still spoken).
	Item func(gtx layout.Context, i int) layout.Dimensions
	// Multiple allows choosing several items: Ctrl/Cmd-click toggles,
	// Shift-click and Shift+Up/Down select a range, Ctrl/Cmd+A all. Chosen is
	// the set (yours, controlled); Selected stays the focused item. The
	// control reads Chosen during layout and never modifies it.
	Multiple bool
	Chosen   map[string]bool // the chosen IDs while Multiple; nil is none
}

// SelectableListState is a SelectableList's scroll and selection state.
// The zero value is ready; it must not be copied after first use.
type SelectableListState struct {
	// List is the scroll position. Read or set it between frames
	// (List.ScrollIntoView, the embedded Position) to save or restore it.
	List kit.List
	sel  selection
}

// Update reports a selection change made by clicks and keys since the last
// call: call it in the handle pass, before Layout. Several moves in one
// frame are reported together as one event.
func (s *SelectableListState) Update(gtx layout.Context) (SelectionEvent, bool) {
	return s.sel.update(gtx)
}

// Layout draws the visible rows, filling the available size, with
// Selected (and Chosen) highlighted, and makes them one focus group. It
// handles the arrow keys and type-ahead while laying out and queues the
// change for s.Update, which also reports clicks.
func (sl SelectableList) Layout(gtx layout.Context, th *kit.Theme, s *SelectableListState) layout.Dimensions {
	s.sel.setRows(len(sl.Items), func(i int) (string, string) { return sl.Items[i].ID, sl.Items[i].Label })
	s.List.Role = kit.RoleListbox
	view := selectionView{context: "list", id: sl.ID, list: &s.List, selected: sl.Selected, multi: sl.Multiple, chosen: sl.Chosen}
	return s.sel.layout(gtx, view, func(gtx layout.Context) layout.Dimensions {
		return th.List(gtx, &s.List, sl.ID, len(sl.Items), func(gtx layout.Context, i int) layout.Dimensions {
			gtx.Constraints.Min.Y = 0
			it := sl.Items[i]
			spec := rowSpec{id: kit.Part(sl.ID, it.ID), label: it.Label, role: kit.RoleOption, selected: s.sel.isOn(it.ID), padX: kit.Space2p5}
			return row(gtx, th, s.sel.items.Get(it.ID), spec, func(gtx layout.Context) layout.Dimensions {
				if sl.Item != nil {
					return sl.Item(gtx, i)
				}
				return th.Text(gtx, it.Label, th.Type.Body, th.Fg, kit.Regular)
			})
		})
	})
}
