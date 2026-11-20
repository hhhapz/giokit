package ui

import (
	"image"
	"image/color"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"

	"github.com/hhhapz/giokit/kit"
)

// ButtonStyle draws a button in a state: its surface (fill, border, ring)
// and content colour. Secondary (the default), Primary, Ghost, Danger and
// Solid cover the usual variants; an app adds its own as a plain function.
type ButtonStyle func(th *kit.Theme, st kit.State) (kit.Surface, color.NRGBA)

// Secondary is an outlined surface: the default.
func Secondary(th *kit.Theme, st kit.State) (kit.Surface, color.NRGBA) {
	return state(th, st, kit.Surface{Bg: th.Surface, Border: th.Border}, th.Fg, th.Focus)
}

// Primary is an accent tint.
func Primary(th *kit.Theme, st kit.State) (kit.Surface, color.NRGBA) {
	return state(th, st, kit.Surface{Bg: th.AccentBg, Border: th.AccentBorder}, th.Accent, th.Focus)
}

// Ghost has no fill until hovered (toolbars).
func Ghost(th *kit.Theme, st kit.State) (kit.Surface, color.NRGBA) {
	fg := th.Muted
	if st.Hovered {
		fg = th.Fg
	}
	return state(th, st, kit.Surface{}, fg, th.Focus)
}

// dangerBorderAlpha is the opacity of Danger's Bad border over its BadBg
// fill: a visible edge that stays quieter than the label.
const dangerBorderAlpha = 90

// Danger is destructive.
func Danger(th *kit.Theme, st kit.State) (kit.Surface, color.NRGBA) {
	return state(th, st, kit.Surface{Bg: th.BadBg, Border: kit.WithAlpha(th.Bad, dangerBorderAlpha)}, th.Bad, th.Focus)
}

// Solid is a solid accent fill: the one main action. Its ring is
// OnAccent, since an accent ring would vanish on the fill.
func Solid(th *kit.Theme, st kit.State) (kit.Surface, color.NRGBA) {
	return state(th, st, kit.Surface{Bg: th.Accent}, th.OnAccent, th.OnAccent)
}

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
	ID    string          // the button's semantic id
	Text  string          // the visible text
	Icon  kit.Icon        // optional, before the text
	Label string          // spoken name when Text is not enough; default Text
	Style ButtonStyle     // nil is Secondary
	Size  kit.ControlSize // ControlDefault is the theme's size
}

// Layout draws the button with Style for c's hover, press and focus state.
// It registers c as the press and focus target; read clicks with
// c.Clicked in the handle pass. Inside th.Disable it is dimmed and inert.
func (b Button) Layout(gtx layout.Context, th *kit.Theme, c *kit.Clickable) layout.Dimensions {
	style := b.Style
	if style == nil {
		style = Secondary
	}
	label := b.Label
	if label == "" {
		label = b.Text
	}
	return th.Press(gtx, c, b.ID, func(gtx layout.Context, st kit.State) layout.Dimensions {
		describe(gtx, kit.RoleButton, label)
		s, fg := style(th, st)
		return s.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			v, h := th.FieldPad(b.Size), th.Space(kit.Space3p5) // one height with fields
			return layout.Inset{Top: v, Bottom: v, Left: h, Right: h}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return kit.Flex{Align: layout.Middle, Gap: kit.Space1p5}.Layout(gtx, th, b.parts(th, fg)...)
			})
		})
	})
}

// parts are the button's row contents in fg: the icon, if any, then the
// text.
func (b Button) parts(th *kit.Theme, fg color.NRGBA) []layout.FlexChild {
	text := kit.Rigid(func(gtx layout.Context) layout.Dimensions { return th.Text(gtx, b.Text, th.Type.Body, fg, kit.Medium) })
	if b.Icon == nil {
		return []layout.FlexChild{text}
	}
	return []layout.FlexChild{kit.Rigid(func(gtx layout.Context) layout.Dimensions { return b.Icon(gtx, th.IconSize, fg) }), text}
}

// IconButton is a small square icon-only button. It stays quiet until
// hovered, or until Active (its row is hovered or selected) shows it at
// rest.
type IconButton struct {
	ID     string   // the button's semantic id
	Icon   kit.Icon // nil draws an empty button
	Label  string   // the spoken name; required, since there is no text
	Active bool     // draw the icon at rest, not only while hovered
}

