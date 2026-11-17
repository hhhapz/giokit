package ui

import (
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/hhhapz/giokit/kit"
)

// Choice is one stable item of a list: ID never changes, Label is shown
// and spoken.
type Choice struct{ ID, Label string }

// selection is the shared single-select behaviour of lists, trees and
// tables: one Tab stop (a FocusGroup), click to select, Up/Down/Home/End
// move the selection (selection follows focus) and scroll it into view.
// It is controlled: the selected id comes in at layout, and picks wait in
// pending until Update reports them. Multi-select and type-ahead grow
// here, once, for all three.
type selection struct {
	group   kit.FocusGroup
	items   kit.Pool[kit.Clickable]
	ids     []string // last frame's rows
	current string   // last laid-out selection
	pending string   // chosen by a click or key, not yet reported
	has     bool
}

// update reports a row picked since the last call (handle pass).
func (s *selection) update(gtx C) (string, bool) {
	for _, id := range s.ids {
		c := s.items.Get(id)
		s.group.Join(c, id == s.current)
		if c.Clicked(gtx) && id != s.current {
			s.pick(id)
		}
	}
	if !s.has {
		return "", false
	}
	s.has = false
	s.current = s.pending
	return s.pending, true
}

func (s *selection) pick(id string) { s.pending, s.has = id, true }

// layout lays out w as the group, in a KeyContext named kind, with the
// movement actions declared. extra declares more actions (tree expand).
// It returns each row's clickable through click.
func (s *selection) layout(gtx C, kind, id string, l *kit.List, ids []string, selected string, extra func(gtx C, cur int), w kit.W) D {
	s.ids = append(s.ids[:0], ids...)
	s.current = selected
	cur := slices.Index(ids, selected)
	for _, rid := range ids {
		s.group.Join(s.items.Get(rid), rid == selected)
	}
	s.items.Sweep()
	s.group.Active = selected
	sel := func(to int) {
		if len(ids) == 0 {
			return
		}
		to = max(0, min(len(ids)-1, to))
		if ids[to] != selected {
			s.pick(ids[to])
		}
		s.group.Active = ids[to]
		l.ScrollIntoView(to)
	}
	return kit.KeyContext(gtx, kind, func(gtx C) D {
		kit.OnAction(gtx, kit.ActMoveUp, func() { sel(max(cur-1, 0)) })
		kit.OnAction(gtx, kit.ActMoveDown, func() { sel(cur + 1) })
		kit.OnAction(gtx, kit.ActFirst, func() { sel(0) })
		kit.OnAction(gtx, kit.ActLast, func() { sel(len(ids) - 1) })
		if extra != nil {
			extra(gtx, cur)
		}
		return s.group.Layout(gtx, id, w)
	})
}

