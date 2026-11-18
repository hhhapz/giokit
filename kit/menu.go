package kit

import (
	"gioui.org/io/semantic"
	"gioui.org/layout"
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
	// Content, if set, is drawn instead of the label text (Label is still
	// spoken). It may run more than once per frame: MenuRowWidth measures it
	// with input disabled and its operations discarded. It must only draw:
	// state it changes stays changed, so it must not read events or count
	// its calls.
	Content layout.Widget
	Role    Role // RoleMenuItem by default; RoleOption in selects and pickers
}

// menuRowGeometry is a MenuRow's spacing at a theme's density, control size
// and icon size. MenuRow draws with it and MenuRowWidth adds it up, so the
// two always agree.
type menuRowGeometry struct {
	// Text sits a little high in its line box, so a row has one step more
	// padding below than above to look centred.
	padTop, padBottom unit.Dp
	padX              unit.Dp // each side
	icon              unit.Dp // leading icon and check mark
	iconGap           unit.Dp // after the leading icon
	hintGap           unit.Dp // before the hint
	checkGap          unit.Dp // before the check mark
}

func (t *Theme) menuRowGeometry() menuRowGeometry {
	return menuRowGeometry{
		padTop:    t.control(5),
		padBottom: t.control(6),
		padX:      t.Space(Space2),
		icon:      t.IconSize,
		iconGap:   t.Space(Space2),
		hintGap:   t.Space(Space4),
		checkGap:  t.Space(Space3),
	}
}

// MenuRow lays out r as a pressable row (lit on hover, focus or
// Highlighted) filling the width. Ids and clicks are c's, as for a button.
func (t *Theme) MenuRow(gtx layout.Context, c *Clickable, id string, r MenuRow) layout.Dimensions {
	g := t.menuRowGeometry()
	return t.Press(gtx, c, id, func(gtx layout.Context, st State) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(r.Label).Add(gtx.Ops)
		role := r.Role
		if role == "" {
			role = RoleMenuItem
		}
		SetRole(gtx, id, role)
		fg := t.Fg
		if r.Danger {
			fg = t.Bad
		}
		lit := st.Hover
		if st.Focused || st.Pressed || r.Highlighted {
			lit = 1
		}
		s := t.Styled(Surface{Bg: t.Highlight(lit), Radius: t.RadiusSm()})
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		inset := layout.Inset{Top: g.padTop, Bottom: g.padBottom, Left: g.padX, Right: g.padX}
		return s.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return Row(gtx,
					Rigid(func(gtx layout.Context) layout.Dimensions {
						if r.Icon == nil {
							return layout.Dimensions{}
						}
						return layout.Inset{Right: g.iconGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return r.Icon(gtx, g.icon, fg) })
					}),
					Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if r.Content != nil {
							return r.Content(gtx)
						}
						return t.Text(gtx, r.Label, t.Type.Body, fg, Regular)
					}),
					Rigid(func(gtx layout.Context) layout.Dimensions {
						if r.Hint == "" {
							return layout.Dimensions{}
						}
						return layout.Inset{Left: g.hintGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return t.Text(gtx, r.Hint, t.Type.Small, t.Dim, Regular) })
					}),
					Rigid(func(gtx layout.Context) layout.Dimensions {
						if !r.Checked {
							return layout.Dimensions{}
						}
						return layout.Inset{Left: g.checkGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return IconCheck(gtx, g.icon, t.Accent) })
					}),
				)
			})
		})
	})
}

// MenuRowWidth is the width in pixels r needs, to size a popup to its widest
// row. It measures r.Content, if set, by running it (see MenuRow.Content).
func (t *Theme) MenuRowWidth(gtx layout.Context, r MenuRow) int {
	g := t.menuRowGeometry()
	var content layout.Dimensions
	if r.Content != nil {
		content = measure(gtx, r.Content)
	} else {
		content = t.measureText(gtx, r.Label, t.Type.Body)
	}
	w := 2*gtx.Dp(g.padX) + content.Size.X
	if r.Icon != nil {
		w += gtx.Dp(g.icon) + gtx.Dp(g.iconGap)
	}
	if r.Hint != "" {
		w += gtx.Dp(g.hintGap) + t.measureText(gtx, r.Hint, t.Type.Small).Size.X
	}
	if r.Checked {
		w += gtx.Dp(g.checkGap) + gtx.Dp(g.icon)
	}
	return w
}

// measureText is the size of s as one line of UI text.
func (t *Theme) measureText(gtx layout.Context, s string, size unit.Sp) layout.Dimensions {
	if s == "" {
		return layout.Dimensions{}
	}
	return measure(gtx, func(gtx layout.Context) layout.Dimensions { return t.Text(gtx, s, size, t.Fg, Regular) })
}
