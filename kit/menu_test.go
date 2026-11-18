package kit

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// TestMenuRowWidthMatchesRow: a row laid out at MenuRowWidth gives its
// content exactly the width the content measured, at any density and icon
// size, so popups sized to their widest row neither clip nor pad it.
func TestMenuRowWidthMatchesRow(t *testing.T) {
	const contentW = 50
	for _, tc := range []struct {
		density float32
		icon    unit.Dp
	}{{1, 14}, {1.5, 20}, {0.8, 12}} {
		th := Light()
		th.Density, th.IconSize = tc.density, tc.icon
		got := -1
		r := MenuRow{Icon: IconPlus, Label: "Paste", Hint: "Ctrl+V", Checked: true,
			Content: func(gtx layout.Context) layout.Dimensions {
				got = gtx.Constraints.Max.X
				return layout.Dimensions{Size: image.Pt(contentW, 10)}
			}}
		var router input.Router
		gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1.25, PxPerSp: 1.25},
			Constraints: layout.Constraints{Max: image.Pt(1000, 1000)}, Source: router.Source()}
		w := th.MenuRowWidth(gtx, r)
		gtx.Constraints = layout.Constraints{Max: image.Pt(w, 1000)}
		var c Clickable
		th.MenuRow(gtx, &c, "row", r)
		if got != contentW {
			t.Errorf("density %v, icon %v: content got width %d, want %d", tc.density, tc.icon, got, contentW)
		}
	}
}

// TestMenuRowWidthMeasuresWithoutInput: measuring runs Content with input
// disabled, so a callback that reads events cannot consume them twice.
func TestMenuRowWidthMeasuresWithoutInput(t *testing.T) {
	th := Light()
	var router input.Router
	gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(1000, 1000)}, Source: router.Source()}
	calls, enabled := 0, false
	th.MenuRowWidth(gtx, MenuRow{Label: "x", Content: func(gtx layout.Context) layout.Dimensions {
		calls++
		enabled = gtx.Enabled()
		return layout.Dimensions{}
	}})
	if calls != 1 || enabled {
		t.Errorf("measure: %d calls, enabled %v; want 1 call with input disabled", calls, enabled)
	}
}
