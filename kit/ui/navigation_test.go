package ui_test

import (
	"testing"

	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// Tabs are addressed by item ID, not label: two tabs with the same label
// are separate tabs (they shared one clickable and one id), and a
// translated label keeps the id.
func TestTabsDuplicateLabels(t *testing.T) {
	th := kit.Dark()
	var s ui.TabsState
	sel := "a"
	items := []ui.Choice{{ID: "a", Label: "Notes"}, {ID: "b", Label: "Notes"}}
	h := kittest.NewT(t, 400, 100, func(gtx layout.Context) {
		if id, ok := s.Update(gtx); ok {
			sel = id
		}
		ui.Tabs{ID: "tabs", Items: items, Selected: sel}.Layout(gtx, th, &s)
	})
	a, b := h.ExpectID("tabs/a"), h.ExpectID("tabs/b")
	if a.Overlaps(b) {
		t.Fatalf("tabs overlap: %v %v", a, b)
	}
	h.ClickID("tabs/b")
	if sel != "b" {
		t.Fatalf("clicked the second tab, selected %q", sel)
	}
	h.ClickID("tabs/a")
	if sel != "a" {
		t.Fatalf("clicked the first tab, selected %q", sel)
	}
	h.ExpectClean()
}

// Two arrow keys in one frame move two tabs: each key moves on from the
// tab the previous one reached, not from the frame's Selected.
func TestTabsKeyBatch(t *testing.T) {
	th := kit.Dark()
	var s ui.TabsState
	sel := "a"
	items := []ui.Choice{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}, {ID: "c", Label: "Gamma"}, {ID: "d", Label: "Delta"}}
	h := kittest.NewT(t, 400, 100, func(gtx layout.Context) {
		if id, ok := s.Update(gtx); ok {
			sel = id
		}
		ui.Tabs{ID: "tabs", Items: items, Selected: sel}.Layout(gtx, th, &s)
	})
	h.ClickID("tabs/a")
	h.Keys(key.NameRightArrow, key.NameRightArrow)
	if sel != "c" {
		t.Fatalf("two Rights in one frame: selected %q, want c", sel)
	}
	h.Keys(key.NameLeftArrow, key.NameLeftArrow)
	if sel != "a" {
		t.Fatalf("two Lefts in one frame: selected %q, want a", sel)
	}
	h.ExpectClean()
}
