package kit_test

import (
	"fmt"
	"image"
	"testing"

	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/fonts"
	"github.com/hhhapz/giokit/kittest"
)

// Buttons, text fields and selects sit side by side in toolbars and forms:
// at every control size and with every font set they must be one height.
func TestControlsLineUp(t *testing.T) {
	for _, set := range fonts.All() {
		for _, size := range []kit.ControlSize{kit.ControlSmall, kit.ControlMedium, kit.ControlLarge} {
			t.Run(fmt.Sprintf("%s/%d", set.Name, size), func(t *testing.T) {
				th := kit.Dark().Derive(func(t *kit.Theme) { t.Controls = size })
				th.UseFonts(set)
				var b kit.Clickable
				var e widget.Editor
				var s kit.Select
				h := kittest.NewT(t, 600, 120, func(gtx kit.C) {
					b.Clicked(gtx)
					s.Changed(gtx)
					gtx.Constraints.Min = image.Point{} // natural heights, not the window's
					kit.Row(gtx,
						kit.Rigid(func(gtx kit.C) kit.D { return th.Button(gtx, &b, "b", "Save", kit.Secondary) }),
						kit.Rigid(kit.Fixed(160, func(gtx kit.C) kit.D { return th.TextField(gtx, &e, "f", "Name") })),
						kit.Rigid(kit.Fixed(160, func(gtx kit.C) kit.D { return th.Select(gtx, &s, "s", []string{"One"}, 0) })),
					)
				})
				h.Theme = th
				bh, fh, sh := h.ExpectID("b").Dy(), h.ExpectID("f").Dy(), h.ExpectID("s/field").Dy()
				t.Logf("button %d, field %d, select %d", bh, fh, sh)
				if abs(bh-fh) > 1 || abs(bh-sh) > 1 {
					t.Errorf("heights differ: button %d, field %d, select %d", bh, fh, sh)
				}
				h.ExpectClean()
			})
		}
	}
}

// Control size must actually change the height, in order.
func TestControlSizeOrders(t *testing.T) {
	var got []int
	for _, size := range []kit.ControlSize{kit.ControlSmall, kit.ControlMedium, kit.ControlLarge} {
		th := kit.Dark().Derive(func(t *kit.Theme) { t.Controls = size })
		var b kit.Clickable
		h, err := kittest.New(300, 100, 1, func(gtx kit.C) {
			gtx.Constraints.Min = image.Point{}
			kit.Row(gtx, kit.Rigid(func(gtx kit.C) kit.D { return th.Button(gtx, &b, "b", "Save", kit.Secondary) }))
		})
		if err != nil {
			kittest.NoGPU(t, err)
		}
		r, _ := h.Find("b")
		got = append(got, r.Dy())
		h.Release()
	}
	if !(got[0] < got[1] && got[1] < got[2]) {
		t.Fatalf("button heights small/medium/large = %v", got)
	}
}

func abs(v int) int { return max(v, -v) }
