package ui

import (
	"image"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/hhhapz/giokit/kit"
)

// ButtonStyle draws a button in a state: its surface (fill, border, ring)
// and content colour. The predeclared styles cover the usual variants; an
// app adds its own as a plain function.
type ButtonStyle func(th *kit.Theme, st kit.State) (kit.Surface, color.NRGBA)

// The predeclared button styles.
var (
	// Secondary is an outlined surface: the default.
	Secondary ButtonStyle = func(th *kit.Theme, st kit.State) (kit.Surface, color.NRGBA) {
		return state(th, st, kit.Surface{Bg: th.Surface, Border: th.Border}, th.Fg, th.Focus)
	}
	// Primary is an accent tint.
	Primary ButtonStyle = func(th *kit.Theme, st kit.State) (kit.Surface, color.NRGBA) {
		return state(th, st, kit.Surface{Bg: th.AccentBg, Border: th.AccentBorder}, th.Accent, th.Focus)
	}
	// Ghost has no fill until hovered (toolbars).
	Ghost ButtonStyle = func(th *kit.Theme, st kit.State) (kit.Surface, color.NRGBA) {
		fg := th.Muted
		if st.Hovered {
			fg = th.Fg
		}
		return state(th, st, kit.Surface{}, fg, th.Focus)
	}
	// Danger is destructive.
	Danger ButtonStyle = func(th *kit.Theme, st kit.State) (kit.Surface, color.NRGBA) {
		return state(th, st, kit.Surface{Bg: th.BadBg, Border: kit.WithAlpha(th.Bad, 90)}, th.Bad, th.Focus)
	}
	// Solid is a solid accent fill: the one main action. Its ring is
	// OnAccent, since an accent ring would vanish on the fill.
	Solid ButtonStyle = func(th *kit.Theme, st kit.State) (kit.Surface, color.NRGBA) {
		return state(th, st, kit.Surface{Bg: th.Accent}, th.OnAccent, th.OnAccent)
	}
)

// state finishes a style's surface: theme corners and widths, the state
// colour, the ring while focused.
func state(th *kit.Theme, st kit.State, s kit.Surface, fg, ring color.NRGBA) (kit.Surface, color.NRGBA) {
	s.Radius = th.Radius
	s = th.Styled(s)
	s.Bg = th.StateColor(s.Bg, st)
	if st.Focused {
		s.Focus = ring
	}
	return s, fg
}

// Button is a push button with text and an optional leading icon.
//
//	ui.Button{ID: "save", Text: "Save", Style: ui.Solid}.Layout(gtx, th, &v.save)
//
// Read clicks with Clickable.Clicked in the handle pass.
type Button struct {
	ID    string
	Text  string
	Icon  kit.Icon        // optional, before the text
	Label string          // spoken name when Text is not enough; default Text
	Style ButtonStyle     // nil is Secondary
	Size  kit.ControlSize // ControlDefault is the theme's size
}

func (b Button) Layout(gtx C, th *kit.Theme, c *kit.Clickable) D {
	style := b.Style
	if style == nil {
		style = Secondary
	}
	label := b.Label
	if label == "" {
		label = b.Text
	}
	return th.Press(gtx, c, b.ID, func(gtx C, st kit.State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		s, fg := style(th, st)
		return s.Layout(gtx, func(gtx C) D {
			v, h := th.ControlPad(b.Size, 7), th.Space(kit.Space3p5) // 7: one height with fields
			return layout.Inset{Top: v, Bottom: v, Left: h, Right: h}.Layout(gtx, func(gtx C) D {
				return kit.Flex{Align: layout.Middle, Gap: kit.Space1p5}.Layout(gtx, th, b.parts(th, fg)...)
			})
		})
	})
}

