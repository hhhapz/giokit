package kit

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// TabStyle picks how Tabs draws its selection.
type TabStyle int

const (
	TabsUnderline TabStyle = iota // text tabs with a sliding underline
	TabsSegmented                 // a segmented control: a sliding pill on a track
)

// Tabs is the state for a row of tabs: one Clickable per tab and the
// animated indicator. Read clicks with Clicked in the handle pass.
type Tabs struct {
	Style  TabStyle
	clicks Pool[Clickable]
	x, w   Tween
	last   []string
}

// Clicked returns the index of a tab clicked since the last call.
func (s *Tabs) Clicked(gtx C) (int, bool) {
	for i, l := range s.last {
		if s.clicks.Get(l).Clicked(gtx) {
			return i, true
		}
	}
	return 0, false
}

// Tabs lays out the tabs; ids are <id>/<label>.
func (t *Theme) Tabs(gtx C, s *Tabs, id string, labels []string, selected int) D {
	s.last = labels
	gtx.Constraints.Min = image.Point{}
	seg := s.Style == TabsSegmented
	pad := t.Space(6)
	if seg {
		pad = t.Space(3)
	}
	// Lay tabs out first (recorded) to learn their positions, then draw the
	// indicator under them and replay.
	m := op.Record(gtx.Ops)
	type span struct{ x, w int }
	spans := make([]span, len(labels))
	x, h := gtx.Dp(pad), 0
	for i, l := range labels {
		st := op.Offset(image.Pt(x, gtx.Dp(pad))).Push(gtx.Ops)
		d := t.press(gtx, s.clicks.Get(l), Part(id, l), func(gtx C, st State) D {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(l).Add(gtx.Ops)
			semantic.SelectedOp(i == selected).Add(gtx.Ops)
			fg := LerpColor(t.Muted, t.Fg, st.Hover)
			if i == selected {
				fg = t.Fg
			}
			sf := t.Styled(Surface{Radius: t.RadiusSm()})
			if st.Focused {
				sf.Focus = t.Focus
			}
			if !seg && i != selected {
				sf.Bg = WithAlpha(t.Hover, uint8(160*st.Hover))
			}
			return sf.Layout(gtx, Pad(t.control(5), t.Space(12), func(gtx C) D {
				return t.Text(gtx, l, t.Type.Body, fg, Medium)
			}))
		})
		st.Pop()
		spans[i] = span{x, d.Size.X}
		x += d.Size.X
		if !seg {
			x += gtx.Dp(2)
		}
		h = max(h, d.Size.Y)
	}
	call := m.Stop()
	size := image.Pt(x+gtx.Dp(pad), h+2*gtx.Dp(pad))

	if seg {
		tr := min(gtx.Dp(t.Radius)+gtx.Dp(pad), size.Y/2)
		paint.FillShape(gtx.Ops, t.Panel, clip.UniformRRect(image.Rectangle{Max: size}, tr).Op(gtx.Ops))
		Border(gtx, image.Rectangle{Max: size}, tr, max(1, gtx.Dp(t.Stroke)), t.Border)
	}
	if selected >= 0 && selected < len(spans) {
		sp := spans[selected]
		ix := int(s.x.ToEase(gtx, float32(sp.x), t.Dur(DurNormal), EaseInOut))
		iw := int(s.w.ToEase(gtx, float32(sp.w), t.Dur(DurNormal), EaseInOut))
		if seg {
			r := image.Rect(ix, gtx.Dp(pad), ix+iw, gtx.Dp(pad)+h)
			ir := min(gtx.Dp(t.Radius), r.Dy()/2)
			paint.FillShape(gtx.Ops, t.Surface, clip.UniformRRect(r, ir).Op(gtx.Ops))
			Border(gtx, r, ir, max(1, gtx.Dp(t.Stroke)), t.Border)
		} else {
			bh := gtx.Dp(2)
			inset := gtx.Dp(8)
			r := image.Rect(ix+inset, size.Y-bh, ix+iw-inset, size.Y)
			paint.FillShape(gtx.Ops, t.Accent, clip.UniformRRect(r, bh/2).Op(gtx.Ops))
		}
	}
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}
