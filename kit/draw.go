package kit

import (
	"image"
	"image/color"
	"maps"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// none is the transparent color. Files outside the palette may not write
// color literals (the lint forbids them), so they name it instead.
var none color.NRGBA

// hairline is the width of HLine and VLine.
const hairline unit.Dp = 1

// Surface draws a (rounded, optionally outlined) background sized to its content.
// Focus replaces the border with a ring in that color (keyboard focus).
// Components build surfaces with Theme.Styled, which fills Stroke and Ring.
type Surface struct {
	Bg, Border color.NRGBA // fill and outline; a zero colour draws nothing
	Radius     unit.Dp     // clamped to half the shorter side: huge values make pills
	Focus      color.NRGBA // when set, a focus ring in this colour replaces Border
	Stroke     unit.Dp     // border width; 0 is a 1dp hairline (New's Theme.Stroke)
	Ring       unit.Dp     // focus ring width; 0 is 2dp (New's Theme.Ring)
}

// Layout lays out w, then draws the surface behind it at w's size: the
// background, then the focus ring if Focus is set, otherwise the border.
// Transparent colors draw nothing. It returns w's dimensions.
func (s Surface) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	rect := image.Rectangle{Max: dims.Size}
	r := min(gtx.Dp(s.Radius), min(dims.Size.X, dims.Size.Y)/2)
	if s.Bg.A > 0 {
		paint.FillShape(gtx.Ops, s.Bg, clip.UniformRRect(rect, r).Op(gtx.Ops))
	}
	ring, stroke := s.Ring, s.Stroke
	if ring == 0 {
		ring = defaultRing
	}
	if stroke == 0 {
		stroke = defaultStroke
	}
	if s.Focus.A > 0 {
		Border(gtx, rect, r, max(1, gtx.Dp(ring)), s.Focus)
	} else if s.Border.A > 0 {
		Border(gtx, rect, r, max(1, gtx.Dp(stroke)), s.Border)
	}
	call.Add(gtx.Ops)
	return dims
}

// Fill paints a flat background behind w.
func Fill(gtx layout.Context, c color.NRGBA, w layout.Widget) layout.Dimensions {
	return Surface{Bg: c}.Layout(gtx, w)
}

// Box draws a fixed square (fill + border) and centers w inside it.
// Use it for checkboxes and icon buttons; it never collapses to its content.
func Box(gtx layout.Context, size unit.Dp, s Surface, w layout.Widget) layout.Dimensions {
	px := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	return s.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, w)
	})
}

// Fixed constrains w to an exact width. A width of 0 (or less) keeps the
// incoming width constraints.
func Fixed(width unit.Dp, w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if width > 0 {
			px := gtx.Dp(width)
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
		}
		return w(gtx)
	}
}

// Stretch makes w take the full available width.
func Stretch(w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return w(gtx)
	}
}

// Pad is shorthand for symmetric insets.
func Pad(v, h unit.Dp, w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: v, Bottom: v, Left: h, Right: h}.Layout(gtx, w)
	}
}

// Grow is a flexible spacer for a Row: in layout.Flexed it claims all the
// width it is given, so the children after it are pushed to the end. It
// claims no height, so in a vertical flex use a Flexed Gap instead.
func Grow(gtx layout.Context) layout.Dimensions {
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
}

// Gap is a fixed spacer.
func Gap(w, h unit.Dp) layout.Widget { return layout.Spacer{Width: w, Height: h}.Layout }

// HLine draws a 1dp horizontal rule in c across the available width: a
// divider, which unlike control borders does not follow Theme.Stroke.
func HLine(c color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		sz := image.Pt(gtx.Constraints.Max.X, gtx.Dp(hairline))
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: sz}.Op())
		return layout.Dimensions{Size: sz}
	}
}

// VLine draws a 1dp vertical rule in c down the available height; see
// HLine.
func VLine(c color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		sz := image.Pt(gtx.Dp(hairline), gtx.Constraints.Max.Y)
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: sz}.Op())
		return layout.Dimensions{Size: sz}
	}
}

// Row lays children out left to right, vertically centered.
func Row(gtx layout.Context, children ...layout.FlexChild) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// Col lays children out top to bottom, aligned to the start.
func Col(gtx layout.Context, children ...layout.FlexChild) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// Rigid is layout.Rigid: a flex child laid out at its own size.
func Rigid(w layout.Widget) layout.FlexChild { return layout.Rigid(w) }

