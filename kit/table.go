package kit

import (
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/unit"
)

// Column describes one table column. Width 0 means flexible (shares leftover space).
type Column struct {
	Title string  // the header text, drawn as a caption
	Width unit.Dp // fixed width; 0 shares the leftover space
	// Align places each cell's content: layout.W or layout.E. The zero
	// value, layout.NW, also means W (cells are one line, so NW and W look
	// the same).
	Align layout.Direction
}

// Table lays out a header and rows over the same columns, so they always
// line up. It is the layout primitive: it draws, but holds no selection,
// sorting or keyboard navigation. For an interactive data grid use
// ui.DataTable, which is built on it.
type Table struct {
	Cols   []Column // left to right; header and rows use the same widths
	Inset  unit.Dp  // horizontal padding inside each row
	RowPad unit.Dp  // vertical padding inside each row
}

// Cells lays out one line of cells across the columns; cell draws column
// col's content.
func (tb Table) Cells(gtx layout.Context, cell func(gtx layout.Context, col int) layout.Dimensions) layout.Dimensions {
	children := make([]layout.FlexChild, len(tb.Cols))
	for i, c := range tb.Cols {
		f := func(gtx layout.Context) layout.Dimensions {
			if c.Width > 0 {
				px := gtx.Dp(c.Width)
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
			} else {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
			}
			align := c.Align
			if align == layout.NW {
				align = layout.W
			}
			return align.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return cell(gtx, i) })
		}
		if c.Width > 0 {
			children[i] = layout.Rigid(f)
		} else {
			children[i] = layout.Flexed(1, f)
		}
	}
	return Row(gtx, children...)
}

// Header lays out the column titles as captions, followed by a rule.
func (tb Table) Header(gtx layout.Context, th *Theme) layout.Dimensions {
	return Col(gtx,
		Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: tb.Inset, Right: tb.Inset, Bottom: th.Space(Space2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return tb.Cells(gtx, func(gtx layout.Context, i int) layout.Dimensions { return th.Caption(gtx, tb.Cols[i].Title) })
			})
		}),
		Rigid(HLine(th.Border)),
	)
}

// Row draws one hoverable row with id, followed by a divider unless last
// is set (the final row). cell gets the row's hover state so controls can
// light up with the row.
func (tb Table) Row(gtx layout.Context, th *Theme, h *HoverArea, id string, last bool, cell func(gtx layout.Context, col int, hovered bool) layout.Dimensions) layout.Dimensions {
	return Col(gtx,
		Rigid(func(gtx layout.Context) layout.Dimensions {
			return h.Layout(gtx, id, func(gtx layout.Context, hovered bool) layout.Dimensions {
				bg := none
				if hovered {
					bg = th.Hover // not Panel: rows often sit on a card, which is Panel
				}
				return th.Styled(Surface{Bg: bg, Radius: th.Radius}).Layout(gtx, Stretch(Pad(tb.RowPad, tb.Inset, func(gtx layout.Context) layout.Dimensions {
					return tb.Cells(gtx, func(gtx layout.Context, i int) layout.Dimensions { return cell(gtx, i, hovered) })
				})))
			})
		}),
		Rigid(func(gtx layout.Context) layout.Dimensions {
			if last {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: tb.Inset, Right: tb.Inset}.Layout(gtx, HLine(th.Divider))
		}),
	)
}
