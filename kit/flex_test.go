package kit

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func flexGtx(max image.Point) layout.Context {
	return layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: max}}
}

func box(w, h int) layout.FlexChild {
	return Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: image.Pt(w, h)} })
}

func TestFlexGapPadDensity(t *testing.T) {
	th := Light()
	for _, tc := range []struct {
		density float32
		want    image.Point
	}{{1, image.Pt(10+8+10+8+10+2*16, 5+2*4)}, {2, image.Pt(10+16+10+16+10+2*32, 5+2*8)}} {
		th.Density = tc.density
		d := Flex{Gap: Space2, Pad: Space1, PadX: Space4}.Layout(flexGtx(image.Pt(500, 500)), th, box(10, 5), box(10, 5), box(10, 5))
		if d.Size != tc.want {
			t.Errorf("density %v: size %v, want %v", tc.density, d.Size, tc.want)
		}
	}
	th.Density = 1
	if d := (Flex{Axis: layout.Vertical, Gap: Space2}).Layout(flexGtx(image.Pt(500, 500)), th, box(10, 5)); d.Size != image.Pt(10, 5) {
		t.Errorf("one child gets no gap: %v", d.Size)
	}
}

func TestFlexSizes(t *testing.T) {
	th := Light()
	gtx := func() layout.Context { return flexGtx(image.Pt(300, 200)) }
	for name, tc := range map[string]struct {
		f    Flex
		want image.Point
	}{
		"content":  {Flex{}, image.Pt(10, 5)},
		"fillW":    {Flex{FillW: true}, image.Pt(300, 5)},
		"fillH":    {Flex{FillH: true}, image.Pt(10, 200)},
		"exact":    {Flex{W: 50, H: 40}, image.Pt(50, 40)},
		"minW":     {Flex{MinW: 80}, image.Pt(80, 5)},
		"maxW":     {Flex{FillW: true, MaxW: 120}, image.Pt(120, 5)},
		"exactCap": {Flex{W: 900}, image.Pt(300, 5)},
		"minCap":   {Flex{MinH: 900}, image.Pt(10, 200)},
	} {
		if d := tc.f.Layout(gtx(), th, box(10, 5)); d.Size != tc.want {
			t.Errorf("%s: size %v, want %v", name, d.Size, tc.want)
		}
	}
}

func TestFlexJustifyFills(t *testing.T) {
	th := Light()
	d := Flex{FillW: true, Justify: layout.SpaceBetween, Align: layout.Middle, Bg: th.Panel, Border: th.Border}.Layout(flexGtx(image.Pt(100, 100)), th, box(10, 20), box(10, 4))
	if d.Size != image.Pt(100, 20) {
		t.Errorf("size %v, want 100x20", d.Size)
	}
}
