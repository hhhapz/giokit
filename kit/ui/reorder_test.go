package ui_test

import (
	"image"
	"testing"

	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// A reorder handle takes part in key dispatch: while it is focused, keys
// it does not claim reach the enclosing KeyContext's actions. It draws a
// focus ring, and a drag moves by whole rows rounded to the nearest.
func TestReorderHandle(t *testing.T) {
	th := kit.Dark()
	var s ui.ReorderListState
	items := []ui.Choice{{ID: "a", Label: "First"}, {ID: "b", Label: "Second"}, {ID: "c", Label: "Third"}}
	confirmed := 0
	h := kittest.NewT(t, 360, 300, func(gtx layout.Context) {
		if from, to, ok := s.Update(gtx); ok {
			ui.Move(items, from, to)
		}
		kit.KeyContext(gtx, "queue", func(gtx layout.Context) layout.Dimensions {
			kit.OnAction(gtx, kit.ActConfirm, func() { confirmed++ })
			return ui.ReorderList{ID: "q", Items: items}.Layout(gtx, th, &s)
		})
	})
	h.Theme = th
	h.ExpectText("Second") // no Item renderer: the row shows its label
	for i := 0; i < 10 && h.FocusedID() != "q/a/drag"; i++ {
		h.Tab(false)
	}
	if h.FocusedID() != "q/a/drag" {
		t.Fatal("reorder handle is not keyboard reachable")
	}
	r := h.ExpectID("q/a/drag")
	if c := h.Pixel(image.Pt(r.Min.X, r.Min.Y+r.Dy()/2)); c != th.Focus {
		t.Errorf("focused handle's edge is %v, want the focus ring %v", c, th.Focus)
	}
	h.Key(key.NameReturn, 0)
	if confirmed != 1 {
		t.Fatalf("Enter on a focused handle ran the enclosing action %d times, want 1", confirmed)
	}
	row := h.Metric.Dp(56)
	h.DragID("q/a/drag", 0, row*6/10) // past half a row: one place down
	if items[1].ID != "a" {
		t.Fatalf("a drag of 0.6 rows: order %v, want a second", items)
	}
	h.DragID("q/a/drag", 0, -row*4/10) // under half a row: stays
	if items[1].ID != "a" {
		t.Fatalf("a drag of 0.4 rows: order %v, want a still second", items)
	}
	h.ExpectClean()
}