func (b Button) parts(th *kit.Theme, fg color.NRGBA) []layout.FlexChild {
	text := kit.Rigid(func(gtx C) D { return th.Text(gtx, b.Text, th.Type.Body, fg, kit.Medium) })
	if b.Icon == nil {
		return []layout.FlexChild{text}
	}
	return []layout.FlexChild{kit.Rigid(func(gtx C) D { return b.Icon(gtx, th.IconSize, fg) }), text}
}

// IconButton is a small square icon-only button. Label is required: it is
// the spoken name. It stays quiet until hovered, or until Active (its row
// is hovered or selected) shows it at rest.
type IconButton struct {
	ID     string
	Icon   kit.Icon
	Label  string
	Active bool
}

func (b IconButton) Layout(gtx C, th *kit.Theme, c *kit.Clickable) D {
	return th.Press(gtx, c, b.ID, func(gtx C, st kit.State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(b.Label).Add(gtx.Ops)
		s, fg := th.Styled(kit.Surface{Radius: th.RadiusSm()}), th.Dim
		if b.Active {
			s.Bg, fg = kit.LerpColor(th.Surface, th.Hover, st.Hover), th.Muted
		} else {
			s.Bg = kit.WithAlpha(th.Hover, uint8(255*st.Hover))
		}
		fg = kit.LerpColor(fg, th.Fg, st.Hover)
		if st.Focused {
			s.Focus = th.Focus
		}
		return kit.Box(gtx, th.IconButtonSize, s, func(gtx C) D { return b.Icon(gtx, th.IconSize, fg) })
	})
}

// Link is an underlined text button, for navigation inside text-heavy UI.
type Link struct {
	ID   string
	Text string
}

func (l Link) Layout(gtx C, th *kit.Theme, c *kit.Clickable) D {
	gtx.Constraints.Min.X = 0
	return th.Press(gtx, c, l.ID, func(gtx C, st kit.State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(l.Text).Add(gtx.Ops)
		fg := th.Accent
		if st.Hovered || st.Focused {
			fg = th.Fg
		}
		s := th.Styled(kit.Surface{Radius: th.Radius})
		if st.Focused {
			s.Focus = th.Focus
		}
		return s.Layout(gtx, func(gtx C) D {
			return layout.UniformInset(th.Space(kit.Space1)).Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = 0
				d := th.Text(gtx, l.Text, th.Type.Body, fg, kit.Medium)
				h := max(1, gtx.Dp(th.Stroke))
				off := op.Offset(image.Pt(0, d.Size.Y)).Push(gtx.Ops)
				paint.FillShape(gtx.Ops, fg, clip.Rect{Max: image.Pt(d.Size.X, h)}.Op())
				off.Pop()
				d.Size.Y += h
				return d
			})
		})
	})
}

// Toolbar is a row of actions that wraps onto another line when space is
// short. Each action keeps its full text and normal focus behaviour. The
// group is named by Theme.Strings.Toolbar; action ids are ID/<action ID>.
type Toolbar struct {
	ID      string
	Actions []ToolbarAction
}

// ToolbarAction is one toolbar button. Read Clickable.Clicked in the handle
// pass, as for a Button. Actions without a Clickable are skipped.
type ToolbarAction struct {
	ID, Text  string
	Icon      kit.Icon
	Style     ButtonStyle // nil is Secondary
	Clickable *kit.Clickable
	Disabled  bool
}

func (tb Toolbar) Layout(gtx C, th *kit.Theme) D {
	items := make([]kit.W, 0, len(tb.Actions))
	for _, a := range tb.Actions {
		if a.Clickable == nil {
			continue
		}
		items = append(items, func(gtx C) D {
			return th.Disable(gtx, a.Disabled, func(gtx C) D {
				return Button{ID: kit.Part(tb.ID, a.ID), Text: a.Text, Icon: a.Icon, Style: a.Style}.Layout(gtx, th, a.Clickable)
			})
		})
	}
	return kit.Group(gtx, tb.ID, th.Strings.Toolbar, func(gtx C) D {
		return kit.Flow(gtx, th.Space(kit.Space1p5), th.Space(kit.Space1p5), items...)
	})
}
