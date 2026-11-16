package kit

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// FormField is one labeled control. Control owns its input state and semantic
// label; use its normal component method inside the callback. ID names the
// row, while the control keeps its own stable ID for focus and tests.
type FormField struct {
	ID, Label, Help, Error string
	Control                W
}

// Form lays out labeled fields. At narrow widths it stacks labels above
// controls; at wider widths it aligns them in a two-column form. Errors and
// help text stay with their field in either layout.
func (t *Theme) Form(gtx C, id string, fields []FormField) D {
	children := make([]layout.FlexChild, 0, len(fields)*2)
	narrow := gtx.Constraints.Max.X < gtx.Dp(440)
	labelWidth := min(gtx.Dp(132), gtx.Constraints.Max.X/3)
	for i := range fields {
		f := fields[i]
		if i > 0 {
			children = append(children, Rigid(Gap(0, t.Space(12))))
		}
		children = append(children, Rigid(func(gtx C) D {
			return namedGroup(gtx, Part(id, f.ID), f.Label, func(gtx C) D {
				label := func(gtx C) D {
					return namedGroup(gtx, Part(id, f.ID, "label"), f.Label, func(gtx C) D {
						return t.Text(gtx, f.Label, t.Type.Body, t.Muted, Medium)
					})
				}
				control := func(gtx C) D {
					return Col(gtx,
						Rigid(func(gtx C) D {
							if f.Control == nil {
								return D{}
							}
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return f.Control(gtx)
						}),
						Rigid(func(gtx C) D {
							if f.Error == "" && f.Help == "" {
								return D{}
							}
							color, message := t.Dim, f.Help
							if f.Error != "" {
								color, message = t.Bad, f.Error
							}
							return layout.Inset{Top: t.Space(3)}.Layout(gtx, func(gtx C) D {
								return t.Paragraph(gtx, message, color)
							})
						}),
					)
				}
				if narrow {
					return Col(gtx,
						Rigid(label),
						Rigid(Gap(0, t.Space(4))),
						Rigid(control),
					)
				}
				return layout.Flex{Alignment: layout.Start}.Layout(gtx,
					Rigid(func(gtx C) D {
						gtx.Constraints.Min.X, gtx.Constraints.Max.X = labelWidth, labelWidth
						return label(gtx)
					}),
					Rigid(Gap(t.Space(12), 0)),
					Flexed(1, control),
				)
			})
		}))
	}
	return namedGroup(gtx, id, "Form", func(gtx C) D { return Col(gtx, children...) })
}

// ToolbarAction is one action in a toolbar. Read Clickable.Clicked in the
// handle pass, as for a normal Button. Label remains visible when wrapping.
type ToolbarAction struct {
	ID, Label string
	Icon      Icon
	Clickable *Clickable
	Kind      ButtonKind
	Disabled  bool
}

// Toolbar wraps actions onto another line when space is short. Each action
// keeps a full text label and its normal keyboard and focus behavior.
func (t *Theme) Toolbar(gtx C, id string, actions []ToolbarAction) D {
	items := make([]W, 0, len(actions))
	for i := range actions {
		a := actions[i]
		if a.Clickable == nil {
			continue
		}
		items = append(items, func(gtx C) D {
			return t.Disable(gtx, a.Disabled, func(gtx C) D {
				return t.IconLabelButton(gtx, a.Clickable, Part(id, a.ID), a.Icon, a.Label, a.Kind)
			})
		})
	}
	return namedGroup(gtx, id, "Toolbar", func(gtx C) D {
		return Flow(gtx, t.Space(6), t.Space(6), items...)
	})
}

// Link draws a text action with a persistent underline. Read c.Clicked in
// the handle pass and open the URL or view there. Gio currently has no Link
// semantic class, so this uses its Button class and an explicit spoken label.
func (t *Theme) Link(gtx C, c *Clickable, id, label string) D {
	gtx.Constraints.Min.X = 0
	return t.press(gtx, c, id, func(gtx C, st State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		fg := t.Accent
		if st.Hovered || st.Focused {
			fg = t.Fg
		}
		s := t.Styled(Surface{Radius: t.Radius})
		if st.Focused {
			s.Focus = t.Focus
		}
		return s.Layout(gtx, func(gtx C) D {
			return layout.Inset{Top: t.Space(3), Bottom: t.Space(3), Left: t.Space(3), Right: t.Space(3)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = 0
				d := t.Text(gtx, label, t.Type.Body, fg, Medium)
				h := max(1, gtx.Dp(1))
				off := op.Offset(image.Pt(0, d.Size.Y)).Push(gtx.Ops)
				paint.FillShape(gtx.Ops, fg, clip.Rect{Max: image.Pt(d.Size.X, h)}.Op())
				off.Pop()
				d.Size.Y += h
				return d
			})
		})
	})
}

// EmptyState gives an empty view a title, explanation and optional action.
// The action is a normal component, with its own focus and semantics.
func (t *Theme) EmptyState(gtx C, id, title, body string, action W) D {
	return namedGroup(gtx, id, title, func(gtx C) D {
		return layout.Center.Layout(gtx, func(gtx C) D {
			maxWidth := min(gtx.Constraints.Max.X, gtx.Dp(360))
			gtx.Constraints.Max.X = maxWidth
			return Col(gtx,
				Rigid(func(gtx C) D { return t.Text(gtx, title, t.Type.Text, t.Fg, SemiBold) }),
				Rigid(Gap(0, t.Space(5))),
				Rigid(func(gtx C) D { return t.Paragraph(gtx, body, t.Muted) }),
				Rigid(func(gtx C) D {
					if action == nil {
						return D{}
					}
					return layout.Inset{Top: t.Space(12)}.Layout(gtx, action)
				}),
			)
		})
	})
}

func namedGroup(gtx C, id, label string, w W) D {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	TagLabel(gtx, id, label)
	call.Add(gtx.Ops)
	return dims
}
