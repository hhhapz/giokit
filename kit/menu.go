package kit

import (
	"image"
	"strconv"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// MenuRow is one row of a menu-like list: menu items, select options,
// combobox suggestions, command palette results.
type MenuRow struct {
	Icon        Icon   // optional, before the label
	Label       string // also the spoken name
	Hint        string // shortcut or detail, dim, at the end
	Danger      bool   // destructive: drawn in the bad colour
	Checked     bool   // a check mark: the selected option
	Highlighted bool   // lit without focus: a combobox's keyboard choice
}

// MenuRow lays out r as a pressable row (lit on hover, focus or
// Highlighted) filling the width. Ids and clicks are c's, as for a button.
func (t *Theme) MenuRow(gtx C, c *Clickable, id string, r MenuRow) D {
	icon, label, hint, danger, checked, hi := r.Icon, r.Label, r.Hint, r.Danger, r.Checked, r.Highlighted
	return t.Press(gtx, c, id, func(gtx C, st State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		fg := t.Fg
		if danger {
			fg = t.Bad
		}
		lit := st.Hover
		if st.Focused || st.Pressed || hi {
			lit = 1
		}
		s := t.Styled(Surface{Bg: t.Highlight(lit), Radius: t.RadiusSm()})
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return s.Layout(gtx, func(gtx C) D {
			return layout.Inset{Top: t.control(5), Bottom: t.control(6), Left: t.Space(Space2), Right: t.Space(Space2)}.Layout(gtx, func(gtx C) D {
				return Row(gtx,
					Rigid(func(gtx C) D {
						if icon == nil {
							return D{}
						}
						return layout.Inset{Right: t.Space(Space2)}.Layout(gtx, func(gtx C) D { return icon(gtx, t.IconSize, fg) })
					}),
					Flexed(1, func(gtx C) D { return t.Text(gtx, label, t.Type.Body, fg, Regular) }),
					Rigid(func(gtx C) D {
						if hint == "" {
							return D{}
						}
						return layout.Inset{Left: t.Space(Space4)}.Layout(gtx, func(gtx C) D { return t.Text(gtx, hint, t.Type.Small, t.Dim, Regular) })
					}),
					Rigid(func(gtx C) D {
						if !checked {
							return D{}
						}
						return layout.Inset{Left: t.Space(Space3)}.Layout(gtx, func(gtx C) D { return IconCheck(gtx, t.IconSize, t.Accent) })
					}),
				)
			})
		})
	})
}

// MenuRowWidth is the width r needs, to size a popup to its widest row.
func (t *Theme) MenuRowWidth(gtx C, r MenuRow) int {
	icon, label, hint, checked := r.Icon != nil, r.Label, r.Hint, r.Checked
	text := func(s string, size unit.Sp) int {
		if s == "" {
			return 0
		}
		m := op.Record(gtx.Ops) // measured, then dropped
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
		d := t.Text(g, s, size, t.Fg, Regular)
		m.Stop()
		return d.Size.X
	}
	w := gtx.Dp(t.Space(Space2))*2 + text(label, t.Type.Body)
	if icon {
		w += gtx.Dp(14 + 8)
	}
	if hint != "" {
		w += gtx.Dp(16) + text(hint, t.Type.Small)
	}
	if checked {
		w += gtx.Dp(12 + 14)
	}
	return w
}

// Field draws field chrome around w: full width, the control padding for
// the theme's control size, surface and border, a hover tint from
// st.Hover, and the focus ring while st.Focused. Field-like triggers
// (select, date) are built on it; TextField draws the same chrome.
func (t *Theme) Field(gtx C, st State, w W) D {
	s := t.Styled(Surface{Bg: t.StateColor(t.Surface, State{Hover: st.Hover}), Border: t.Border, Radius: t.Radius})
	if st.Focused {
		s.Focus = t.Focus
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return s.Layout(gtx, func(gtx C) D {
		return layout.Inset{Top: t.control(7), Bottom: t.control(7), Left: t.Space(Space2p5), Right: t.Space(Space2)}.Layout(gtx, w)
	})
}

func itoa(i int) string { return strconv.Itoa(i) }
