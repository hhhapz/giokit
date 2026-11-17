package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
)

// Badge is a pill coloured by status.
type Badge struct {
	Status kit.Status
	Text   string
}

func (b Badge) Layout(gtx C, th *kit.Theme) D {
	fg, bg, _ := th.StatusColors(b.Status)
	return th.Pill(gtx, b.Text, fg, bg)
}

// Banner is an inline alert: status icon, bold title, optional body.
type Banner struct {
	Status      kit.Status
	Title, Body string
}

func (b Banner) Layout(gtx C, th *kit.Theme) D {
	fg, bg, icon := th.StatusColors(b.Status)
	return th.Styled(kit.Surface{Bg: bg, Border: kit.WithAlpha(fg, 60), Radius: th.Radius}).Layout(gtx, kit.Stretch(kit.Pad(th.Space(kit.Space2p5), th.Space(kit.Space3), func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			kit.Rigid(func(gtx C) D {
				return layout.Inset{Top: 1, Right: th.Space(kit.Space2p5)}.Layout(gtx, func(gtx C) D { return icon(gtx, th.IconSizeLg, fg) })
			}),
			kit.Flexed(1, func(gtx C) D {
				return kit.Col(gtx,
					kit.Rigid(func(gtx C) D { return th.Text(gtx, b.Title, th.Type.Body, th.Fg, kit.SemiBold) }),
					kit.Rigid(func(gtx C) D {
						if b.Body == "" {
							return D{}
						}
						return layout.Inset{Top: 2}.Layout(gtx, func(gtx C) D { return th.Paragraph(gtx, b.Body, th.Muted) })
					}),
				)
			}),
		)
	})))
}

// Kbd renders a key chord as key caps: "Ctrl+K" is [Ctrl] [K]. Show a
// bound shortcut with Kbd{Chord: keymap.Hint("Save")}.
type Kbd struct{ Chord string }

func (k Kbd) Layout(gtx C, th *kit.Theme) D {
	keys := strings.Split(k.Chord, "+")
	caps := make([]layout.FlexChild, 0, len(keys))
	for _, key := range keys {
		caps = append(caps, kit.Rigid(func(gtx C) D {
			return th.Styled(kit.Surface{Bg: th.Surface, Border: th.Border, Radius: th.RadiusSm()}).Layout(gtx, func(gtx C) D {
				return layout.Inset{Top: 1, Bottom: 2, Left: 5, Right: 5}.Layout(gtx, func(gtx C) D {
					return th.Text(gtx, key, th.Type.Small, th.Muted, kit.Medium)
				})
			})
		}))
	}
	return kit.Flex{Align: layout.Middle, Gap: kit.Space1 - 1}.Layout(gtx, th, caps...)
}

// Stat is a card with a caption and a big value; it fills the width (use
// it inside Flexed for equal-width rows).
type Stat struct {
	Title, Value string
	ValueColor   color.NRGBA // zero is Fg
}

func (s Stat) Layout(gtx C, th *kit.Theme) D {
	vc := s.ValueColor
	if vc.A == 0 {
		vc = th.Fg
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return th.Styled(kit.Surface{Bg: th.Panel, Border: th.Divider, Radius: th.RadiusLg()}).Layout(gtx, kit.Stretch(kit.Pad(th.Space(kit.Space3), th.Space(kit.Space3p5), func(gtx C) D {
		return kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return th.Caption(gtx, s.Title) }),
			kit.Rigid(kit.Gap(0, th.Space(kit.Space1))),
			kit.Rigid(func(gtx C) D { return th.Text(gtx, s.Value, th.Type.Stat, vc, kit.SemiBold) }),
		)
	})))
}

// Artwork draws square, cropped artwork, or a placeholder while the image
// is not set. Label describes the image to screen readers and tests.
type Artwork struct {
	ID, Label string
	Size      unit.Dp
}

func (a Artwork) Layout(gtx C, th *kit.Theme, im *kit.Image) D {
	px := gtx.Dp(a.Size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	r := gtx.Dp(th.Radius)
	defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Max}, r).Push(gtx.Ops).Pop()
	kit.TagLabel(gtx, a.ID, a.Label)
	semantic.Image.Add(gtx.Ops)
	if src, ok := im.Op(); ok {
		return widget.Image{Src: src, Fit: widget.Cover, Position: layout.Center}.Layout(gtx)
	}
	paint.FillShape(gtx.Ops, th.AccentBg, clip.Rect{Max: gtx.Constraints.Max}.Op())
	return D{Size: gtx.Constraints.Max}
}

// Split is a resizable two-pane layout. Drag the divider, or focus it (Tab)
// and use the arrow keys. The divider's id is ID.
type Split struct {
	ID   string
	Axis layout.Axis // Horizontal: panes side by side; Vertical: stacked
	Min  unit.Dp     // minimum pane size; 0 is Theme.PaneMinSize
	A, B kit.W
}

