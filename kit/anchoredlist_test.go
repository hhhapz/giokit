package kit

import (
	"fmt"
	"image"
	"slices"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// TestAnchoredListFollowsItsAnchor: inserting items above the first visible
// one keeps that item first; appending below leaves the position alone.
func TestAnchoredListFollowsItsAnchor(t *testing.T) {
	th := Light()
	var a AnchoredList
	frame := func(ids []string) {
		gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(100, 100))}
		th.AnchoredList(gtx, &a, "feed", ids, func(gtx layout.Context, i int) layout.Dimensions { return layout.Dimensions{Size: image.Pt(100, 20)} })
	}
	var ids []string
	for i := range 50 {
		ids = append(ids, fmt.Sprint("m", i))
	}
	frame(ids)
	a.List.Position.First, a.List.Position.Offset = 10, 5
	frame(ids)

	ids = slices.Insert(ids, 0, "new1", "new2")
	frame(ids)
	if p := a.List.Position; p.First != 12 || p.Offset != 5 {
		t.Errorf("after inserting above: First/Offset %d/%d, want 12/5", p.First, p.Offset)
	}

	before := a.List.Position
	ids = append(ids, "tail")
	frame(ids)
	if p := a.List.Position; p.First != before.First || p.Offset != before.Offset || p.BeforeEnd != before.BeforeEnd {
		t.Errorf("after appending below: %+v, want %+v", p, before)
	}
}
