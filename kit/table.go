package kit

import (
	"gioui.org/layout"
	"gioui.org/unit"
)

// Column describes one table column. Width 0 means flexible (shares leftover space).
type Column struct {
	Title string
	Width unit.Dp
	Align layout.Direction // layout.W (default) or layout.E
}

// Table lays out a header and rows over the same columns, so they always line up.
type Table struct {
	Cols   []Column
	Inset  unit.Dp // horizontal padding inside each row
	RowPad unit.Dp // vertical padding inside each row
}

// Cells lays out one line of cells across the columns.
func (tb Table) Cells(gtx C, cell func(gtx C, col int) D) D {
	children := make([]layout.FlexChild, len(tb.Cols))
	for i, c := range tb.Cols {
		f := func(gtx C) D {
			if c.Width > 0 {
				px := gtx.Dp(c.Width)
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
			} else {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
			}
			align := c.Align
			if align == 0 {
				align = layout.W
			}
			return align.Layout(gtx, func(gtx C) D { return cell(gtx, i) })
		}
		if c.Width > 0 {
			children[i] = layout.Rigid(f)
		} else {
			children[i] = layout.Flexed(1, f)
		}
	}
	return Row(gtx, children...)
}

func (tb Table) Header(gtx C, t *Theme) D {
	return Col(gtx,
		Rigid(func(gtx C) D {
			return layout.Inset{Left: tb.Inset, Right: tb.Inset, Bottom: 8}.Layout(gtx, func(gtx C) D {
				return tb.Cells(gtx, func(gtx C, i int) D { return t.Caption(gtx, tb.Cols[i].Title) })
			})
		}),
		Rigid(HLine(t.Border)),
	)
}

// Row draws one hoverable row followed by a divider (unless last).
// cell gets the row's hover state so controls can light up with the row.
func (tb Table) Row(gtx C, t *Theme, h *Hover, id string, last bool, cell func(gtx C, col int, hovered bool) D) D {
	return Col(gtx,
		Rigid(func(gtx C) D {
			return h.Layout(gtx, id, func(gtx C, hovered bool) D {
				bg := none
				if hovered {
					bg = t.Hover // not Panel: rows often sit on a card, which is Panel
				}
				return t.Styled(Surface{Bg: bg, Radius: t.Radius}).Layout(gtx, Stretch(Pad(tb.RowPad, tb.Inset, func(gtx C) D {
					return tb.Cells(gtx, func(gtx C, i int) D { return cell(gtx, i, hovered) })
				})))
			})
		}),
		Rigid(func(gtx C) D {
			if last {
				return D{}
			}
			return layout.Inset{Left: tb.Inset, Right: tb.Inset}.Layout(gtx, HLine(t.Divider))
		}),
	)
}
