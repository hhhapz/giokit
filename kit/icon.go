package kit

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Icon draws a vector glyph in a size×size square. Prefer icons over symbol
// characters: the bundled fonts cover Latin text, not ✓ ↻ ▾ ▸ (tofu boxes).
type Icon func(gtx C, size unit.Dp, c color.NRGBA) D

// Icons are built from filled outlines, not clip.Stroke: Gio re-flattens
// strokes on the CPU every frame. A polyline becomes one quad per segment
// plus a round dot per vertex (round caps and joins), all in one path whose
// contours share a winding, so they union under the nonzero fill rule.

// stroke draws a polyline in a 24-unit design grid scaled to size.
func stroke(gtx C, size unit.Dp, c color.NRGBA, width float32, pts ...f32.Point) D {
	px := gtx.Dp(size)
	s := float32(px) / 24
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
	return D{Size: image.Pt(px, px)}
}

// circleContour appends a clockwise circle.
func circleContour(p *clip.Path, c f32.Point, r float32) {
	rrectContour(p, rectF{c.X - r, c.Y - r, c.X + r, c.Y + r}, r, false)
}

// arc draws a circular arc (degrees, 0 = 3 o'clock, clockwise) on the
// 24-grid as a filled band with round caps.
func arc(gtx C, size unit.Dp, c color.NRGBA, width float32, cx, cy, r, from, sweep float32) D {
	px := gtx.Dp(size)
	s := float32(px) / 24
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
		n := max(2, int(sweep/10))
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
	return D{Size: image.Pt(px, px)}
}

func dot(gtx C, size unit.Dp, c color.NRGBA, cx, cy, r float32) {
	s := float32(gtx.Dp(size)) / 24
	rect := image.Rect(int((cx-r)*s), int((cy-r)*s), int((cx+r)*s+0.5), int((cy+r)*s+0.5))
	paint.FillShape(gtx.Ops, c, clip.Ellipse(rect).Op(gtx.Ops))
}

var (
	IconCheck = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		return stroke(gtx, size, c, 3, f32.Pt(5, 12.5), f32.Pt(10, 17.5), f32.Pt(19, 7.5))
	})
	IconChevronDown = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		return stroke(gtx, size, c, 2.5, f32.Pt(6, 9), f32.Pt(12, 15), f32.Pt(18, 9))
	})
	IconChevronRight = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		return stroke(gtx, size, c, 2.5, f32.Pt(9, 6), f32.Pt(15, 12), f32.Pt(9, 18))
	})
	IconChevronLeft = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		return stroke(gtx, size, c, 2.5, f32.Pt(15, 6), f32.Pt(9, 12), f32.Pt(15, 18))
	})
	IconChevronUp = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		return stroke(gtx, size, c, 2.5, f32.Pt(6, 15), f32.Pt(12, 9), f32.Pt(18, 15))
	})
	IconMinus = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		return stroke(gtx, size, c, 2.5, f32.Pt(6, 12), f32.Pt(18, 12))
	})
	IconPlus = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		stroke(gtx, size, c, 2.5, f32.Pt(6, 12), f32.Pt(18, 12))
		return stroke(gtx, size, c, 2.5, f32.Pt(12, 6), f32.Pt(12, 18))
	})
	IconX = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		stroke(gtx, size, c, 2.5, f32.Pt(6, 6), f32.Pt(18, 18))
		return stroke(gtx, size, c, 2.5, f32.Pt(18, 6), f32.Pt(6, 18))
	})
	IconArrowDown = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		stroke(gtx, size, c, 2.5, f32.Pt(12, 5), f32.Pt(12, 19))
		return stroke(gtx, size, c, 2.5, f32.Pt(6, 13), f32.Pt(12, 19), f32.Pt(18, 13))
	})
	IconSearch = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		arc(gtx, size, c, 2.5, 10.5, 10.5, 6, 0, 360)
		return stroke(gtx, size, c, 2.5, f32.Pt(15, 15), f32.Pt(20, 20))
	})
	IconMenu = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		stroke(gtx, size, c, 2.5, f32.Pt(4, 6), f32.Pt(20, 6))
		stroke(gtx, size, c, 2.5, f32.Pt(4, 12), f32.Pt(20, 12))
		return stroke(gtx, size, c, 2.5, f32.Pt(4, 18), f32.Pt(20, 18))
	})
	// IconMore is three dots: "more actions" (row and toolbar overflow menus).
	IconMore = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		for _, x := range []float32{5, 12, 19} {
			dot(gtx, size, c, x, 12, 1.9)
		}
		return D{Size: image.Pt(gtx.Dp(size), gtx.Dp(size))}
	})
	IconInfo = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		arc(gtx, size, c, 2, 12, 12, 9, 0, 360)
		stroke(gtx, size, c, 2.2, f32.Pt(12, 11), f32.Pt(12, 17))
		dot(gtx, size, c, 12, 7.5, 1.4)
		return D{Size: image.Pt(gtx.Dp(size), gtx.Dp(size))}
	})
	IconAlert = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		stroke(gtx, size, c, 2, f32.Pt(12, 3.5), f32.Pt(21.5, 20), f32.Pt(2.5, 20), f32.Pt(12, 3.5))
		stroke(gtx, size, c, 2.2, f32.Pt(12, 9.5), f32.Pt(12, 14))
		dot(gtx, size, c, 12, 17, 1.3)
		return D{Size: image.Pt(gtx.Dp(size), gtx.Dp(size))}
	})
	IconCircleCheck = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		arc(gtx, size, c, 2, 12, 12, 9, 0, 360)
		return stroke(gtx, size, c, 2.2, f32.Pt(8, 12.5), f32.Pt(11, 15.5), f32.Pt(16.5, 9.5))
	})
	IconCircleX = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		arc(gtx, size, c, 2, 12, 12, 9, 0, 360)
		stroke(gtx, size, c, 2.2, f32.Pt(9, 9), f32.Pt(15, 15))
		return stroke(gtx, size, c, 2.2, f32.Pt(15, 9), f32.Pt(9, 15))
	})
	IconSun = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		arc(gtx, size, c, 2, 12, 12, 4, 0, 360)
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			cs, sn := float32(math.Cos(a)), float32(math.Sin(a))
			stroke(gtx, size, c, 2, f32.Pt(12+7*cs, 12+7*sn), f32.Pt(12+9.5*cs, 12+9.5*sn))
		}
		return D{Size: image.Pt(gtx.Dp(size), gtx.Dp(size))}
	})
	IconLayout = Icon(func(gtx C, size unit.Dp, c color.NRGBA) D {
		stroke(gtx, size, c, 2, f32.Pt(4, 4), f32.Pt(20, 4), f32.Pt(20, 20), f32.Pt(4, 20), f32.Pt(4, 4))
		return stroke(gtx, size, c, 2, f32.Pt(10, 4), f32.Pt(10, 20))
	})
)