// row draws one selectable row: accent when selected, panel on hover,
// ring while focused, the content padded like a control.
func row(gtx C, th *kit.Theme, c *kit.Clickable, id, label string, selected bool, padX unit.Dp, content kit.W) D {
	return th.Press(gtx, c, id, func(gtx C, st kit.State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.SelectedOp(selected).Add(gtx.Ops)
		bg := th.Surface
		if selected {
			bg = th.AccentBg
		} else if st.Hovered {
			bg = th.Panel
		}
		surf := th.Styled(kit.Surface{Bg: bg, Radius: th.Radius})
		if st.Focused {
			surf.Focus = th.Focus
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return surf.Layout(gtx, kit.Pad(th.ControlPad(kit.ControlDefault, 7), th.Space(padX), content))
	})
}

// SelectableList is a scrolling list with one selected item. It is one Tab
// stop; Up/Down/Home/End move the selection. Rows are ID/<choice ID>.
//
//	if id, ok := v.list.Update(gtx); ok { v.sel = id }        // handle pass
//	ui.SelectableList{ID: "files", Items: files, Selected: v.sel}.Layout(gtx, th, &v.list)
type SelectableList struct {
	ID       string
	Items    []Choice
	Selected string
}

// SelectableListState is a SelectableList's scroll and selection state.
type SelectableListState struct {
	List kit.List
	sel  selection
}

// Update returns the id of an item selected since the last call.
func (s *SelectableListState) Update(gtx C) (string, bool) { return s.sel.update(gtx) }

func (sl SelectableList) Layout(gtx C, th *kit.Theme, s *SelectableListState) D {
	ids := make([]string, len(sl.Items))
	for i, c := range sl.Items {
		ids[i] = c.ID
	}
	return s.sel.layout(gtx, "list", sl.ID, &s.List, ids, sl.Selected, nil, func(gtx C) D {
		return th.List(gtx, &s.List, sl.ID, len(sl.Items), func(gtx C, i int) D {
			gtx.Constraints.Min.Y = 0
			it := sl.Items[i]
			return row(gtx, th, s.sel.items.Get(it.ID), kit.Part(sl.ID, it.ID), it.Label, it.ID == sl.Selected, kit.Space2p5, func(gtx C) D {
				return th.Text(gtx, it.Label, th.Type.Body, th.Fg, kit.Regular)
			})
		})
	})
}

// TreeNode is a nested item; IDs must be unique and stable across frames.
type TreeNode struct {
	ID, Label string
	Children  []TreeNode
}

type treeRow struct {
	node  TreeNode
	depth int
}

// Tree is a scrolling tree with one selected node. Expansion is the
// tree's own state (TreeState.SetExpanded to preset it). Keyboard
// (WAI-ARIA): Up/Down/Home/End move; Right opens a closed node or moves to
// its first child; Left closes an open node or moves to its parent. Rows
// are ID/<node ID>, toggles ID/<node ID>/toggle.
type Tree struct {
	ID       string
	Roots    []TreeNode
	Selected string
}

// TreeState is a Tree's scroll, expansion and selection state.
type TreeState struct {
	List     kit.List
	expanded map[string]bool
	rows     []treeRow
	sel      selection
	toggles  kit.Pool[kit.Clickable]
}

func (s *TreeState) Expanded(id string) bool { return s.expanded[id] }

func (s *TreeState) SetExpanded(id string, open bool) {
	if s.expanded == nil {
		s.expanded = make(map[string]bool)
	}
	s.expanded[id] = open
}

// Update returns the id of a node selected since the last call, and runs
// the expand toggles.
func (s *TreeState) Update(gtx C) (string, bool) {
	for _, r := range s.rows {
		if len(r.node.Children) == 0 {
			continue
		}
		toggle := s.toggles.Get(r.node.ID)
		s.sel.group.Join(toggle, false) // arrows expand; the toggle is no Tab stop
		if toggle.Clicked(gtx) {
			s.SetExpanded(r.node.ID, !s.Expanded(r.node.ID))
			kit.Notify(gtx)
		}
	}
	return s.sel.update(gtx)
}

func (t Tree) Layout(gtx C, th *kit.Theme, s *TreeState) D {
	s.rows = s.rows[:0]
	var add func([]TreeNode, int)
	add = func(nodes []TreeNode, depth int) {
		for _, n := range nodes {
			s.rows = append(s.rows, treeRow{n, depth})
			if s.Expanded(n.ID) {
				add(n.Children, depth+1)
			}
		}
	}
	add(t.Roots, 0)
	ids := make([]string, len(s.rows))
	for i, r := range s.rows {
		ids[i] = r.node.ID
		if len(r.node.Children) > 0 {
			s.sel.group.Join(s.toggles.Get(r.node.ID), false)
		}
	}
	defer s.toggles.Sweep()
	selectRow := func(i int) {
		if ids[i] != t.Selected {
			s.sel.pick(ids[i])
		}
		s.sel.group.Active = ids[i]
		s.List.ScrollIntoView(i)
	}
	keys := func(gtx C, cur int) {
		kit.OnAction(gtx, kit.ActExpand, func() {
			if cur < 0 || len(s.rows[cur].node.Children) == 0 {
				return
			}
			if n := s.rows[cur].node; !s.Expanded(n.ID) {
				s.SetExpanded(n.ID, true)
			} else {
				selectRow(cur + 1)
			}
		})
		kit.OnAction(gtx, kit.ActCollapse, func() {
			if cur < 0 {
				return
			}
			r := s.rows[cur]
			if len(r.node.Children) > 0 && s.Expanded(r.node.ID) {
				s.SetExpanded(r.node.ID, false)
				return
			}
			for i := cur - 1; i >= 0; i-- {
				if s.rows[i].depth < r.depth {
					selectRow(i)
					return
				}
			}
		})
	}
	return s.sel.layout(gtx, "tree", t.ID, &s.List, ids, t.Selected, keys, func(gtx C) D {
		return th.List(gtx, &s.List, t.ID, len(s.rows), func(gtx C, i int) D { return t.row(gtx, th, s, s.rows[i]) })
	})
}

func (t Tree) row(gtx C, th *kit.Theme, s *TreeState, r treeRow) D {
	return kit.Row(gtx,
		kit.Rigid(kit.Gap(th.Space(kit.Space4*unit.Dp(r.depth)), 0)),
		kit.Rigid(func(gtx C) D {
			if len(r.node.Children) == 0 {
				return kit.Box(gtx, th.IconButtonSize, kit.Surface{}, func(C) D { return D{} })
			}
			icon, label := kit.IconChevronRight, fmt.Sprintf(th.Strings.Expand, r.node.Label)
			if s.Expanded(r.node.ID) {
				icon, label = kit.IconChevronDown, fmt.Sprintf(th.Strings.Collapse, r.node.Label)
			}
			return IconButton{ID: kit.Part(t.ID, r.node.ID, "toggle"), Icon: icon, Label: label}.Layout(gtx, th, s.toggles.Get(r.node.ID))
		}),
		kit.Flexed(1, func(gtx C) D {
			return row(gtx, th, s.sel.items.Get(r.node.ID), kit.Part(t.ID, r.node.ID), r.node.Label, r.node.ID == t.Selected, kit.Space2, func(gtx C) D {
				return th.Text(gtx, r.node.Label, th.Type.Body, th.Fg, kit.Regular)
			})
		}),
	)
}

// DataRow has a stable ID and one text cell per column.
type DataRow struct {
	ID    string
	Cells []string
}

// DataTable is a table with a sticky header that sorts (click a header,
// again to reverse), aligned columns and one selected row; rows virtualize
// within the available height. Headers ID/sort/<column>, rows
// ID/<row ID>.
type DataTable struct {
	ID       string
	Columns  []kit.Column
	Rows     []DataRow
	Selected string
}

// DataTableState is a DataTable's scroll, sort and selection state.
type DataTableState struct {
	List       kit.List
	SortColumn int
	Descending bool
	Sorted     bool // whether a header has been clicked
	headers    kit.Pool[kit.Clickable]
	ncols      int
	sel        selection
}

// Update returns the id of a row selected since the last call, and runs
// the sort headers.
func (s *DataTableState) Update(gtx C) (string, bool) {
	for i := range s.ncols {
		if s.headers.Get(strconv.Itoa(i)).Clicked(gtx) {
			if s.Sorted && s.SortColumn == i {
				s.Descending = !s.Descending
			} else {
				s.SortColumn, s.Descending = i, false
			}
			s.Sorted = true
			kit.Notify(gtx)
		}
	}
	return s.sel.update(gtx)
}

func (dt DataTable) Layout(gtx C, th *kit.Theme, s *DataTableState) D {
	s.ncols = len(dt.Columns)
	table := kit.Table{Cols: dt.Columns}
	rows := dt.sorted(s)
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	rowsID := kit.Part(dt.ID, "rows")
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return table.Cells(gtx, func(gtx C, col int) D {
				label := dt.Columns[col].Title
				if s.Sorted && s.SortColumn == col {
					f := th.Strings.SortedAsc
					if s.Descending {
						f = th.Strings.SortedDesc
					}
					label = fmt.Sprintf(f, label)
				}
				c := strconv.Itoa(col)
				return Button{ID: kit.Part(dt.ID, "sort", c), Text: label, Style: Ghost}.Layout(gtx, th, s.headers.Get(c))
			})
		}),
		kit.Rigid(kit.HLine(th.Divider)),
		kit.Flexed(1, func(gtx C) D {
			return s.sel.layout(gtx, "table", rowsID, &s.List, ids, dt.Selected, nil, func(gtx C) D {
				return th.List(gtx, &s.List, rowsID, len(rows), func(gtx C, i int) D {
					r := rows[i]
					return row(gtx, th, s.sel.items.Get(r.ID), kit.Part(dt.ID, r.ID), strings.Join(r.Cells, ", "), r.ID == dt.Selected, kit.Space2, func(gtx C) D {
						return table.Cells(gtx, func(gtx C, col int) D {
							if col >= len(r.Cells) {
								return D{}
							}
							return th.Text(gtx, r.Cells[col], th.Type.Body, th.Fg, kit.Regular)
						})
					})
				})
			})
		}),
	)
}

