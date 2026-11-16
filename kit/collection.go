package kit

import (
	"gioui.org/io/semantic"
	"gioui.org/unit"
)

// Choice is one stable item in a SelectableList.
type Choice struct{ ID, Label string }

// SelectableList owns scrolling, focus, and one selected ID. It is one Tab
// stop; Up/Down/Home/End move the selection (selection follows focus).
type SelectableList struct {
	List     List
	Selected string
	sel      selection
	changed  bool
}

func (s *SelectableList) Changed() (string, bool) {
	id, ok := s.Selected, s.changed
	s.changed = false
	return id, ok
}

func (t *Theme) SelectableList(gtx C, s *SelectableList, id string, choices []Choice) D {
	ids := make([]string, len(choices))
	for i, c := range choices {
		ids[i] = c.ID
	}
	clicks := s.sel.handle(gtx, ids, &s.Selected, &s.changed)
	return s.sel.layout(gtx, "list", id, &s.List, ids, &s.Selected, &s.changed, nil, func(gtx C) D {
		return t.selectableRows(gtx, s, id, choices, clicks)
	})
}

func (t *Theme) selectableRows(gtx C, s *SelectableList, id string, choices []Choice, clicks []*Clickable) D {
	return t.List(gtx, &s.List, id, len(choices), func(gtx C, i int) D {
		gtx.Constraints.Min.Y = 0
		item := choices[i]
		return t.press(gtx, clicks[i], Part(id, item.ID), func(gtx C, st State) D {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(item.Label).Add(gtx.Ops)
			semantic.SelectedOp(s.Selected == item.ID).Add(gtx.Ops)
			bg := t.Surface
			if s.Selected == item.ID {
				bg = t.AccentBg
			} else if st.Hovered {
				bg = t.Panel
			}
			surf := t.Styled(Surface{Bg: bg, Radius: t.Radius})
			if st.Focused {
				surf.Focus = t.Focus
			}
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return surf.Layout(gtx, Pad(t.control(7), t.Space(10), func(gtx C) D {
				return t.Text(gtx, item.Label, t.Type.Body, t.Fg, Regular)
			}))
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

// Tree owns expanded state, selection, and a scrolling list.
type Tree struct {
	List     List
	Selected string
	expanded map[string]bool
	rows     []treeRow
	sel      selection
	toggles  Pool[Clickable]
	changed  bool
}

func (tr *Tree) Expanded(id string) bool { return tr.expanded[id] }
func (tr *Tree) SetExpanded(id string, open bool) {
	if tr.expanded == nil {
		tr.expanded = make(map[string]bool)
	}
	tr.expanded[id] = open
}
func (tr *Tree) Changed() (string, bool) {
	id, ok := tr.Selected, tr.changed
	tr.changed = false
	return id, ok
}

func (t *Theme) Tree(gtx C, tr *Tree, id string, roots []TreeNode) D {
	tr.rows = tr.rows[:0]
	var appendNodes func([]TreeNode, int)
	appendNodes = func(nodes []TreeNode, depth int) {
		for _, node := range nodes {
			tr.rows = append(tr.rows, treeRow{node, depth})
			if tr.Expanded(node.ID) {
				appendNodes(node.Children, depth+1)
			}
		}
	}
	appendNodes(roots, 0)
	ids := make([]string, len(tr.rows))
	for i, row := range tr.rows {
		ids[i] = row.node.ID
	}
	clicks := tr.sel.handle(gtx, ids, &tr.Selected, &tr.changed)
	defer tr.toggles.Sweep()
	for _, row := range tr.rows {
		if len(row.node.Children) == 0 {
			continue
		}
		toggle := tr.toggles.Get(row.node.ID)
		tr.sel.group.Join(toggle, false) // arrows expand; the toggle is no Tab stop
		if toggle.Clicked(gtx) {
			tr.SetExpanded(row.node.ID, !tr.Expanded(row.node.ID))
			Notify(gtx)
		}
	}
	cur := tr.sel.cur
	selectRow := func(i int) {
		if tr.Selected != ids[i] {
			tr.Selected, tr.changed = ids[i], true
		}
		tr.sel.group.Active = tr.Selected
		tr.List.ScrollIntoView(i)
	}
	// WAI-ARIA tree: Right opens a closed node or moves to its first child;
	// Left closes an open node or moves to its parent.
	keys := func(gtx C) {
		OnAction(gtx, ActExpand, func() {
			if cur < 0 || len(tr.rows[cur].node.Children) == 0 {
				return
			}
			if n := tr.rows[cur].node; !tr.Expanded(n.ID) {
				tr.SetExpanded(n.ID, true)
			} else {
				selectRow(cur + 1)
			}
		})
		OnAction(gtx, ActCollapse, func() {
			if cur < 0 {
				return
			}
			row := tr.rows[cur]
			if len(row.node.Children) > 0 && tr.Expanded(row.node.ID) {
				tr.SetExpanded(row.node.ID, false)
				return
			}
			for i := cur - 1; i >= 0; i-- {
				if tr.rows[i].depth < row.depth {
					selectRow(i)
					return
				}
			}
		})
	}
	return tr.sel.layout(gtx, "tree", id, &tr.List, ids, &tr.Selected, &tr.changed, keys, func(gtx C) D {
		return t.treeRows(gtx, tr, id, clicks)
	})
}

func (t *Theme) treeRows(gtx C, tr *Tree, id string, clicks []*Clickable) D {
	return t.List(gtx, &tr.List, id, len(tr.rows), func(gtx C, i int) D {
		row := tr.rows[i]
		return Row(gtx,
			Rigid(Gap(t.Space(unit.Dp(row.depth*16)), 0)),
			Rigid(func(gtx C) D {
				if len(row.node.Children) == 0 {
					return Box(gtx, 22, Surface{}, func(C) D { return D{} })
				}
				icon := IconChevronRight
				if tr.Expanded(row.node.ID) {
					icon = IconChevronDown
				}
				label := "Expand " + row.node.Label
				if tr.Expanded(row.node.ID) {
					label = "Collapse " + row.node.Label
				}
				return t.IconButtonLabel(gtx, tr.toggles.Get(row.node.ID), Part(id, row.node.ID, "toggle"), label, icon, false)
			}),
			Flexed(1, func(gtx C) D {
				return t.press(gtx, clicks[i], Part(id, row.node.ID), func(gtx C, st State) D {
					semantic.Button.Add(gtx.Ops)
					semantic.LabelOp(row.node.Label).Add(gtx.Ops)
					semantic.SelectedOp(tr.Selected == row.node.ID).Add(gtx.Ops)
					bg := t.Surface
					if tr.Selected == row.node.ID {
						bg = t.AccentBg
					} else if st.Hovered {
						bg = t.Panel
					}
					surf := t.Styled(Surface{Bg: bg, Radius: t.Radius})
					if st.Focused {
						surf.Focus = t.Focus
					}
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return surf.Layout(gtx, Pad(t.control(7), t.Space(8), func(gtx C) D { return t.Text(gtx, row.node.Label, t.Type.Body, t.Fg, Regular) }))
				})
			}),
		)
	})
}
