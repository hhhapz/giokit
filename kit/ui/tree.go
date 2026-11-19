package ui

import (
	"fmt"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/unit"

	"github.com/hhhapz/giokit/kit"
)

// TreeNode is a nested item; IDs must be unique, non-empty and stable
// across frames.
type TreeNode struct {
	ID, Label string     // the node's stable id and visible, spoken text
	Children  []TreeNode // nil or empty for a leaf
}

// treeRow is one visible row of a tree: a node under expanded ancestors.
type treeRow struct {
	node  TreeNode
	depth int
}

// Tree is a scrolling tree with one selected node, or several with
// Multiple. Keyboard (WAI-ARIA): Up/Down/Home/End move; Right opens a
// closed node or moves to its first child; Left closes an open node or
// moves to its parent; typing a label's first letters jumps to it. Rows
// are ID/<node ID> (role treeitem), expand toggles ID/<node ID>/toggle.
//
// Which nodes are expanded is retained interaction state, owned by
// TreeState rather than passed in props: the user opens and closes nodes,
// and the app may preset or read them with SetExpanded and Expanded.
type Tree struct {
	ID string // the tree's id; see the type doc for the row parts
	// Roots are the top-level nodes, in order.
	Roots []TreeNode
	// Selected is the selected node's ID (controlled); "" selects nothing.
	Selected string
	// Node, if set, draws a node's content inside its row (an icon beside
	// the label); the label is still spoken.
	Node func(gtx layout.Context, n TreeNode) layout.Dimensions
	// Multiple and Chosen: as for SelectableList (ranges follow the visible rows).
	Multiple bool
	Chosen   map[string]bool // the chosen IDs while Multiple; nil is none
}

// TreeState is a Tree's scroll, expansion and selection state. The zero
// value is ready, with every node closed; it must not be copied after
// first use.
type TreeState struct {
	List     kit.List // the scroll position (see SelectableListState.List)
	expanded map[string]bool
	rows     []treeRow // the visible rows as last laid out
	sel      selection
	toggles  kit.Pool[kit.Clickable]
}

// Expanded reports whether node id is open.
func (s *TreeState) Expanded(id string) bool { return s.expanded[id] }

// SetExpanded opens or closes node id. It takes effect at the next layout;
// expansion is kept for ids that are not (or no longer) in the tree.
func (s *TreeState) SetExpanded(id string, open bool) {
	if s.expanded == nil {
		s.expanded = make(map[string]bool)
	}
	s.expanded[id] = open
}

// Update runs the expand toggles and reports a selection change made since
// the last call (see SelectableListState.Update).
func (s *TreeState) Update(gtx layout.Context) (SelectionEvent, bool) {
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

// appendVisible appends nodes and, under each expanded node, its
// descendants, depth first.
func (s *TreeState) appendVisible(rows []treeRow, nodes []TreeNode, depth int) []treeRow {
	for _, n := range nodes {
		rows = append(rows, treeRow{node: n, depth: depth})
		if s.Expanded(n.ID) {
			rows = s.appendVisible(rows, n.Children, depth+1)
		}
	}
	return rows
}

// setRows rebuilds the visible rows from roots and hands them to the
// selection. A key handler calls it after expanding or collapsing, so the
// next key of the same frame sees the new rows.
func (s *TreeState) setRows(roots []TreeNode) {
	s.rows = s.appendVisible(s.rows[:0], roots, 0)
	s.sel.setRows(len(s.rows), func(i int) (string, string) { return s.rows[i].node.ID, s.rows[i].node.Label })
}

// expand handles Right: it opens the focused node, or moves into an open
// one's first child.
func (s *TreeState) expand(roots []TreeNode) {
	cur := s.sel.cursor
	if cur < 0 || cur >= len(s.rows) || len(s.rows[cur].node.Children) == 0 {
		return
	}
	if n := s.rows[cur].node; !s.Expanded(n.ID) {
		s.SetExpanded(n.ID, true)
		s.setRows(roots)
		return
	}
	s.sel.focusRow(&s.List, cur+1)
}

// collapse handles Left: it closes the focused node if open, otherwise
// moves to its parent.
func (s *TreeState) collapse(roots []TreeNode) {
	cur := s.sel.cursor
	if cur < 0 || cur >= len(s.rows) {
		return
	}
	r := s.rows[cur]
	if len(r.node.Children) > 0 && s.Expanded(r.node.ID) {
		s.SetExpanded(r.node.ID, false)
		s.setRows(roots)
		return
	}
	for i := cur - 1; i >= 0; i-- {
		if s.rows[i].depth < r.depth {
			s.sel.focusRow(&s.List, i)
			return
		}
	}
}

// Layout draws the visible rows (nodes under expanded ancestors), filling
// the available size, indented by depth, with Selected highlighted, and
// makes them one focus group. It handles the arrow keys (Right and Left
// expand and collapse in s) and type-ahead while laying out; s.Update
// reports the selection change and runs the expand toggles.
func (t Tree) Layout(gtx layout.Context, th *kit.Theme, s *TreeState) layout.Dimensions {
	s.setRows(t.Roots)
	for _, r := range s.rows {
		if len(r.node.Children) > 0 {
			s.sel.group.Join(s.toggles.Get(r.node.ID), false)
		}
	}
	defer s.toggles.Sweep()
	kit.SetRole(gtx, t.ID, kit.RoleTree)
	view := selectionView{context: "tree", id: t.ID, list: &s.List, selected: t.Selected, multi: t.Multiple, chosen: t.Chosen,
		actions: func(gtx layout.Context) {
			kit.OnAction(gtx, kit.ActExpand, func() { s.expand(t.Roots) })
			kit.OnAction(gtx, kit.ActCollapse, func() { s.collapse(t.Roots) })
		}}
	return s.sel.layout(gtx, view, func(gtx layout.Context) layout.Dimensions {
		return th.List(gtx, &s.List, t.ID, len(s.rows), func(gtx layout.Context, i int) layout.Dimensions { return t.row(gtx, th, s, s.rows[i]) })
	})
}

// row draws one visible row: indent, expand toggle (or a spacer of the same
// size for a leaf), then the selectable node.
func (t Tree) row(gtx layout.Context, th *kit.Theme, s *TreeState, r treeRow) layout.Dimensions {
	return kit.Row(gtx,
		kit.Rigid(kit.Gap(th.Space(kit.Space4*unit.Dp(r.depth)), 0)),
		kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(r.node.Children) == 0 {
				return kit.Box(gtx, th.IconButtonSize, kit.Surface{}, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
			}
			icon, label := kit.IconChevronRight, fmt.Sprintf(th.Strings.Expand, r.node.Label)
			if s.Expanded(r.node.ID) {
				icon, label = kit.IconChevronDown, fmt.Sprintf(th.Strings.Collapse, r.node.Label)
			}
			return IconButton{ID: kit.Part(t.ID, r.node.ID, "toggle"), Icon: icon, Label: label}.Layout(gtx, th, s.toggles.Get(r.node.ID))
		}),
		kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			spec := rowSpec{id: kit.Part(t.ID, r.node.ID), label: r.node.Label, role: kit.RoleTreeItem, selected: s.sel.isOn(r.node.ID), padX: kit.Space2}
			return row(gtx, th, s.sel.items.Get(r.node.ID), spec, func(gtx layout.Context) layout.Dimensions {
				if t.Node != nil {
					return t.Node(gtx, r.node)
				}
				return th.Text(gtx, r.node.Label, th.Type.Body, th.Fg, kit.Regular)
			})
		}),
	)
}
