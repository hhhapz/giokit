package ui_test

import (
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// Custom renderers draw the content; the plain strings still name each
// item for screen readers and tests.
func TestRenderers(t *testing.T) {
	th := kit.Dark()
	var sel ui.SelectState
	var list ui.SelectableListState
	var table ui.DataTableState
	opts := []ui.Choice{{ID: "small", Label: "Small"}, {ID: "large", Label: "Large"}}
	h := kittest.NewT(t, 640, 420, func(gtx layout.Context) {
		sel.Update(gtx)
		list.Update(gtx)
		table.Update(gtx)
		kit.Flex{Gap: kit.Space4}.Layout(gtx, th,
			kit.Rigid(kit.Fixed(160, func(gtx layout.Context) layout.Dimensions {
				return ui.Select{ID: "size", Options: opts, Selected: "small", Option: func(gtx layout.Context, c ui.Choice) layout.Dimensions {
					return th.Text(gtx, c.Label+" (custom)", th.Type.Body, th.Fg, kit.Regular)
				}}.Layout(gtx, th, &sel)
			})),
			kit.Rigid(kit.Fixed(160, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.Y = gtx.Dp(120)
				return ui.SelectableList{ID: "files", Items: []ui.Choice{{ID: "a", Label: "Alpha"}}, Item: func(gtx layout.Context, i int) layout.Dimensions {
					return th.Text(gtx, "Alpha (custom)", th.Type.Body, th.Fg, kit.Regular)
				}}.Layout(gtx, th, &list)
			})),
			kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return ui.DataTable{ID: "t", Columns: []ui.DataColumn{{ID: "name", Title: "Name"}, {ID: "state", Title: "State"}},
					Rows: []ui.DataRow{{ID: "r", Cells: []string{"Build", "ok"}}},
					Cell: func(gtx layout.Context, r ui.DataRow, col int) layout.Dimensions {
						if col == 1 {
							return ui.Badge{Status: kit.StatusGood, Text: "passing"}.Layout(gtx, th)
						}
						return th.Text(gtx, r.Cells[col], th.Type.Body, th.Fg, kit.Regular)
					}}.Layout(gtx, th, &table)
			}),
		)
	})
	h.ExpectText("Alpha (custom)")
	h.ExpectText("passing")
	h.ClickID("size/field")
	h.ExpectText("Large (custom)")
	for id, want := range map[string]string{"size/large": "Large", "files/a": "Alpha", "t/r": "Build, ok"} {
		if got := label(h, id); got != want {
			t.Errorf("%s spoken as %q, want %q", id, got, want)
		}
	}
	h.ExpectClean()
}

func label(h *kittest.Harness, id string) string {
	var find func([]kittest.A11yNode) string
	find = func(ns []kittest.A11yNode) string {
		for _, n := range ns {
			if n.ID == id {
				return n.Label
			}
			if l := find(n.Children); l != "" {
				return l
			}
		}
		return ""
	}
	return find(h.A11yTree())
}

// Typing in a focused list jumps to the item starting with the letters;
// a repeated letter cycles, a pause starts a new prefix.
func TestTypeAhead(t *testing.T) {
	th := kit.Dark()
	var s ui.SelectableListState
	sel := "a"
	items := []ui.Choice{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}, {ID: "br", Label: "Bravo"}, {ID: "c", Label: "Charlie"}}
	h := kittest.NewT(t, 300, 300, func(gtx layout.Context) {
		if e, ok := s.Update(gtx); ok {
			sel = e.Selected
		}
		ui.SelectableList{ID: "l", Items: items, Selected: sel}.Layout(gtx, th, &s)
	})
	h.Tab(false)
	steps := []struct {
		keys string
		wait bool
		want string
	}{
		{"b", false, "b"},  // first match
		{"b", false, "br"}, // repeated letter cycles
		{"c", true, "c"},   // after a pause, a new prefix
		{"br", true, "br"}, // a longer prefix narrows
	}
	for _, st := range steps {
		if st.wait {
			h.Wait(2 * time.Second)
		}
		for _, r := range st.keys {
			h.Key(key.Name(strings.ToUpper(string(r))), 0)
		}
		if sel != st.want {
			t.Fatalf("after typing %q: selected %q, want %q", st.keys, sel, st.want)
		}
	}
}

