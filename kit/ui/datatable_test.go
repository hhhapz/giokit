package ui_test

import (
	"cmp"
	"strconv"
	"testing"

	"github.com/hhhapz/giokit/gio/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// A sort column out of range sorts nothing instead of panicking.
func TestDataTableSortColumnOutOfRange(t *testing.T) {
	th := kit.Dark()
	rows := []ui.DataRow{{ID: "b", Cells: []string{"Bravo"}}, {ID: "a", Cells: []string{"Alpha"}}}
	for _, col := range []int{-1, 5} {
		s := ui.DataTableState{SortColumn: col, Sorted: true}
		h := kittest.NewT(t, 300, 200, func(gtx layout.Context) {
			s.Update(gtx)
			ui.DataTable{ID: "t", Columns: []ui.DataColumn{{ID: "name", Title: "Name"}}, Rows: rows}.Layout(gtx, th, &s)
		})
		if a, b := h.ExpectID("t/a"), h.ExpectID("t/b"); a.Min.Y < b.Min.Y {
			t.Errorf("SortColumn %d reordered the rows", col)
		}
		h.ExpectText("Name")
	}
}

// A column's Compare orders its cells (numbers here); without one the
// cells sort as case-insensitive text. Header ids follow column ids.
func TestDataTableCompare(t *testing.T) {
	th := kit.Dark()
	var s ui.DataTableState
	rows := []ui.DataRow{{ID: "ten", Cells: []string{"ten", "10"}}, {ID: "nine", Cells: []string{"Nine", "9"}}}
	numeric := func(a, b string) int {
		x, _ := strconv.Atoi(a)
		y, _ := strconv.Atoi(b)
		return cmp.Compare(x, y)
	}
	cols := []ui.DataColumn{{ID: "name", Title: "Name"}, {ID: "count", Title: "Count", Compare: numeric}}
	h := kittest.NewT(t, 400, 200, func(gtx layout.Context) {
		s.Update(gtx)
		ui.DataTable{ID: "t", Columns: cols, Rows: rows}.Layout(gtx, th, &s)
	})
	first := func() string {
		if h.ExpectID("t/nine").Min.Y < h.ExpectID("t/ten").Min.Y {
			return "nine"
		}
		return "ten"
	}
	h.ClickID("t/sort/count")
	if got := first(); got != "nine" {
		t.Errorf("numeric sort: %s first, want nine (9 < 10)", got)
	}
	h.ClickID("t/sort/count")
	if got := first(); got != "ten" {
		t.Errorf("numeric sort, descending: %s first, want ten", got)
	}
	h.ClickID("t/sort/name")
	if got := first(); got != "nine" {
		t.Errorf("text sort: %s first, want Nine before ten", got)
	}
	h.ExpectClean()
}
