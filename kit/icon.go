package kit

import (
	"image"
	"image/color"
	"math"

	"github.com/hhhapz/giokit/gio/f32"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"
	"github.com/hhhapz/giokit/gio/unit"
)

// Icon draws a vector glyph in a size×size square and returns that size.
// Prefer icons over symbol characters: the bundled fonts cover Latin text,
// not ✓ ↻ ▾ ▸ (tofu boxes). The Icon* functions are the built-in set; any
// function of this shape is an icon.
type Icon func(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions

// iconGrid is the side of the design grid icons are drawn on: coordinates
// and line widths are in grid units, scaled so the grid fills size.
const iconGrid = 24

// gridScale is the pixels per grid unit for an icon of size.
func gridScale(gtx layout.Context, size unit.Dp) float32 { return float32(gtx.Dp(size)) / iconGrid }

// stroke draws a polyline of the given width through pts, on the design
// grid, and returns the icon's size. Despite the name it fills: icons are
// built from filled outlines, not clip.Stroke, because Gio re-flattens
// strokes on the CPU every frame. A polyline becomes one quad per segment
// plus a round dot per vertex (round caps and joins), all in one path whose
// contours share a winding, so they union under the nonzero fill rule.
func stroke(gtx layout.Context, size unit.Dp, c color.NRGBA, width float32, pts ...f32.Point) layout.Dimensions {
	px := gtx.Dp(size)
	s := gridScale(gtx, size)
	hw := width * s / 2
	var p clip.Path
	p.Begin(gtx.Ops)
	for i := range pts {
		a := pts[i].Mul(s)
		circleContour(&p, a, hw)
		if i == 0 {
			continue
		}
		b := pts[i-1].Mul(s)
		d := a.Sub(b)
		l := float32(math.Hypot(float64(d.X), float64(d.Y)))
		if l == 0 {
			continue
		}
		n := f32.Pt(d.Y/l*hw, -d.X/l*hw) // clockwise on screen, like circleContour
		p.MoveTo(b.Add(n))
		p.LineTo(a.Add(n))
		p.LineTo(a.Sub(n))
		p.LineTo(b.Sub(n))
		p.Close()
	}
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
	return layout.Dimensions{Size: image.Pt(px, px)}
}

// arcSegmentDeg is the angle, in degrees, an arc's outline spans per
// straight segment.
const arcSegmentDeg = 10

// circleContour appends a clockwise circle.
func circleContour(p *clip.Path, c f32.Point, r float32) {
	rrectContour(p, rectF{c.X - r, c.Y - r, c.X + r, c.Y + r}, r, false)
}

// arc draws a circular arc of radius r around (cx, cy) on the design grid,
// as a filled band with round caps, and returns the icon's size. from and
// sweep are in degrees, 0 at 3 o'clock, clockwise; a sweep of 360 or more
// draws a full ring.
func arc(gtx layout.Context, size unit.Dp, c color.NRGBA, width float32, cx, cy, r, from, sweep float32) layout.Dimensions {
	px := gtx.Dp(size)
	s := gridScale(gtx, size)
	ctr, rad, hw := f32.Pt(cx, cy).Mul(s), r*s, width*s/2
	var p clip.Path
	p.Begin(gtx.Ops)
	if sweep >= 360 {
		rrectContour(&p, rectF{ctr.X - rad - hw, ctr.Y - rad - hw, ctr.X + rad + hw, ctr.Y + rad + hw}, rad+hw, false)
		rrectContour(&p, rectF{ctr.X - rad + hw, ctr.Y - rad + hw, ctr.X + rad - hw, ctr.Y + rad - hw}, rad-hw, true)
	} else {
		at := func(deg, rr float32) f32.Point {
			a := float64(deg) * math.Pi / 180
			return f32.Pt(ctr.X+rr*float32(math.Cos(a)), ctr.Y+rr*float32(math.Sin(a)))
		}
		n := max(2, int(sweep/arcSegmentDeg))
		p.MoveTo(at(from, rad+hw))
		for i := 1; i <= n; i++ {
			p.LineTo(at(from+sweep*float32(i)/float32(n), rad+hw))
		}
		for i := n; i >= 0; i-- {
			p.LineTo(at(from+sweep*float32(i)/float32(n), rad-hw))
		}
		p.Close()
		circleContour(&p, at(from, rad), hw)
		circleContour(&p, at(from+sweep, rad), hw)
	}
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
	return layout.Dimensions{Size: image.Pt(px, px)}
}

// dot fills a circle of radius r around (cx, cy) on the design grid.
func dot(gtx layout.Context, size unit.Dp, c color.NRGBA, cx, cy, r float32) {
	s := gridScale(gtx, size)
	rect := image.Rect(int((cx-r)*s), int((cy-r)*s), int((cx+r)*s+0.5), int((cy+r)*s+0.5))
	paint.FillShape(gtx.Ops, c, clip.Ellipse(rect).Op(gtx.Ops))
}

// IconCheck is a check mark: a selected option or a done state.
func IconCheck(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	return stroke(gtx, size, c, 3, f32.Pt(5, 12.5), f32.Pt(10, 17.5), f32.Pt(19, 7.5))
}

// IconChevronDown is a downward chevron: opens a dropdown or expands a section.
func IconChevronDown(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	return stroke(gtx, size, c, 2.5, f32.Pt(6, 9), f32.Pt(12, 15), f32.Pt(18, 9))
}

// IconChevronRight is a rightward chevron: a collapsed section or forward navigation.
func IconChevronRight(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	return stroke(gtx, size, c, 2.5, f32.Pt(9, 6), f32.Pt(15, 12), f32.Pt(9, 18))
}

// IconChevronLeft is a leftward chevron: back navigation.
func IconChevronLeft(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	return stroke(gtx, size, c, 2.5, f32.Pt(15, 6), f32.Pt(9, 12), f32.Pt(15, 18))
}

// IconChevronUp is an upward chevron: collapses a section.
func IconChevronUp(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	return stroke(gtx, size, c, 2.5, f32.Pt(6, 15), f32.Pt(12, 9), f32.Pt(18, 15))
}

// IconMinus is a horizontal bar: remove, decrement or an indeterminate check.
func IconMinus(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	return stroke(gtx, size, c, 2.5, f32.Pt(6, 12), f32.Pt(18, 12))
}

// IconPlus is a plus sign: add or increment.
func IconPlus(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	stroke(gtx, size, c, 2.5, f32.Pt(6, 12), f32.Pt(18, 12))
	return stroke(gtx, size, c, 2.5, f32.Pt(12, 6), f32.Pt(12, 18))
}

// IconX is a cross: close or clear.
func IconX(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	stroke(gtx, size, c, 2.5, f32.Pt(6, 6), f32.Pt(18, 18))
	return stroke(gtx, size, c, 2.5, f32.Pt(18, 6), f32.Pt(6, 18))
}

// IconArrowDown is a downward arrow: download, or sort descending.
func IconArrowDown(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	stroke(gtx, size, c, 2.5, f32.Pt(12, 5), f32.Pt(12, 19))
	return stroke(gtx, size, c, 2.5, f32.Pt(6, 13), f32.Pt(12, 19), f32.Pt(18, 13))
}

// IconSearch is a magnifying glass: search.
func IconSearch(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	arc(gtx, size, c, 2.5, 10.5, 10.5, 6, 0, 360)
	return stroke(gtx, size, c, 2.5, f32.Pt(15, 15), f32.Pt(20, 20))
}

// IconMenu is three horizontal bars: a navigation menu.
func IconMenu(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	stroke(gtx, size, c, 2.5, f32.Pt(4, 6), f32.Pt(20, 6))
	stroke(gtx, size, c, 2.5, f32.Pt(4, 12), f32.Pt(20, 12))
	return stroke(gtx, size, c, 2.5, f32.Pt(4, 18), f32.Pt(20, 18))
}

// IconMore is three dots: "more actions" (row and toolbar overflow menus).
func IconMore(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	for _, x := range []float32{5, 12, 19} {
		dot(gtx, size, c, x, 12, 1.9)
	}
	return layout.Dimensions{Size: image.Pt(gtx.Dp(size), gtx.Dp(size))}
}

// IconInfo is an "i" in a circle: information.
func IconInfo(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	arc(gtx, size, c, 2, 12, 12, 9, 0, 360)
	stroke(gtx, size, c, 2.2, f32.Pt(12, 11), f32.Pt(12, 17))
	dot(gtx, size, c, 12, 7.5, 1.4)
	return layout.Dimensions{Size: image.Pt(gtx.Dp(size), gtx.Dp(size))}
}

// IconAlert is an exclamation mark in a triangle: a warning.
func IconAlert(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	stroke(gtx, size, c, 2, f32.Pt(12, 3.5), f32.Pt(21.5, 20), f32.Pt(2.5, 20), f32.Pt(12, 3.5))
	stroke(gtx, size, c, 2.2, f32.Pt(12, 9.5), f32.Pt(12, 14))
	dot(gtx, size, c, 12, 17, 1.3)
	return layout.Dimensions{Size: image.Pt(gtx.Dp(size), gtx.Dp(size))}
}

// IconCircleCheck is a check mark in a circle: success.
func IconCircleCheck(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	arc(gtx, size, c, 2, 12, 12, 9, 0, 360)
	return stroke(gtx, size, c, 2.2, f32.Pt(8, 12.5), f32.Pt(11, 15.5), f32.Pt(16.5, 9.5))
}

// IconCircleX is a cross in a circle: an error.
func IconCircleX(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	arc(gtx, size, c, 2, 12, 12, 9, 0, 360)
	stroke(gtx, size, c, 2.2, f32.Pt(9, 9), f32.Pt(15, 15))
	return stroke(gtx, size, c, 2.2, f32.Pt(15, 9), f32.Pt(9, 15))
}

// IconSun is a sun: light theme or brightness.
func IconSun(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	arc(gtx, size, c, 2, 12, 12, 4, 0, 360)
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		cs, sn := float32(math.Cos(a)), float32(math.Sin(a))
		stroke(gtx, size, c, 2, f32.Pt(12+7*cs, 12+7*sn), f32.Pt(12+9.5*cs, 12+9.5*sn))
	}
	return layout.Dimensions{Size: image.Pt(gtx.Dp(size), gtx.Dp(size))}
}

// IconSettings is a gear: settings or preferences.
func IconSettings(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	arc(gtx, size, c, 2, 12, 12, 6, 0, 360)
	arc(gtx, size, c, 2, 12, 12, 2.5, 0, 360)
	for i := 0; i < 8; i++ {
		a := float64(i)*math.Pi/4 + math.Pi/8
		cs, sn := float32(math.Cos(a)), float32(math.Sin(a))
		stroke(gtx, size, c, 3, f32.Pt(12+7.2*cs, 12+7.2*sn), f32.Pt(12+9*cs, 12+9*sn))
	}
	return layout.Dimensions{Size: image.Pt(gtx.Dp(size), gtx.Dp(size))}
}

// IconLayout is a split rectangle: a layout or panes.
func IconLayout(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	stroke(gtx, size, c, 2, f32.Pt(4, 4), f32.Pt(20, 4), f32.Pt(20, 20), f32.Pt(4, 20), f32.Pt(4, 4))
	return stroke(gtx, size, c, 2, f32.Pt(10, 4), f32.Pt(10, 20))
}
