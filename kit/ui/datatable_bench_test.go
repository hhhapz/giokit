package ui_test

import (
	"github.com/hhhapz/giokit/gio/layout"

	"fmt"
	"testing"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// BenchmarkDataTableSorted measures a frame of a large sorted table: the
// table sorts its rows on every layout, since callers may change Rows in
// place between frames.
func BenchmarkDataTableSorted(b *testing.B) {
	th := kit.Dark()
	rows := make([]ui.DataRow, 10000)
	for i := range rows {
		rows[i] = ui.DataRow{ID: fmt.Sprint(i), Cells: []string{fmt.Sprintf("Item %05d", (i*7919)%len(rows)), "Active"}}
	}
	s := ui.DataTableState{Sorted: true}
	h, err := kittest.New(640, 480, 1, func(gtx layout.Context) {
		s.Update(gtx)
		ui.DataTable{ID: "t", Columns: []ui.DataColumn{{ID: "name", Title: "Name"}, {ID: "status", Title: "Status"}}, Rows: rows}.Layout(gtx, th, &s)
	})
	if err != nil {
		kittest.NoGPU(b, err)
	}
	defer h.Release()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.Settle()
	}
}