// Multiple selection: Ctrl-click toggles, Shift-click and Shift+arrows
// select ranges from the anchor, a plain move selects one, Ctrl+A all.
func TestMultiSelect(t *testing.T) {
	th := kit.Dark()
	var s ui.SelectableListState
	focus, chosen := "", map[string]bool{}
	var items []ui.Choice
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		items = append(items, ui.Choice{ID: id, Label: strings.ToUpper(id)})
	}
	h := kittest.NewT(t, 300, 400, func(gtx layout.Context) {
		if e, ok := s.Update(gtx); ok {
			focus, chosen = e.Selected, e.Chosen
		}
		ui.SelectableList{ID: "l", Items: items, Selected: focus, Multiple: true, Chosen: chosen}.Layout(gtx, th, &s)
	})
	set := func() string {
		var out string
		for _, it := range items {
			if chosen[it.ID] {
				out += it.ID
			}
		}
		return out
	}
	steps := []struct {
		name string
		do   func()
		want string
	}{
		{"click a", func() { h.ClickID("l/a") }, "a"},
		{"ctrl-click c", func() { h.ClickIDMods("l/c", key.ModShortcut) }, "ac"},
		{"shift-click e", func() { h.ClickIDMods("l/e", key.ModShift) }, "cde"},
		{"ctrl-click d", func() { h.ClickIDMods("l/d", key.ModShortcut) }, "ce"},
		{"up", func() { h.Key(key.NameUpArrow, 0) }, "c"},
		{"shift+up", func() { h.Key(key.NameUpArrow, key.ModShift) }, "bc"},
		{"ctrl+a", func() { h.Key("A", key.ModShortcut) }, "abcde"},
	}
	for _, st := range steps {
		st.do()
		if got := set(); got != st.want {
			t.Fatalf("%s: chosen %q, want %q (focus %q)", st.name, got, st.want, focus)
		}
	}
	h.ExpectClean()
}

// Trees and tables share the multiple selection: a Shift range follows the
// visible order (a tree's expanded rows, a table's sort).
func TestMultiSelectTreeTable(t *testing.T) {
	th := kit.Dark()
	var ts ui.TreeState
	var ds ui.DataTableState
	tf, tc := "", map[string]bool{}
	df, dc := "", map[string]bool{}
	roots := []ui.TreeNode{{ID: "p", Label: "Project", Children: []ui.TreeNode{{ID: "x", Label: "X"}, {ID: "y", Label: "Y"}}}}
	rows := []ui.DataRow{{ID: "3", Cells: []string{"Charlie"}}, {ID: "1", Cells: []string{"Alpha"}}, {ID: "2", Cells: []string{"Bravo"}}}
	ts.SetExpanded("p", true)
	h := kittest.NewT(t, 640, 360, func(gtx layout.Context) {
		if e, ok := ts.Update(gtx); ok {
			tf, tc = e.Selected, e.Chosen
		}
		if e, ok := ds.Update(gtx); ok {
			df, dc = e.Selected, e.Chosen
		}
		kit.Flex{Gap: kit.Space4}.Layout(gtx, th,
			kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return ui.Tree{ID: "tree", Roots: roots, Selected: tf, Multiple: true, Chosen: tc}.Layout(gtx, th, &ts)
			}),
			kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return ui.DataTable{ID: "t", Columns: []ui.DataColumn{{ID: "name", Title: "Name"}}, Rows: rows, Selected: df, Multiple: true, Chosen: dc}.Layout(gtx, th, &ds)
			}),
		)
	})
	h.ClickID("tree/p")
	h.ClickIDMods("tree/y", key.ModShift)
	if len(tc) != 3 || !tc["x"] {
		t.Fatalf("tree range: %v", tc)
	}
	h.ClickID("t/sort/name") // sort by name: Alpha, Bravo, Charlie
	h.ClickID("t/1")
	h.ClickIDMods("t/2", key.ModShift)
	if len(dc) != 2 || !dc["1"] || !dc["2"] || dc["3"] {
		t.Fatalf("table range in sort order: %v", dc)
	}
}

