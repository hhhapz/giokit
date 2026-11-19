package kit

import (
	"image"
	"testing"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/unit"
)

func measureWidget(w layout.Widget, c layout.Constraints) (got layout.Constraints, size image.Point) {
	gtx := layout.Context{Ops: new(op.Ops), Constraints: c, Metric: unit.Metric{PxPerDp: 2, PxPerSp: 2}}
	dims := w(gtx)
	return gtx.Constraints, dims.Size
}

// Fixed(0) keeps the incoming width, as documented; a width fixes it.
func TestFixedWidth(t *testing.T) {
	in := layout.Constraints{Min: image.Pt(10, 5), Max: image.Pt(300, 50)}
	var seen layout.Constraints
	probe := func(gtx layout.Context) layout.Dimensions {
		seen = gtx.Constraints
		return layout.Dimensions{}
	}
	measureWidget(Fixed(0, probe), in)
	if seen != in {
		t.Errorf("Fixed(0): constraints %v, want the incoming %v", seen, in)
	}
	measureWidget(Fixed(40, probe), in)
	if seen.Min.X != 80 || seen.Max.X != 80 || seen.Min.Y != 5 || seen.Max.Y != 50 {
		t.Errorf("Fixed(40) at 2px/dp: constraints %v, want width exactly 80", seen)
	}
}

// Grow claims the width it is given and no height.
func TestGrowIsHorizontal(t *testing.T) {
	_, size := measureWidget(Grow, layout.Constraints{Max: image.Pt(300, 50)})
	if size != image.Pt(300, 0) {
		t.Errorf("Grow size %v, want 300x0", size)
	}
}