// sorted is the rows in the state's sort order (case-insensitive, stable).
func (dt DataTable) sorted(s *DataTableState) []DataRow {
	rows := append([]DataRow(nil), dt.Rows...)
	if !s.Sorted {
		return rows
	}
	cell := func(r DataRow) string {
		if s.SortColumn < len(r.Cells) {
			return r.Cells[s.SortColumn]
		}
		return ""
	}
	slices.SortStableFunc(rows, func(a, b DataRow) int {
		c := strings.Compare(strings.ToLower(cell(a)), strings.ToLower(cell(b)))
		if s.Descending {
			return -c
		}
		return c
	})
	return rows
}

// ReorderList is a scrollable queue whose rows can be reordered: drag a
// row's handle (ID/<item>/drag; Up/Down while it is focused) or use its
// buttons (ID/<item>/up, ID/<item>/down). Rows have a fixed height, so a
// drag maps to an index without measuring offscreen content. Apply the
// moves Update reports to your items (Move does it for a slice).
type ReorderList struct {
	ID    string
	Items []Choice
	Item  func(gtx C, c Choice) D // the row's content: title, artwork, metadata
}

// ReorderListState is a ReorderList's scroll, drag and button state.
type ReorderListState struct {
	List     kit.AnchoredList
	handles  kit.Pool[reorderHandle]
	up, down kit.Pool[kit.Clickable]
	ids      []string // last frame's items
	rowPx    int
	move     [2]int // the first move requested this frame
	moved    bool
}