// SplitState holds the divider position (Ratio: the first pane's share; 0
// is half). Save Ratio with your app state to keep the layout.
type SplitState struct {
	Ratio float32

	drag  gesture.Drag
	hover gesture.Hover
	grab  float32 // pointer offset inside the handle at press
	start int     // handle position at press (px)
	pos   int     // handle position last frame (px)
	hov   kit.Tween
}

func (sp Split) Layout(gtx C, th *kit.Theme, s *SplitState) D {
	if s.Ratio == 0 {
		s.Ratio = 0.5
	}
	total := gtx.Constraints.Max.X
	if sp.Axis == layout.Vertical {
		total = gtx.Constraints.Max.Y
	}
	minPx := gtx.Dp(sp.Min)
	if sp.Min == 0 {
		minPx = gtx.Dp(th.PaneMinSize)
	}
	hit := gtx.Dp(th.HitSize)              // a usable target; the visible line stays thin
	along := func(p image.Point) float32 { // coordinate along the split axis
		if sp.Axis == layout.Vertical {
			return float32(p.Y)
		}
		return float32(p.X)
	}
	set := func(px float32) {
		if total <= 0 {
			return
		}
		lo, hi := float32(minPx), float32(total-minPx)
		if hi < lo {
			lo, hi = float32(total)/2, float32(total)/2
		}
		s.Ratio = max(lo, min(hi, px)) / float32(total)
		kit.Notify(gtx)
	}

	// Input before drawing.
	axis := gesture.Horizontal
	if sp.Axis == layout.Vertical {
		axis = gesture.Vertical
	}
	for {
		e, ok := s.drag.Update(gtx.Metric, gtx.Source, axis)
		if !ok {
			break
		}
		// Gio reports a drag relative to the handle as it was at press time
		// (the grab), not as it is now: anchor to the press position.
		p := along(e.Position.Round())
		switch e.Kind {
		case pointer.Press:
			s.grab, s.start = p, s.pos
		case pointer.Drag:
			set(float32(s.start) + p - s.grab + float32(hit)/2)
		}
	}
	for {
		e, ok := gtx.Event(
			key.FocusFilter{Target: s},
			key.Filter{Focus: s, Name: key.NameLeftArrow}, key.Filter{Focus: s, Name: key.NameRightArrow},
			key.Filter{Focus: s, Name: key.NameUpArrow}, key.Filter{Focus: s, Name: key.NameDownArrow},
		)
		if !ok {
			break
		}
		if ke, ok := e.(key.Event); ok && ke.State == key.Press {
			step := float32(total) * 0.05
			switch ke.Name {
			case key.NameLeftArrow, key.NameUpArrow:
				set(s.Ratio*float32(total) - step)
			case key.NameRightArrow, key.NameDownArrow:
				set(s.Ratio*float32(total) + step)
			}
		}
	}
	hovered := s.hover.Update(gtx.Source)

	first := int(s.Ratio * float32(total))
	first = max(min(first, total-minPx), min(minPx, total/2))
	s.pos = first - hit/2
	pane := func(w kit.W, off, size int) {
		gtx := gtx
		var o image.Point
		if sp.Axis == layout.Vertical {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, size))
			o = image.Pt(0, off)
		} else {
			gtx.Constraints = layout.Exact(image.Pt(size, gtx.Constraints.Max.Y))
			o = image.Pt(off, 0)
		}
		st := op.Offset(o).Push(gtx.Ops)
		cl := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
		w(gtx)
		cl.Pop()
		st.Pop()
	}
	line := max(1, gtx.Dp(th.Stroke))
	pane(sp.A, 0, first)
	pane(sp.B, first+line, total-first-line)

	// Divider line, highlighted while hovered, dragged or focused.
	focused := gtx.Focused(s)
	active := float32(0)
	if hovered || s.drag.Dragging() || focused {
		active = 1
	}
	ha := s.hov.To(gtx, active, th.Dur(kit.DurFast))
	maxX, maxY := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
	lineRect, hitRect := image.Rect(first, 0, first+line, maxY), image.Rect(s.pos, 0, s.pos+hit, maxY)
	glow := image.Rect(first-1, 0, first+2, maxY)
	cursor := pointer.CursorColResize
	if sp.Axis == layout.Vertical {
		lineRect, hitRect = image.Rect(0, first, maxX, first+line), image.Rect(0, s.pos, maxX, s.pos+hit)
		glow = image.Rect(0, first-1, maxX, first+2)
		cursor = pointer.CursorRowResize
	}
	paint.FillShape(gtx.Ops, th.Border, clip.Rect(lineRect).Op())
	if ha > 0 {
		c := th.Accent
		if focused {
			c = th.Focus
		}
		paint.FillShape(gtx.Ops, kit.WithAlpha(c, uint8(255*ha)), clip.Rect(glow).Op())
	}
	area := clip.Rect(hitRect).Push(gtx.Ops)
	cursor.Add(gtx.Ops)
	s.drag.Add(gtx.Ops)
	s.hover.Add(gtx.Ops)
	event.Op(gtx.Ops, s)
	kit.Tag(gtx, sp.ID) // the drag listens for presses, so this area keeps its semantics
	area.Pop()
	return D{Size: gtx.Constraints.Max}
}
