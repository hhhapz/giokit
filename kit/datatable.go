package kit

import (
	"slices"
	"strings"

	"gioui.org/io/semantic"
)

// DataRow has a stable ID and one text cell per Table column.
type DataRow struct {
	ID    string
	Cells []string
}

// DataTable adds a sticky header, sorting, and row selection to Table's
// aligned columns. The List virtualizes rows within the available height.
type DataTable struct {
	Table      Table
	List       List
	Selected   string
	SortColumn int
	Descending bool
	sorted     bool
	headers    Pool[Clickable]
	sel        selection
	changed    bool
}

func (dt *DataTable) Changed() (string, bool) {
	id, ok := dt.Selected, dt.changed
	dt.changed = false
	return id, ok
}

func (t *Theme) DataTable(gtx C, dt *DataTable, id string, input []DataRow) D {
	for i := range dt.Table.Cols {
		if dt.headers.Get(itoa(i)).Clicked(gtx) {
			if dt.sorted && dt.SortColumn == i {
				dt.Descending = !dt.Descending
			} else {
				dt.SortColumn, dt.Descending = i, false
			}
			dt.sorted = true
		}
	}
	rows := append([]DataRow(nil), input...)
	if dt.sorted {
		slices.SortStableFunc(rows, func(a, b DataRow) int {
			cell := func(r DataRow) string {
				if dt.SortColumn < len(r.Cells) {
					return r.Cells[dt.SortColumn]
				}
				return ""
			}
			c := strings.Compare(strings.ToLower(cell(a)), strings.ToLower(cell(b)))
			if dt.Descending {
				return -c
			}
			return c
		})
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	clicks := dt.sel.handle(gtx, ids, &dt.Selected, &dt.changed)
	return Col(gtx,
		Rigid(func(gtx C) D {
			return dt.Table.Cells(gtx, func(gtx C, col int) D {
				label := dt.Table.Cols[col].Title
				if dt.sorted && dt.SortColumn == col {
					if dt.Descending {
						label += " (desc)"
					} else {
						label += " (asc)"
					}
				}
				return t.Button(gtx, dt.headers.Get(itoa(col)), Part(id, "sort", itoa(col)), label, Ghost)
			})
		}),
		Rigid(HLine(t.Divider)),
		Flexed(1, func(gtx C) D {
			return dt.sel.layout(gtx, "table", Part(id, "rows"), &dt.List, ids, &dt.Selected, &dt.changed, nil, func(gtx C) D {
				return t.List(gtx, &dt.List, Part(id, "rows"), len(rows), func(gtx C, i int) D {
					row := rows[i]
					return t.press(gtx, clicks[i], Part(id, row.ID), func(gtx C, st State) D {
						semantic.Button.Add(gtx.Ops)
						semantic.LabelOp(strings.Join(row.Cells, ", ")).Add(gtx.Ops)
						semantic.SelectedOp(dt.Selected == row.ID).Add(gtx.Ops)
						bg := t.Surface
						if dt.Selected == row.ID {
							bg = t.AccentBg
						} else if st.Hovered {
							bg = t.Panel
						}
						surf := t.Styled(Surface{Bg: bg, Radius: t.Radius})
						if st.Focused {
							surf.Focus = t.Focus
						}
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return surf.Layout(gtx, Pad(t.control(7), t.Space(8), func(gtx C) D {
							return dt.Table.Cells(gtx, func(gtx C, col int) D {
								if col >= len(row.Cells) {
									return D{}
								}
								return t.Text(gtx, row.Cells[col], t.Type.Body, t.Fg, Regular)
							})
						}))
					})
				})
			})
		}),
	)
}
