package ui

import (
	"fmt"
	"slices"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/hhhapz/giokit/kit"
)

// DataRow has a stable ID and one text cell per column.
type DataRow struct {
	// ID names the row: unique, non-empty and stable across frames.
	ID string
	// Cells are the row's text, one per column: sorted on, spoken (joined
	// with ", ") and drawn unless DataTable.Cell is set. A missing cell is
	// empty.
	Cells []string
}

// DataColumn is one DataTable column.
type DataColumn struct {
	// ID names the column: its sort header is <table ID>/sort/<column ID>.
	// It must be unique within the table, non-empty and stable: not the
	// translated title.
	ID    string
	Title string // the header's visible and spoken text
	// Width is the column's width; 0 shares the leftover space.
	Width unit.Dp
	// Align places each cell's content: layout.W (the zero value's
	// meaning) or layout.E.
	Align layout.Direction
	// Compare, if set, orders two of the column's cell texts when sorting
	// by it: negative when a sorts before b. Use it for numbers, dates and
	// sizes. nil compares the texts case-insensitively, so "10" sorts
	// before "9".
	Compare func(a, b string) int
}

// DataTable is a table with a sticky header that sorts (click a header,
// again to reverse), aligned columns and one selected row, or several with
// Multiple; rows virtualize within the available height. Headers are
// ID/sort/<column ID>, the rows ID/rows (a grid) and each row ID/<row ID>.
//
// The rows are sorted afresh on every layout, so Rows may change between
// frames, in place or not.
type DataTable struct {
	ID      string       // the table's id; see the type doc for its parts
	Columns []DataColumn // left to right
	Rows    []DataRow    // in model order; the table sorts a copy of the order
	// Selected is the selected row's ID (controlled); "" selects nothing.
	Selected string
	// Cell, if set, draws row r's cell in column col (a status badge, a
	// progress bar). It is called for every column, whether or not r has a
	// cell there.
	Cell func(gtx layout.Context, r DataRow, col int) layout.Dimensions
	// Multiple and Chosen: as for SelectableList (ranges follow the sort order).
	Multiple bool
	Chosen   map[string]bool // the chosen IDs while Multiple; nil is none
}

// DataTableState is a DataTable's scroll, sort and selection state. The
// zero value is ready and unsorted; it must not be copied after first use.
//
// The sort is retained interaction state: header clicks change it, and the
// app may preset or read it.
type DataTableState struct {
	List kit.List // the rows' scroll position (see SelectableListState.List)
	// SortColumn is the index in Columns of the column the rows are sorted
	// by, when Sorted. An index out of range sorts nothing.
	SortColumn int
	Descending bool // sort from last to first
	// Sorted reports whether the rows are sorted: a header was clicked, or
	// the app set it.
	Sorted bool

	headers kit.Pool[kit.Clickable] // by column ID
	colIDs  []string                // the column IDs as last laid out
	cols    []kit.Column            // the layout columns, rebuilt each frame
	order   []int                   // row indexes in display order
	keys    []string                // sort keys by row index, rebuilt each frame
	sel     selection
}

// Update runs the sort headers and reports a selection change made since
// the last call (see SelectableListState.Update).
func (s *DataTableState) Update(gtx layout.Context) (SelectionEvent, bool) {
	s.sortHeaders(gtx)
	return s.sel.update(gtx)
}

// sortHeaders runs the header clicks: a new column sorts ascending, the
// same one again reverses.
func (s *DataTableState) sortHeaders(gtx layout.Context) {
	for i, id := range s.colIDs {
		if !s.headers.Get(id).Clicked(gtx) {
			continue
		}
		if s.Sorted && s.SortColumn == i {
			s.Descending = !s.Descending
		} else {
			s.SortColumn, s.Descending = i, false
		}
		s.Sorted = true
		kit.Notify(gtx)
	}
}

// sortedBy reports whether the rows are sorted by column col.
func (s *DataTableState) sortedBy(col int) bool { return s.Sorted && s.SortColumn == col }