// Flexed is layout.Flexed: a flex child given weight's share of the space
// left after the rigid children.
func Flexed(weight float32, w layout.Widget) layout.FlexChild { return layout.Flexed(weight, w) }

// Border fills a w-px rounded border just inside rect (radius r). It is a
// filled ring (outer contour minus inner contour), not a clip.Stroke: Gio
// re-flattens strokes on the CPU every frame, which was over half the frame
// time on control-heavy pages. Use it for every outline; r = height/2 makes
// pills and circles.
func Border(gtx layout.Context, rect image.Rectangle, r, w int, c color.NRGBA) {
	if c.A == 0 || w <= 0 || rect.Dx() <= 0 || rect.Dy() <= 0 {
		return
	}
	var p clip.Path
	p.Begin(gtx.Ops)
	rrectContour(&p, toRectF(rect), float32(r), false)
	inner := rect.Inset(w)
	if inner.Dx() > 0 && inner.Dy() > 0 {
		rrectContour(&p, toRectF(inner), float32(max(0, r-w)), true)
	}
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
}

// rectF is a rectangle in float pixel coordinates.
type rectF struct{ x0, y0, x1, y1 float32 }

func toRectF(r image.Rectangle) rectF {
	return rectF{x0: float32(r.Min.X), y0: float32(r.Min.Y), x1: float32(r.Max.X), y1: float32(r.Max.Y)}
}

// quarterArcK places a cubic Bézier's control points to approximate a
// quarter circle: each lies this fraction of the radius from its end point
// along the tangent.
const quarterArcK = 0.5523

// corner is one rounded corner of a contour, drawn clockwise: a line to
// from, then a quarter arc through c1 and c2 to to.
type corner struct {
	from, c1, c2, to f32.Point
}

// rrectContour appends one closed rounded-rect contour, clockwise (on screen)
// or counter-clockwise, so an inner contour cuts a hole.
func rrectContour(p *clip.Path, b rectF, r float32, ccw bool) {
	r = min(r, (b.x1-b.x0)/2, (b.y1-b.y0)/2)
	c := r * quarterArcK
	pt := f32.Pt
	// Clockwise from the top edge; the last corner ends at the start.
	corners := [4]corner{
		{from: pt(b.x1-r, b.y0), c1: pt(b.x1-r+c, b.y0), c2: pt(b.x1, b.y0+r-c), to: pt(b.x1, b.y0+r)},
		{from: pt(b.x1, b.y1-r), c1: pt(b.x1, b.y1-r+c), c2: pt(b.x1-r+c, b.y1), to: pt(b.x1-r, b.y1)},
		{from: pt(b.x0+r, b.y1), c1: pt(b.x0+r-c, b.y1), c2: pt(b.x0, b.y1-r+c), to: pt(b.x0, b.y1-r)},
		{from: pt(b.x0, b.y0+r), c1: pt(b.x0, b.y0+r-c), c2: pt(b.x0+r-c, b.y0), to: pt(b.x0+r, b.y0)},
	}
	start := corners[3].to
	p.MoveTo(start)
	if !ccw {
		for _, k := range corners {
			p.LineTo(k.from)
			p.CubeTo(k.c1, k.c2, k.to)
		}
	} else {
		// The same path backwards: each arc reversed, then the line that
		// led into it.
		for i := len(corners) - 1; i >= 0; i-- {
			k := corners[i]
			p.CubeTo(k.c2, k.c1, k.from)
			if i > 0 {
				p.LineTo(corners[i-1].to)
			}
		}
	}
	p.Close()
}

// unbounded is a constraint, in pixels, no widget reaches.
const unbounded = 1 << 20

// measure returns w's natural size: w is laid out unconstrained, with its
// operations discarded, input disabled, and outside the window's frame, so
// semantic and focus registration (Tag, SetRole, FocusID) have no effect.
// w still runs: Go state it changes stays changed, so measured callbacks
// must only draw.
func measure(gtx layout.Context, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	defer m.Stop()
	gtx = gtx.Disabled()
	gtx.Constraints = layout.Constraints{Max: image.Pt(unbounded, unbounded)}
	if _, ok := gtx.Values[frameKey]; ok {
		gtx.Values = maps.Clone(gtx.Values)
		delete(gtx.Values, frameKey)
	}
	return w(gtx)
}
