package kit

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"
)

// Flex is the styled flex container: a row or column with gaps, padding, an
// optional surface and size limits, in one zero-value struct.
//
//	kit.Flex{Axis: layout.Vertical, Gap: kit.Space2, Pad: kit.Space4,
//		Bg: th.Panel, Border: th.Border, Radius: th.Radius,
//	}.Layout(gtx, th, kit.Rigid(title), kit.Flexed(1, body))
//
// Spacing values (Gap, Pad*) are scaled by th's Density; sizes are not.
// Align is the cross axis (zero: Start), Justify the main axis (zero:
// SpaceEnd, packed at the start); Justify only matters when the container
// is wider than its children, so pair it with FillW/FillH or W/H.
type Flex struct {
	Axis    layout.Axis      // layout.Horizontal (the zero value) or Vertical
	Gap     unit.Dp          // between children
	Align   layout.Alignment // cross axis; see the type doc
	Justify layout.Spacing   // main axis; see the type doc

	Pad        unit.Dp // all sides
	PadX, PadY unit.Dp // override Pad on one axis when non-zero

	Bg, Border color.NRGBA // drawn with th.Styled (theme stroke width)
	Radius     unit.Dp     // the surface's corner radius

	W, H       unit.Dp // exact size; 0 leaves it to the constraints
	MinW, MaxW unit.Dp // 0 is no limit
	MinH, MaxH unit.Dp // 0 is no limit
	FillW      bool    // take the full available width
	FillH      bool    // take the full available height
}

// Layout lays out children with f's style. Gaps go between children, so a
// zero-size child still gets one on each side; leave it out instead.
func (f Flex) Layout(gtx layout.Context, th *Theme, children ...layout.FlexChild) layout.Dimensions {
	gtx.Constraints = f.constrain(gtx)

	px, py := f.Pad, f.Pad
	if f.PadX != 0 {
		px = f.PadX
	}
	if f.PadY != 0 {
		py = f.PadY
	}
	px, py = th.Space(px), th.Space(py)

	if f.Gap != 0 && len(children) > 1 {
		g := th.Space(f.Gap)
		sp := layout.Spacer{Width: g}
		if f.Axis == layout.Vertical {
			sp = layout.Spacer{Height: g}
		}
		gap := layout.Rigid(sp.Layout)
		all := make([]layout.FlexChild, 0, 2*len(children)-1)
		for i, c := range children {
			if i > 0 {
				all = append(all, gap)
			}
			all = append(all, c)
		}
		children = all
	}

	body := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: f.Axis, Alignment: f.Align, Spacing: f.Justify}.Layout(gtx, children...)
	}
	inner := body
	if px != 0 || py != 0 {
		inner = func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: py, Bottom: py, Left: px, Right: px}.Layout(gtx, body)
		}
	}
	if f.Bg.A == 0 && f.Border.A == 0 {
		return inner(gtx)
	}
	return th.Styled(Surface{Bg: f.Bg, Border: f.Border, Radius: f.Radius}).Layout(gtx, inner)
}

// constrain applies the size fields to the incoming constraints, never
// exceeding the incoming maximum.
func (f Flex) constrain(gtx layout.Context) layout.Constraints {
	c := gtx.Constraints
	c.Min.X, c.Max.X = sizeAxis(gtx, c.Min.X, c.Max.X, axisSize{exact: f.W, min: f.MinW, max: f.MaxW, fill: f.FillW})
	c.Min.Y, c.Max.Y = sizeAxis(gtx, c.Min.Y, c.Max.Y, axisSize{exact: f.H, min: f.MinH, max: f.MaxH, fill: f.FillH})
	return c
}

// axisSize is Flex's size fields along one axis.
type axisSize struct {
	exact, min, max unit.Dp // 0: unset
	fill            bool
}

// sizeAxis narrows the constraint range [lo, hi] (pixels) along one axis to
// s, never beyond hi, and returns the new range.
func sizeAxis(gtx layout.Context, lo, hi int, s axisSize) (int, int) {
	if s.max != 0 {
		hi = max(lo, min(hi, gtx.Dp(s.max)))
	}
	if s.min != 0 {
		lo = max(lo, min(gtx.Dp(s.min), hi))
	}
	if s.fill {
		lo = hi
	}
	if s.exact != 0 {
		v := min(gtx.Dp(s.exact), hi)
		lo, hi = v, v
	}
	return min(lo, hi), hi
}
