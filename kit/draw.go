package kit

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

type (
	C = layout.Context
	D = layout.Dimensions
	W = layout.Widget
)

var none color.NRGBA

// Surface draws a (rounded, optionally outlined) background sized to its content.
// Focus replaces the border with a ring in that color (keyboard focus).
// Components build surfaces with Theme.Styled, which fills Stroke and Ring.
type Surface struct {
	Bg, Border color.NRGBA
	Radius     unit.Dp // clamped to half the shorter side: huge values make pills
	Focus      color.NRGBA
	Stroke     unit.Dp // border width; 0 is a 1dp hairline
	Ring       unit.Dp // focus ring width; 0 is 2dp
}

func (s Surface) Layout(gtx C, w W) D {
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
		ring = 2
	}
	if stroke == 0 {
		stroke = 1
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
func Fill(gtx C, c color.NRGBA, w W) D { return Surface{Bg: c}.Layout(gtx, w) }

// Box draws a fixed square (fill + border) and centers w inside it.
// Use it for checkboxes and icon buttons; it never collapses to its content.
func Box(gtx C, size unit.Dp, s Surface, w W) D {
	px := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	return s.Layout(gtx, func(gtx C) D {
		return layout.Center.Layout(gtx, w)
	})
}

// Fixed constrains w to an exact width (0 keeps the incoming width).
func Fixed(width unit.Dp, w W) W {
	return func(gtx C) D {
		px := gtx.Dp(width)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
		return w(gtx)
	}
}

// Stretch makes w take the full available width.
func Stretch(w W) W {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return w(gtx)
	}
}

// Pad is shorthand for symmetric insets.
func Pad(v, h unit.Dp, w W) W {
	return func(gtx C) D { return layout.Inset{Top: v, Bottom: v, Left: h, Right: h}.Layout(gtx, w) }
}

// Grow is a flexible spacer for layout.Flexed that actually claims its space.
func Grow(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, 0)} }

// Gap is a fixed spacer.
func Gap(w, h unit.Dp) W { return layout.Spacer{Width: w, Height: h}.Layout }

func HLine(c color.NRGBA) W {
	return func(gtx C) D {
		sz := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: sz}.Op())
		return D{Size: sz}
	}
}

func VLine(c color.NRGBA) W {
	return func(gtx C) D {
		sz := image.Pt(gtx.Dp(1), gtx.Constraints.Max.Y)
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: sz}.Op())
		return D{Size: sz}
	}
}

// Row and Col are terse Flex constructors with middle alignment for rows.
func Row(gtx C, children ...layout.FlexChild) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func Col(gtx C, children ...layout.FlexChild) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

var (
	Rigid  = layout.Rigid
	Flexed = layout.Flexed
)

// Border fills a w-px rounded border just inside rect (radius r). It is a
// filled ring (outer contour minus inner contour), not a clip.Stroke: Gio
// re-flattens strokes on the CPU every frame, which was over half the frame
// time on control-heavy pages. Use it for every outline; r = height/2 makes
// pills and circles.
func Border(gtx C, rect image.Rectangle, r, w int, c color.NRGBA) {
	if c.A == 0 || w <= 0 || rect.Dx() <= 0 || rect.Dy() <= 0 {
		return
	}
	var p clip.Path
	p.Begin(gtx.Ops)
	rrectContour(&p, layoutF(rect), float32(r), false)
	inner := rect.Inset(w)
	if inner.Dx() > 0 && inner.Dy() > 0 {
		rrectContour(&p, layoutF(inner), float32(max(0, r-w)), true)
	}
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
}

type rectF struct{ x0, y0, x1, y1 float32 }

func layoutF(r image.Rectangle) rectF {
	return rectF{float32(r.Min.X), float32(r.Min.Y), float32(r.Max.X), float32(r.Max.Y)}
}

// rrectContour appends one closed rounded-rect contour, clockwise (on screen)
// or counter-clockwise, so an inner contour cuts a hole.
func rrectContour(p *clip.Path, b rectF, r float32, ccw bool) {
	r = min(r, (b.x1-b.x0)/2, (b.y1-b.y0)/2)
	const k = 0.5523 // cubic approximation of a quarter circle
	c := r * k
	pt := f32.Pt
	if !ccw {
		p.MoveTo(pt(b.x0+r, b.y0))
		p.LineTo(pt(b.x1-r, b.y0))
		p.CubeTo(pt(b.x1-r+c, b.y0), pt(b.x1, b.y0+r-c), pt(b.x1, b.y0+r))
		p.LineTo(pt(b.x1, b.y1-r))
		p.CubeTo(pt(b.x1, b.y1-r+c), pt(b.x1-r+c, b.y1), pt(b.x1-r, b.y1))
		p.LineTo(pt(b.x0+r, b.y1))
		p.CubeTo(pt(b.x0+r-c, b.y1), pt(b.x0, b.y1-r+c), pt(b.x0, b.y1-r))
		p.LineTo(pt(b.x0, b.y0+r))
		p.CubeTo(pt(b.x0, b.y0+r-c), pt(b.x0+r-c, b.y0), pt(b.x0+r, b.y0))
	} else {
		p.MoveTo(pt(b.x0+r, b.y0))
		p.CubeTo(pt(b.x0+r-c, b.y0), pt(b.x0, b.y0+r-c), pt(b.x0, b.y0+r))
		p.LineTo(pt(b.x0, b.y1-r))
		p.CubeTo(pt(b.x0, b.y1-r+c), pt(b.x0+r-c, b.y1), pt(b.x0+r, b.y1))
		p.LineTo(pt(b.x1-r, b.y1))
		p.CubeTo(pt(b.x1-r+c, b.y1), pt(b.x1, b.y1-r+c), pt(b.x1, b.y1-r))
		p.LineTo(pt(b.x1, b.y0+r))
		p.CubeTo(pt(b.x1, b.y0+r-c), pt(b.x1-r+c, b.y0), pt(b.x1-r, b.y0))
	}
	p.Close()
}