// Layout sorts the row order by the state's sort, then draws the sticky
// header and the visible rows, filling the available size, with Selected
// (and Chosen) highlighted. It handles the rows' keys and type-ahead while
// laying out; s.Update runs the sort headers and reports the selection.
func (dt DataTable) Layout(gtx layout.Context, th *kit.Theme, s *DataTableState) layout.Dimensions {
	s.colIDs, s.cols = s.colIDs[:0], s.cols[:0]
	for _, c := range dt.Columns {
		s.colIDs = append(s.colIDs, c.ID)
		s.cols = append(s.cols, kit.Column{Title: c.Title, Width: c.Width, Align: c.Align})
	}
	defer s.headers.Sweep()
	dt.sort(s)
	s.sel.setRows(len(s.order), func(i int) (string, string) {
		r := dt.Rows[s.order[i]]
		label := ""
		if len(r.Cells) > 0 {
			label = r.Cells[0]
		}
		return r.ID, label
	})
	table := kit.Table{Cols: s.cols}
	rowsID := kit.Part(dt.ID, "rows")
	return kit.Col(gtx,
		kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			return table.Cells(gtx, func(gtx layout.Context, col int) layout.Dimensions { return dt.header(gtx, th, s, col) })
		}),
		kit.Rigid(kit.HLine(th.Divider)),
		kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			kit.SetRole(gtx, rowsID, kit.RoleGrid)
			view := selectionView{context: "table", id: rowsID, list: &s.List, selected: dt.Selected, multi: dt.Multiple, chosen: dt.Chosen}
			return s.sel.layout(gtx, view, func(gtx layout.Context) layout.Dimensions {
				return th.List(gtx, &s.List, rowsID, len(s.order), func(gtx layout.Context, i int) layout.Dimensions {
					return dt.row(gtx, th, s, table, dt.Rows[s.order[i]])
				})
			})
		}),
	)
}

// header draws column col's sort button; the sorted column's label says
// which way it is sorted.
func (dt DataTable) header(gtx layout.Context, th *kit.Theme, s *DataTableState, col int) layout.Dimensions {
	c := dt.Columns[col]
	label := c.Title
	if s.sortedBy(col) {
		f := th.Strings.SortedAsc
		if s.Descending {
			f = th.Strings.SortedDesc
		}
		label = fmt.Sprintf(f, label)
	}
	return Button{ID: kit.Part(dt.ID, "sort", c.ID), Text: label, Style: Ghost}.Layout(gtx, th, s.headers.Get(c.ID))
}

// row draws one data row.
func (dt DataTable) row(gtx layout.Context, th *kit.Theme, s *DataTableState, table kit.Table, r DataRow) layout.Dimensions {
	spec := rowSpec{id: kit.Part(dt.ID, r.ID), label: strings.Join(r.Cells, ", "), role: kit.RoleRow, selected: s.sel.isOn(r.ID), padX: kit.Space2}
	return row(gtx, th, s.sel.items.Get(r.ID), spec, func(gtx layout.Context) layout.Dimensions {
		return table.Cells(gtx, func(gtx layout.Context, col int) layout.Dimensions {
			if dt.Cell != nil {
				return dt.Cell(gtx, r, col)
			}
			if col >= len(r.Cells) {
				return layout.Dimensions{}
			}
			return th.Text(gtx, r.Cells[col], th.Type.Body, th.Fg, kit.Regular)
		})
	})
}

// sort fills s.order with the row indexes in display order: the given
// order, or stably sorted by the state's column.
func (dt DataTable) sort(s *DataTableState) {
	s.order = s.order[:0]
	for i := range dt.Rows {
		s.order = append(s.order, i)
	}
	col := s.SortColumn
	if !s.Sorted || col < 0 || col >= len(dt.Columns) {
		return
	}
	cell := func(r DataRow) string {
		if col < len(r.Cells) {
			return r.Cells[col]
		}
		return ""
	}
	sign := 1
	if s.Descending {
		sign = -1
	}
	if cmp := dt.Columns[col].Compare; cmp != nil {
		slices.SortStableFunc(s.order, func(a, b int) int { return sign * cmp(cell(dt.Rows[a]), cell(dt.Rows[b])) })
		return
	}
	// Lower-case each cell once, rather than in every comparison.
	s.keys = s.keys[:0]
	for _, r := range dt.Rows {
		s.keys = append(s.keys, strings.ToLower(cell(r)))
	}
	slices.SortStableFunc(s.order, func(a, b int) int { return sign * strings.Compare(s.keys[a], s.keys[b]) })
}