// Layout draws the icon in a square hit target for c's state. It registers
// c as the press and focus target; read clicks with c.Clicked.
func (b IconButton) Layout(gtx layout.Context, th *kit.Theme, c *kit.Clickable) layout.Dimensions {
	return th.Press(gtx, c, b.ID, func(gtx layout.Context, st kit.State) layout.Dimensions {
		describe(gtx, kit.RoleButton, b.Label)
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
		return kit.Box(gtx, th.IconButtonSize, s, func(gtx layout.Context) layout.Dimensions {
			if b.Icon == nil {
				return layout.Dimensions{}
			}
			return b.Icon(gtx, th.IconSize, fg)
		})
	})
}

// Link is an underlined text button, for navigation inside text-heavy UI.
type Link struct {
	ID   string // the link's semantic id
	Text string // the visible and spoken text
}

// Layout draws Text underlined, in the accent colour at rest and in Fg
// while hovered or focused. It registers c as the press and focus target;
// read clicks with c.Clicked. It opens nothing itself.
func (l Link) Layout(gtx layout.Context, th *kit.Theme, c *kit.Clickable) layout.Dimensions {
	gtx.Constraints.Min.X = 0
	return th.Press(gtx, c, l.ID, func(gtx layout.Context, st kit.State) layout.Dimensions {
		describe(gtx, kit.RoleLink, l.Text)
		fg := th.Accent
		if st.Hovered || st.Focused {
			fg = th.Fg
		}
		s := th.Styled(kit.Surface{Radius: th.Radius})
		if st.Focused {
			s.Focus = th.Focus
		}
		return s.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(th.Space(kit.Space1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
//
//	if id, ok := v.tools.Update(gtx); ok { ... }   // handle pass
//	ui.Toolbar{ID: "tools", Actions: actions}.Layout(gtx, th, &v.tools)
type Toolbar struct {
	ID      string          // the group's id; actions are ID/<action ID>
	Actions []ToolbarAction // left to right, wrapping
}

// ToolbarAction is one toolbar button, reported by ToolbarState.Update
// with its ID. IDs must be unique within the toolbar and stable.
type ToolbarAction struct {
	ID, Text string      // stable id within the toolbar, and visible text
	Icon     kit.Icon    // optional, before the text
	Style    ButtonStyle // nil is Secondary
	Disabled bool        // dimmed, out of the Tab order, never reported
}

// ToolbarState holds a Toolbar's buttons, one per action ID. Buttons of
// actions that leave the toolbar are dropped a few frames later. The zero
// value is ready to use; do not copy it after first use.
type ToolbarState struct {
	buttons kit.Pool[kit.Clickable]
	last    []string // the action ids laid out last
}

// Update returns the ID of an action clicked since the last call. Call it
// in the handle pass; a second click in the same frame is returned by the
// next call.
func (s *ToolbarState) Update(gtx layout.Context) (string, bool) {
	for _, id := range s.last {
		if s.buttons.Get(id).Clicked(gtx) {
			return id, true
		}
	}
	return "", false
}

// Layout draws one Button per action from s's buttons, wrapping onto new
// lines, and records the toolbar group. It remembers the actions for
// s.Update, which reports clicks; it drops buttons of departed actions.
func (tb Toolbar) Layout(gtx layout.Context, th *kit.Theme, s *ToolbarState) layout.Dimensions {
	s.last = s.last[:0]
	items := make([]layout.Widget, 0, len(tb.Actions))
	for _, a := range tb.Actions {
		s.last = append(s.last, a.ID)
		c := s.buttons.Get(a.ID)
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return th.Disable(gtx, a.Disabled, func(gtx layout.Context) layout.Dimensions {
				return Button{ID: kit.Part(tb.ID, a.ID), Text: a.Text, Icon: a.Icon, Style: a.Style}.Layout(gtx, th, c)
			})
		})
	}
	s.buttons.Sweep()
	return kit.Group(gtx, tb.ID, th.Strings.Toolbar, kit.RoleToolbar, func(gtx layout.Context) layout.Dimensions {
		return kit.Flow(gtx, th.Space(kit.Space1p5), th.Space(kit.Space1p5), items...)
	})
}
