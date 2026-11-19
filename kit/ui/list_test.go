package ui_test

import (
	"testing"

	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// Several movement keys delivered to one frame each move from where the
// previous one left the selection, not from the frame's starting row.
func TestSelectionKeyBatch(t *testing.T) {
	th := kit.Dark()
	var s ui.SelectableListState
	sel := "a"
	items := []ui.Choice{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}, {ID: "c", Label: "Gamma"}, {ID: "d", Label: "Delta"}}
	h := kittest.NewT(t, 300, 300, func(gtx layout.Context) {
		if e, ok := s.Update(gtx); ok {
			sel = e.Selected
		}
		ui.SelectableList{ID: "l", Items: items, Selected: sel}.Layout(gtx, th, &s)
	})
	h.ClickID("l/a")
	steps := []struct {
		keys []key.Name
		want string
	}{
		{[]key.Name{key.NameDownArrow, key.NameDownArrow}, "c"},
		{[]key.Name{key.NameUpArrow, key.NameDownArrow}, "c"},
		{[]key.Name{key.NameDownArrow, key.NameUpArrow, key.NameUpArrow}, "b"},
		{[]key.Name{key.NameEnd, key.NameUpArrow}, "c"},
	}
	for _, st := range steps {
		h.Keys(st.keys...)
		if sel != st.want {
			t.Fatalf("after %v in one frame: selected %q, want %q", st.keys, sel, st.want)
		}
	}
	h.ExpectClean()
}
