package ui_test

import (
	"testing"
	"time"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// The open calendar is a WAI-ARIA grid: one Tab stop for the cells (it
// was 42, one per day), besides the paging and view buttons.
func TestDateFieldGridIsOneTabStop(t *testing.T) {
	th := kit.Dark()
	var s ui.DateFieldState
	v := time.Date(2026, 11, 20, 0, 0, 0, 0, time.Local)
	h := kittest.NewT(t, 500, 500, func(gtx kit.C) {
		if d, ok := s.Update(gtx); ok {
			v = d
		}
		gtx.Constraints.Max.X = gtx.Dp(220)
		ui.DateField{ID: "due", Value: v}.Layout(gtx, th, &s)
	})
	h.ClickID("due/field")
	stops := map[string]bool{}
	for range 20 {
		h.Tab(false)
		stops[h.FocusedID()] = true
	}
	// The field, the panel, the paging and view buttons, and the grid
	// (reported as its active day).
	want := []string{"due/field", "due", "due/prev", "due/view", "due/next", "due/2026-11-20"}
	for _, id := range want {
		if !stops[id] {
			t.Errorf("Tab never reached %q", id)
		}
	}
	if len(stops) != len(want) {
		t.Errorf("%d Tab stops in the open calendar, want %d (the grid is one): %v", len(stops), len(want), stops)
	}
	h.ExpectClean()
}