type reorderHandle struct {
	drag  gesture.Drag
	start float32
}

// reorderRow is a queue row's height: a title over a line of metadata.
const reorderRow = unit.Dp(56)

// Update returns a move (from, to) requested since the last call; apply it
// to your items in the handle pass. One move a frame: any other in the same
// frame was made against the old order and is dropped.
func (s *ReorderListState) Update(gtx C) (from, to int, ok bool) {
	n := len(s.ids)
	s.moved = false
	request := func(from, to int) {
		if !s.moved { // later moves were made against the old order: drop them
			s.move, s.moved = [2]int{from, to}, true
		}
	}
	for i, id := range s.ids {
		if s.up.Get(id).Clicked(gtx) && i > 0 {
			request(i, i-1)
		}
		if s.down.Get(id).Clicked(gtx) && i < n-1 {
			request(i, i+1)
		}
		h := s.handles.Get(id)
		for {
			e, ok := h.drag.Update(gtx.Metric, gtx.Source, gesture.Vertical)
			if !ok {
				break
			}
			switch e.Kind {
			case pointer.Press:
				h.start = e.Position.Y
			case pointer.Release:
				if s.rowPx > 0 {
					shift := int((e.Position.Y - h.start) / float32(s.rowPx))
					if to := max(0, min(n-1, i+shift)); to != i {
						request(i, to)
					}
				}
			}
		}
		for {
			e, ok := gtx.Event(key.FocusFilter{Target: h}, key.Filter{Focus: h, Name: key.NameUpArrow}, key.Filter{Focus: h, Name: key.NameDownArrow})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				to := i - 1
				if ke.Name == key.NameDownArrow {
					to = i + 1
				}
				if to >= 0 && to < n {
					request(i, to)
				}
			}
		}
	}
	if !s.moved {
		return 0, 0, false
	}
	kit.Notify(gtx)
	return s.move[0], s.move[1], true
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

func (rl ReorderList) Layout(gtx C, th *kit.Theme, s *ReorderListState) D {
	rowPx := max(1, gtx.Dp(reorderRow))
	s.rowPx = rowPx
	s.ids = s.ids[:0]
	for _, c := range rl.Items {
		s.ids = append(s.ids, c.ID)
	}
	defer s.up.Sweep()
	defer s.down.Sweep()
	defer s.handles.Sweep()
	return th.AnchoredList(gtx, &s.List, rl.ID, s.ids, func(gtx C, i int) D {
		c := rl.Items[i]
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = rowPx, rowPx
		return kit.Row(gtx,
			kit.Rigid(func(gtx C) D { return rl.handle(gtx, th, s, c, rowPx) }),
			kit.Flexed(1, func(gtx C) D { gtx.Constraints.Min.Y = 0; return rl.Item(gtx, c) }),
			kit.Rigid(func(gtx C) D {
				return th.Disable(gtx, i == 0, func(gtx C) D {
					return IconButton{ID: kit.Part(rl.ID, c.ID, "up"), Icon: kit.IconChevronUp, Label: fmt.Sprintf(th.Strings.MoveUp, c.Label)}.Layout(gtx, th, s.up.Get(c.ID))
				})
			}),
			kit.Rigid(func(gtx C) D {
				return th.Disable(gtx, i == len(rl.Items)-1, func(gtx C) D {
					return IconButton{ID: kit.Part(rl.ID, c.ID, "down"), Icon: kit.IconChevronDown, Label: fmt.Sprintf(th.Strings.MoveDown, c.Label)}.Layout(gtx, th, s.down.Get(c.ID))
				})
			}),
			kit.Rigid(kit.Gap(th.Space(kit.Space1p5), 0)),
		)
	})
}

// handle is a row's drag handle: a focusable button that drags vertically.
func (rl ReorderList) handle(gtx C, th *kit.Theme, s *ReorderListState, c Choice, rowPx int) D {
	gtx.Constraints.Min = image.Point{}
	h := s.handles.Get(c.ID)
	sz := image.Pt(gtx.Dp(34), rowPx)
	defer clip.Rect{Max: sz}.Push(gtx.Ops).Pop()
	kit.TagLabel(gtx, kit.Part(rl.ID, c.ID, "drag"), fmt.Sprintf(th.Strings.Reorder, c.Label))
	semantic.Button.Add(gtx.Ops)
	if gtx.Enabled() {
		h.drag.Add(gtx.Ops)
		event.Op(gtx.Ops, h)
		pointer.CursorGrab.Add(gtx.Ops)
	}
	gtx.Constraints.Min, gtx.Constraints.Max = sz, sz
	return layout.Center.Layout(gtx, func(gtx C) D { return kit.IconMenu(gtx, th.IconSizeLg, th.Muted) })
}
