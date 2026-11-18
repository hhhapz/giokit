package ui_test

import (
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"

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
	h := kittest.NewT(t, 500, 500, func(gtx layout.Context) {
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

// Month paging keeps the day when the target month has it and otherwise
// takes the month's last day: January 31 pages to February 28 (29 in a
// leap year), not into March as time.AddDate would.
func TestDateFieldMonthEnds(t *testing.T) {
	th := kit.Dark()
	for _, c := range []struct {
		from time.Time
		keys []key.Name
		want string
	}{
		{time.Date(2026, 1, 31, 0, 0, 0, 0, time.Local), []key.Name{key.NamePageDown}, "due/2026-02-28"},
		{time.Date(2028, 1, 31, 0, 0, 0, 0, time.Local), []key.Name{key.NamePageDown}, "due/2028-02-29"},
		{time.Date(2026, 3, 31, 0, 0, 0, 0, time.Local), []key.Name{key.NamePageUp}, "due/2026-02-28"},
		{time.Date(2026, 1, 31, 0, 0, 0, 0, time.Local), []key.Name{key.NamePageDown, key.NamePageDown}, "due/2026-03-28"},
	} {
		var s ui.DateFieldState
		v := c.from
		h := kittest.NewT(t, 500, 500, func(gtx layout.Context) {
			if d, ok := s.Update(gtx); ok {
				v = d
			}
			gtx.Constraints.Max.X = gtx.Dp(220)
			ui.DateField{ID: "due", Value: v}.Layout(gtx, th, &s)
		})
		h.ClickID("due/field") // the open calendar focuses its grid on the value
		for _, k := range c.keys {
			h.Key(k, 0)
		}
		if got := h.FocusedID(); got != c.want {
			t.Errorf("from %s, %v: cursor on %q, want %q", c.from.Format("2006-01-02"), c.keys, got, c.want)
		}
	}
}

// The calendar works on Value's calendar day in Value's own zone, and
// returns picks in that zone, whatever the local zone is. Month and
// weekday names come from the theme's Strings.
func TestDateFieldZoneAndStrings(t *testing.T) {
	th := kit.Dark()
	th.Strings.Weekdays = [7]string{"Do", "Lu", "Ma", "Mi", "Ju", "Vi", "Sá"}
	th.Strings.Months[1] = "Feb."
	far := time.FixedZone("UTC+14", 14*60*60) // its midnight is the previous day in most zones
	var s ui.DateFieldState
	v := time.Date(2026, 3, 1, 0, 0, 0, 0, far)
	h := kittest.NewT(t, 500, 500, func(gtx layout.Context) {
		if d, ok := s.Update(gtx); ok {
			v = d
		}
		gtx.Constraints.Max.X = gtx.Dp(220)
		ui.DateField{ID: "due", Value: v}.Layout(gtx, th, &s)
	})
	h.ClickID("due/field")
	if got := h.FocusedID(); got != "due/2026-03-01" {
		t.Errorf("cursor on %q, want Value's own day", got)
	}
	h.ExpectText("Lu")
	h.ClickID("due/2026-03-05")
	if want := time.Date(2026, 3, 5, 0, 0, 0, 0, far); !v.Equal(want) || v.Location() != far {
		t.Errorf("picked %v, want %v", v, want)
	}
	h.ClickID("due/field")
	h.ClickID("due/view")
	h.ExpectText("Feb.")
}
