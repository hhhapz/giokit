package kit

import (
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Progress is a thin full-width bar. value in 0..1; a negative value draws an
// indeterminate bar (a sliding segment; static when Motion is 0).
func (t *Theme) Progress(gtx C, value float32) D {
	h := gtx.Dp(4)
	w := gtx.Constraints.Max.X
	sz := image.Pt(w, h)
	paint.FillShape(gtx.Ops, t.Surface, clip.UniformRRect(image.Rectangle{Max: sz}, h/2).Op(gtx.Ops))
	var x0, x1 int
	if value < 0 {
		p := Loop(gtx, t.Dur(1400*time.Millisecond))
		seg := w / 3
		x0 = int(float32(w+seg)*EaseInOut(p)) - seg
		x1 = x0 + seg
	} else {
		x1 = int(float32(w) * min(1, value))
	}
	defer clip.UniformRRect(image.Rectangle{Max: sz}, h/2).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, t.Accent, clip.Rect(image.Rect(max(0, x0), 0, min(w, x1), h)).Op())
	return D{Size: sz}
}

// Spinner is a rotating arc for "working…" states. It schedules frames with
// Loop, so a UI showing it still settles in kittest.
func (t *Theme) Spinner(gtx C, size unit.Dp) D {
	p := Loop(gtx, t.Dur(900*time.Millisecond))
	arc(gtx, size, WithAlpha(t.Accent, 50), 2.5, 12, 12, 9, 0, 360)
	return arc(gtx, size, t.Accent, 2.5, 12, 12, 9, 360*p-90, 100)
}

// Status selects colors for banners and badges.
type Status int

const (
	StatusInfo Status = iota
	StatusGood
	StatusWarn
	StatusBad
)

func (t *Theme) statusColors(s Status) (fg, bg color.NRGBA, icon Icon) {
	switch s {
	case StatusGood:
		return t.Good, t.GoodBg, IconCircleCheck
	case StatusWarn:
		return t.Warn, t.WarnBg, IconAlert
	case StatusBad:
		return t.Bad, t.BadBg, IconCircleX
	}
	return t.Accent, t.AccentBg, IconInfo
}

// Badge is a Pill colored by status.
func (t *Theme) Badge(gtx C, s Status, text string) D {
	fg, bg, _ := t.statusColors(s)
	return t.Pill(gtx, text, fg, bg)
}

// Banner is an inline alert: icon, bold title, optional body.
func (t *Theme) Banner(gtx C, s Status, title, body string) D {
	fg, bg, icon := t.statusColors(s)
	return t.Styled(Surface{Bg: bg, Border: WithAlpha(fg, 60), Radius: t.Radius}).Layout(gtx, Stretch(Pad(t.Space(10), t.Space(12), func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			Rigid(func(gtx C) D {
				return layout.Inset{Top: 1, Right: 10}.Layout(gtx, func(gtx C) D { return icon(gtx, 16, fg) })
			}),
			Flexed(1, func(gtx C) D {
				return Col(gtx,
					Rigid(func(gtx C) D { return t.Text(gtx, title, t.Type.Body, t.Fg, SemiBold) }),
					Rigid(func(gtx C) D {
						if body == "" {
							return D{}
						}
						return layout.Inset{Top: 2}.Layout(gtx, func(gtx C) D { return t.Paragraph(gtx, body, t.Muted) })
					}),
				)
			}),
		)
	})))
}

// Kbd renders a key chord as key caps: "Ctrl+K" → [Ctrl] [K].
func (t *Theme) Kbd(gtx C, chord string) D {
	keys := strings.Split(chord, "+")
	var children []layout.FlexChild
	for i, k := range keys {
		if i > 0 {
			children = append(children, Rigid(Gap(3, 0)))
		}
		children = append(children, Rigid(func(gtx C) D {
			return t.Styled(Surface{Bg: t.Surface, Border: t.Border, Radius: t.RadiusSm()}).Layout(gtx, func(gtx C) D {
				return layout.Inset{Top: 1, Bottom: 2, Left: 5, Right: 5}.Layout(gtx, func(gtx C) D {
					return t.Text(gtx, k, t.Type.Small, t.Muted, Medium)
				})
			})
		}))
	}
	return Row(gtx, children...)
}

// Tooltip shows text below its child after the pointer rests on it for a
// moment. State per tooltip; draws above everything (op.Defer).
type Tooltip struct {
	hover   gesture.Hover
	since   time.Time
	showing bool
	fade    Tween
}

// TooltipDelay is how long the pointer must rest before a tooltip shows.
const TooltipDelay = 500 * time.Millisecond

func (t *Theme) Tooltip(gtx C, tip *Tooltip, text string, w W) D {
	hovered := tip.hover.Update(gtx.Source)
	switch {
	case !hovered:
		tip.since, tip.showing = time.Time{}, false
	case tip.since.IsZero():
		tip.since = gtx.Now
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(TooltipDelay)})
	case !tip.showing && gtx.Now.Sub(tip.since) >= TooltipDelay:
		tip.showing = true
		tip.fade.From(gtx, 0, t.Dur(DurFast))
	case !tip.showing:
		gtx.Execute(op.InvalidateCmd{At: tip.since.Add(TooltipDelay)})
	}
	rec := op.Record(gtx.Ops)
	dims := w(gtx)
	child := rec.Stop()
	area := clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops)
	tip.hover.Add(gtx.Ops) // an ancestor of the child's areas, so clicks still reach it
	child.Add(gtx.Ops)
	area.Pop()
	if !tip.showing {
		return dims
	}
	a := tip.fade.To(gtx, 1, t.Dur(DurFast))
	m := op.Record(gtx.Ops)
	op.Offset(image.Pt(0, dims.Size.Y+gtx.Dp(6))).Add(gtx.Ops)
	o := paint.PushOpacity(gtx.Ops, a)
	gtx.Constraints.Min = image.Point{}
	t.Styled(Surface{Bg: t.Overlay, Border: t.Border, Radius: t.Radius}).Layout(gtx, func(gtx C) D {
		return layout.Inset{Top: 4, Bottom: 5, Left: 8, Right: 8}.Layout(gtx, func(gtx C) D {
			return t.Text(gtx, text, t.Type.Small, t.Fg, Regular)
		})
	})
	o.Pop()
	op.Defer(gtx.Ops, m.Stop())
	return dims
}
