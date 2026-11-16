package kit

import (
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Text draws a single line of text in the UI typeface.
func (t *Theme) Text(gtx C, s string, size unit.Sp, c color.NRGBA, w font.Weight) D {
	l := material.Label(t.mat, size, s)
	l.Color = c
	l.Font.Weight = w
	l.MaxLines = 1
	return l.Layout(gtx)
}

// Paragraph draws wrapping body text.
func (t *Theme) Paragraph(gtx C, s string, c color.NRGBA) D {
	l := material.Label(t.mat, t.Type.Body, s)
	l.Color = c
	l.LineHeightScale = 1.35
	return l.Layout(gtx)
}

// ParagraphLine is wrapping body text with a selectable font weight.
func (t *Theme) ParagraphLine(gtx C, s string, c color.NRGBA, w font.Weight) D {
	l := material.Label(t.mat, t.Type.Body, s)
	l.Color = c
	l.Font.Weight = w
	l.LineHeightScale = 1.35
	return l.Layout(gtx)
}

// Code draws a single line in the monospace typeface (logs, ids, paths).
func (t *Theme) Code(gtx C, s string, size unit.Sp, c color.NRGBA) D {
	l := material.Label(t.mat, size, s)
	l.Color = c
	l.Font.Typeface = t.Mono
	l.MaxLines = 1
	return l.Layout(gtx)
}

// CodeBlock is Code that wraps: monospace paragraphs (previews, output).
func (t *Theme) CodeBlock(gtx C, s string, size unit.Sp, c color.NRGBA) D {
	l := material.Label(t.mat, size, s)
	l.Color = c
	l.Font.Typeface = t.Mono
	return l.Layout(gtx)
}

// Heading is a page or section title.
func (t *Theme) Heading(gtx C, s string) D {
	return t.Text(gtx, s, t.Type.Title, t.Fg, t.HeadingWeight)
}

// Caption is the small uppercase label used for section and column headings.
func (t *Theme) Caption(gtx C, s string) D {
	return t.Text(gtx, strings.ToUpper(s), t.Type.Caption, t.Dim, Bold)
}

// Pill is a rounded status badge.
func (t *Theme) Pill(gtx C, s string, fg, bg color.NRGBA) D {
	return t.Styled(Surface{Bg: bg, Radius: t.RadiusFull()}).Layout(gtx, func(gtx C) D {
		return layout.Inset{Top: 2, Bottom: 3, Left: 9, Right: 9}.Layout(gtx, func(gtx C) D {
			return t.Text(gtx, s, t.Type.Small, fg, Medium)
		})
	})
}

// Card is a padded panel with a hairline outline.
func (t *Theme) Card(gtx C, w W) D {
	return t.Styled(Surface{Bg: t.Panel, Border: t.Divider, Radius: t.RadiusLg()}).Layout(gtx, Pad(t.Space(14), t.Space(16), w))
}

type ButtonKind int

const (
	Secondary ButtonKind = iota // outlined surface
	Primary                     // accent tint
	Ghost                       // no fill until hovered (toolbars)
	Danger                      // destructive
	Solid                       // solid accent fill, the one main action
)

// PressTint is how far a pressed button's fill moves toward Fg (hover goes
// half as far). Palette.Legible keeps button text readable on it.
const PressTint = 0.16

// tint mixes c toward the foreground: a hover/press tint that works on light
// and dark themes alike.
func (t *Theme) tint(c color.NRGBA, amount float32) color.NRGBA {
	if c.A == 0 {
		return WithAlpha(t.Fg, uint8(amount*255))
	}
	return LerpColor(c, t.Fg, amount)
}

func (t *Theme) buttonColors(kind ButtonKind) (Surface, color.NRGBA) {
	switch kind {
	case Primary:
		return t.Styled(Surface{Bg: t.AccentBg, Border: t.AccentBorder, Radius: t.Radius}), t.Accent
	case Ghost:
		return t.Styled(Surface{Radius: t.Radius}), t.Muted
	case Danger:
		return t.Styled(Surface{Bg: t.BadBg, Border: WithAlpha(t.Bad, 90), Radius: t.Radius}), t.Bad
	case Solid:
		return t.Styled(Surface{Bg: t.Accent, Radius: t.Radius}), t.OnAccent
	}
	return t.Styled(Surface{Bg: t.Surface, Border: t.Border, Radius: t.Radius}), t.Fg
}

// Button is a text button.
func (t *Theme) Button(gtx C, c *Clickable, id, label string, kind ButtonKind) D {
	return t.IconLabelButton(gtx, c, id, nil, label, kind)
}

// IconLabelButton is a button with a leading icon (nil for none).
func (t *Theme) IconLabelButton(gtx C, c *Clickable, id string, icon Icon, label string, kind ButtonKind) D {
	return t.press(gtx, c, id, func(gtx C, st State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		s, fg := t.buttonColors(kind)
		amt := PressTint / 2 * st.Hover
		if st.Pressed {
			amt = PressTint
		}
		s.Bg = t.tint(s.Bg, amt)
		if kind == Ghost && st.Hovered {
			fg = t.Fg
		}
		if st.Focused {
			s.Focus = t.Focus
			if kind == Solid { // an accent ring vanishes on an accent fill
				s.Focus = t.OnAccent
			}
		}
		return s.Layout(gtx, func(gtx C) D {
			return layout.Inset{Top: t.control(7), Bottom: t.control(7), Left: t.Space(14), Right: t.Space(14)}.Layout(gtx, func(gtx C) D {
				return Row(gtx,
					Rigid(func(gtx C) D {
						if icon == nil {
							return D{}
						}
						return layout.Inset{Right: 6}.Layout(gtx, func(gtx C) D { return icon(gtx, 14, fg) })
					}),
					Rigid(func(gtx C) D { return t.Text(gtx, label, t.Type.Body, fg, Medium) }),
				)
			})
		})
	})
}

func (t *Theme) Checkbox(gtx C, c *Clickable, id string, on bool) D {
	return t.CheckboxLabel(gtx, c, id, "", on)
}

// CheckboxLabel is a checkbox whose label is part of the click target.
func (t *Theme) CheckboxLabel(gtx C, c *Clickable, id, label string, on bool) D {
	return t.press(gtx, c, id, func(gtx C, st State) D {
		semantic.CheckBox.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.SelectedOp(on).Add(gtx.Ops)
		s := t.Styled(Surface{Bg: WithAlpha(t.Surface, uint8(255*st.Hover)), Border: LerpColor(t.Dim, t.Muted, st.Hover), Radius: min(t.RadiusSm(), 5)})
		if on {
			s.Bg, s.Border = t.Good, t.Good
		}
		if st.Focused {
			s.Focus = t.Focus
		}
		return t.withLabel(gtx, label, func(gtx C) D {
			return Box(gtx, 18, s, func(gtx C) D {
				if !on {
					return D{}
				}
				return IconCheck(gtx, 14, t.Bg)
			})
		})
	})
}

// IconButton is a small square icon button. Quiet until its row (active) or itself is hovered.
func (t *Theme) IconButton(gtx C, c *Clickable, id string, icon Icon, active bool) D {
	return t.IconButtonLabel(gtx, c, id, id, icon, active)
}

// IconButtonLabel is an icon button with a spoken label independent of its id.
func (t *Theme) IconButtonLabel(gtx C, c *Clickable, id, label string, icon Icon, active bool) D {
	return t.press(gtx, c, id, func(gtx C, st State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		s, fg := t.Styled(Surface{Radius: t.RadiusSm()}), t.Dim
		if active {
			s.Bg, fg = LerpColor(t.Surface, t.Hover, st.Hover), t.Muted
		} else {
			s.Bg = WithAlpha(t.Hover, uint8(255*st.Hover))
		}
		fg = LerpColor(fg, t.Fg, st.Hover)
		if st.Focused {
			s.Focus = t.Focus
		}
		return Box(gtx, 22, s, func(gtx C) D { return icon(gtx, 14, fg) })
	})
}

// Stepper is "− value +". parts <id>/minus and <id>/plus.
func (t *Theme) Stepper(gtx C, minus, plus *Clickable, id, value string, vc color.NRGBA, active bool) D {
	return Row(gtx,
		Rigid(func(gtx C) D {
			return t.IconButtonLabel(gtx, minus, Part(id, "minus"), "Decrease "+id, IconMinus, active)
		}),
		Rigid(Fixed(36, func(gtx C) D {
			return layout.Center.Layout(gtx, func(gtx C) D { return t.Text(gtx, value, t.Type.Text, vc, SemiBold) })
		})),
		Rigid(func(gtx C) D { return t.IconButtonLabel(gtx, plus, Part(id, "plus"), "Increase "+id, IconPlus, active) }),
	)
}

// NavItem is a full-width sidebar or list entry with an optional trailing
// value. Clicking focuses it (quietly: no ring); inside a FocusGroup the group takes focus, so arrows work right after.
func (t *Theme) NavItem(gtx C, c *Clickable, id, label, trailing string, tc color.NRGBA, active bool) D {
	c.FocusOnClick = true
	return t.press(gtx, c, id, func(gtx C, st State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.SelectedOp(active).Add(gtx.Ops)
		bg, fg := WithAlpha(t.Hover, uint8(255*st.Hover)), t.Muted
		if active {
			bg, fg = t.Surface, t.Fg
		}
		s := t.Styled(Surface{Bg: bg, Radius: t.Radius})
		if st.Focused {
			s.Focus = t.Focus
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return s.Layout(gtx, Pad(t.control(7), t.Space(10), func(gtx C) D {
			return Row(gtx,
				Flexed(1, func(gtx C) D { return t.Text(gtx, label, t.Type.Body, fg, Regular) }),
				Rigid(func(gtx C) D { return t.Text(gtx, trailing, t.Type.Small, tc, Medium) }),
			)
		}))
	})
}

// Stat is a card with a caption and a big value. Use inside Flexed for equal-width rows.
func (t *Theme) Stat(title, value string, vc color.NRGBA) W {
	return Stretch(func(gtx C) D {
		return t.Styled(Surface{Bg: t.Panel, Border: t.Divider, Radius: t.RadiusLg()}).Layout(gtx, Stretch(Pad(t.Space(12), t.Space(14), func(gtx C) D {
			return Col(gtx,
				Rigid(func(gtx C) D { return t.Caption(gtx, title) }),
				Rigid(Gap(0, 4)),
				Rigid(func(gtx C) D { return t.Text(gtx, value, t.Type.Stat, vc, SemiBold) }),
			)
		})))
	})
}

// Bar is a full-width panel strip (title bar, status bar, toolbars).
func (t *Theme) Bar(gtx C, v, h unit.Dp, w W) D {
	return Fill(gtx, t.Panel, Stretch(Pad(t.Space(v), t.Space(h), w)))
}
