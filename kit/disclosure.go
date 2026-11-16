package kit

import (
	"gioui.org/io/semantic"
	"gioui.org/layout"
)

// Collapsible is the state of a Disclosure: Open, and its header button.
type Collapsible struct {
	Clickable
	Open bool
}

// Disclosure is a section with a clickable header (chevron, title, an
// optional trailing count) that shows or hides its body. Click or press
// Enter/Space on the header to toggle; the header id is id.
func (t *Theme) Disclosure(gtx C, c *Collapsible, id, title, trailing string, body W) D {
	if c.Clicked(gtx) {
		c.Open = !c.Open
	}
	return Col(gtx,
		Rigid(func(gtx C) D {
			return t.press(gtx, &c.Clickable, id, func(gtx C, st State) D {
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(title).Add(gtx.Ops)
				semantic.SelectedOp(c.Open).Add(gtx.Ops)
				s := t.Styled(Surface{Bg: WithAlpha(t.Hover, uint8(255*st.Hover)), Radius: t.Radius})
				if st.Focused {
					s.Focus = t.Focus
				}
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return s.Layout(gtx, func(gtx C) D {
					return layout.Inset{Top: t.Space(6), Bottom: t.Space(6), Left: t.Space(4), Right: t.Space(8)}.Layout(gtx, func(gtx C) D {
						chev := IconChevronRight
						if c.Open {
							chev = IconChevronDown
						}
						return Row(gtx,
							Rigid(func(gtx C) D { return chev(gtx, 14, t.Muted) }),
							Rigid(Gap(6, 0)),
							Rigid(func(gtx C) D { return t.Caption(gtx, title) }),
							Rigid(Gap(8, 0)),
							Rigid(func(gtx C) D {
								if trailing == "" {
									return D{}
								}
								return t.Pill(gtx, trailing, t.Muted, t.Surface)
							}),
						)
					})
				})
			})
		}),
		Rigid(func(gtx C) D {
			if !c.Open {
				return D{}
			}
			return layout.Inset{Top: t.Space(8)}.Layout(gtx, body)
		}),
	)
}
