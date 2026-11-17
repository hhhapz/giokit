package kit

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"
)

// The spacing scale, GPUI/Tailwind style: SpaceN is N×4dp, with half steps
// spelled 0p5, 1p5, 2p5, 3p5. Use these instead of hand-picked dp values; pass
// them through Theme.Space (or kit.Flex, which does it) so Density applies.
const (
	Space0p5 unit.Dp = 2
	Space1   unit.Dp = 4
	Space1p5 unit.Dp = 6
	Space2   unit.Dp = 8
	Space2p5 unit.Dp = 10
	Space3   unit.Dp = 12
	Space3p5 unit.Dp = 14
	Space4   unit.Dp = 16
	Space5   unit.Dp = 20
	Space6   unit.Dp = 24
	Space8   unit.Dp = 32
	Space10  unit.Dp = 40
	Space12  unit.Dp = 48
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
	Axis    layout.Axis
	Gap     unit.Dp // between children
	Align   layout.Alignment
	Justify layout.Spacing

	Pad        unit.Dp // all sides
	PadX, PadY unit.Dp // override Pad on one axis when non-zero

	Bg, Border color.NRGBA // drawn with th.Styled (theme stroke width)
	Radius     unit.Dp

	W, H       unit.Dp // exact size; 0 leaves it to the constraints
	MinW, MaxW unit.Dp // 0 is no limit
	MinH, MaxH unit.Dp
	FillW      bool // take the full available width
	FillH      bool // take the full available height
}

// Layout lays out children with f's style. Gaps go between children, so a
// zero-size child still gets one on each side; leave it out instead.
func (f Flex) Layout(gtx C, th *Theme, children ...layout.FlexChild) D {
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

	body := func(gtx C) D {
		return layout.Flex{Axis: f.Axis, Alignment: f.Align, Spacing: f.Justify}.Layout(gtx, children...)
	}
	inner := body
	if px != 0 || py != 0 {
		inner = func(gtx C) D {
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
func (f Flex) constrain(gtx C) layout.Constraints {
	c := gtx.Constraints
	axis := func(lo, hi *int, exact, minV, maxV unit.Dp, fill bool) {
		if maxV != 0 {
			*hi = max(*lo, min(*hi, gtx.Dp(maxV)))
		}
		if minV != 0 {
			*lo = max(*lo, min(gtx.Dp(minV), *hi))
		}
		if fill {
			*lo = *hi
		}
		if exact != 0 {
			v := min(gtx.Dp(exact), *hi)
			*lo, *hi = v, v
		}
		*lo = min(*lo, *hi)
	}
	axis(&c.Min.X, &c.Max.X, f.W, f.MinW, f.MaxW, f.FillW)
	axis(&c.Min.Y, &c.Max.Y, f.H, f.MinH, f.MaxH, f.FillH)
	return c
}
