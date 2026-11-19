package ui_test

import (
	"testing"

	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// Right twice in one frame opens a closed node, then moves to its first
// child: the second key sees the rows the first one revealed.
func TestTreeKeyBatch(t *testing.T) {
	th := kit.Dark()
	var s ui.TreeState
	sel := "p"
	roots := []ui.TreeNode{
		{ID: "p", Label: "Project", Children: []ui.TreeNode{{ID: "x", Label: "X"}}},
		{ID: "q", Label: "Other"},
	}
	h := kittest.NewT(t, 300, 300, func(gtx layout.Context) {
		if e, ok := s.Update(gtx); ok {
			sel = e.Selected
		}
		ui.Tree{ID: "tree", Roots: roots, Selected: sel}.Layout(gtx, th, &s)
	})
	h.ClickID("tree/p")
	h.Keys(key.NameRightArrow, key.NameRightArrow)
	if !s.Expanded("p") || sel != "x" {
		t.Fatalf("after Right, Right in one frame: expanded %v, selected %q; want open, x", s.Expanded("p"), sel)
	}
	h.Keys(key.NameLeftArrow, key.NameLeftArrow)
	if s.Expanded("p") || sel != "p" {
		t.Fatalf("after Left, Left in one frame: expanded %v, selected %q; want closed, p", s.Expanded("p"), sel)
	}
	h.ExpectClean()
}