// Row hover uses the Hover token, as table rows do: it differs from a card's
// Panel background (lists used Panel, so a hovered row merged into the card).
func TestRowHoverVisibleOnCard(t *testing.T) {
	th := kit.Dark()
	var s ui.SelectableListState
	h := kittest.NewT(t, 300, 200, func(gtx layout.Context) {
		s.Update(gtx)
		th.Card(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.SelectableList{ID: "l", Items: []ui.Choice{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}}}.Layout(gtx, th, &s)
		})
	})
	h.Theme = th
	h.Hover("l/b")
	h.ExpectBg("l/b", th.Hover)
}

// Components carry WAI-ARIA roles beyond Gio's five classes.
func TestRoles(t *testing.T) {
	th := kit.Dark()
	var tabs ui.TabsState
	var list ui.SelectableListState
	var tree ui.TreeState
	var table ui.DataTableState
	var sel ui.SelectState
	var nav kit.Clickable
	h := kittest.NewT(t, 900, 400, func(gtx layout.Context) {
		sel.Update(gtx)
		kit.Flex{Gap: kit.Space2}.Layout(gtx, th,
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Tabs{ID: "tabs", Items: []ui.Choice{{ID: "one", Label: "One"}, {ID: "two", Label: "Two"}}}.Layout(gtx, th, &tabs)
			}),
			kit.Rigid(kit.Fixed(120, func(gtx layout.Context) layout.Dimensions {
				return ui.SelectableList{ID: "list", Items: []ui.Choice{{ID: "a", Label: "A"}}}.Layout(gtx, th, &list)
			})),
			kit.Rigid(kit.Fixed(120, func(gtx layout.Context) layout.Dimensions {
				return ui.Tree{ID: "tree", Roots: []ui.TreeNode{{ID: "n", Label: "N"}}}.Layout(gtx, th, &tree)
			})),
			kit.Rigid(kit.Fixed(160, func(gtx layout.Context) layout.Dimensions {
				return ui.DataTable{ID: "t", Columns: []ui.DataColumn{{ID: "c", Title: "C"}}, Rows: []ui.DataRow{{ID: "r", Cells: []string{"R"}}}}.Layout(gtx, th, &table)
			})),
			kit.Rigid(kit.Fixed(140, func(gtx layout.Context) layout.Dimensions {
				return ui.Select{ID: "s", Options: []ui.Choice{{ID: "x", Label: "X"}, {ID: "y", Label: "Y"}}}.Layout(gtx, th, &sel)
			})),
			kit.Rigid(kit.Fixed(120, func(gtx layout.Context) layout.Dimensions {
				return ui.NavItem{ID: "nav", Label: "Home"}.Layout(gtx, th, &nav)
			})),
		)
	})
	h.ClickID("s/field")
	for id, want := range map[string]kit.Role{
		"tabs": kit.RoleTabList, "tabs/one": kit.RoleTab,
		"list": kit.RoleListbox, "list/a": kit.RoleOption,
		"tree": kit.RoleTree, "tree/n": kit.RoleTreeItem,
		"t/rows": kit.RoleGrid, "t/r": kit.RoleRow,
		"s/field": kit.RoleCombobox, "s": kit.RoleListbox, "s/y": kit.RoleOption,
		"nav": kit.RoleLink,
	} {
		if got := h.Role(id); got != want {
			t.Errorf("%s: role %q, want %q", id, got, want)
		}
	}
	if id := h.ByRole(kit.RoleTab, "Two"); id != "tabs/two" {
		t.Errorf("ByRole(tab, Two) = %q", id)
	}
}
