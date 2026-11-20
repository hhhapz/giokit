package kit_test

import (
	"fmt"
	"image"
	"testing"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/fonts"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// Buttons, text fields and selects sit side by side in toolbars and forms:
// at every control size and with every font set they must be one height.
func TestControlsLineUp(t *testing.T) {
	for _, name := range fonts.Names() {
		set, _ := fonts.ByName(name)
		for _, size := range []kit.ControlSize{kit.ControlSmall, kit.ControlMedium, kit.ControlLarge} {
			t.Run(fmt.Sprintf("%s/%d", set.Name, size), func(t *testing.T) {
				th := kit.Dark().Derive(func(t *kit.Theme) { t.Controls = size })
				th.UseFonts(set)
				var b kit.Clickable
				var e widget.Editor
				var s ui.SelectState
				h := kittest.NewT(t, 600, 120, func(gtx layout.Context) {
					b.Clicked(gtx)
					s.Update(gtx)
					gtx.Constraints.Min = image.Point{} // natural heights, not the window's
					kit.Row(gtx,
						kit.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.Button{ID: "b", Text: "Save"}.Layout(gtx, th, &b)
						}),
						kit.Rigid(kit.Fixed(160, func(gtx layout.Context) layout.Dimensions {
							return ui.TextField{ID: "f", Placeholder: "Name"}.Layout(gtx, th, &e)
						})),
						kit.Rigid(kit.Fixed(160, func(gtx layout.Context) layout.Dimensions {
							return ui.Select{ID: "s", Options: []ui.Choice{{ID: "one", Label: "One"}}, Selected: "one"}.Layout(gtx, th, &s)
						})),
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
		h, err := kittest.New(300, 100, 1, func(gtx layout.Context) {
			gtx.Constraints.Min = image.Point{}
			kit.Row(gtx, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Button{ID: "b", Text: "Save"}.Layout(gtx, th, &b)
			}))
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

// A per-instance Size lines controls up like the theme's Controls does: a
// small button beside a small field, select and date field share a height.
func TestInstanceSizesLineUp(t *testing.T) {
	th := kit.Dark()
	for _, size := range []kit.ControlSize{kit.ControlSmall, kit.ControlMedium, kit.ControlLarge} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			var b kit.Clickable
			var e widget.Editor
			var s ui.SelectState
			var d ui.DateFieldState
			h := kittest.NewT(t, 800, 120, func(gtx layout.Context) {
				b.Clicked(gtx)
				s.Update(gtx)
				d.Update(gtx)
				gtx.Constraints.Min = image.Point{}
				kit.Row(gtx,
					kit.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.Button{ID: "b", Text: "Save", Size: size}.Layout(gtx, th, &b)
					}),
					kit.Rigid(kit.Fixed(160, func(gtx layout.Context) layout.Dimensions {
						return ui.TextField{ID: "f", Placeholder: "Name", Size: size}.Layout(gtx, th, &e)
					})),
					kit.Rigid(kit.Fixed(160, func(gtx layout.Context) layout.Dimensions {
						return ui.Select{ID: "s", Options: []ui.Choice{{ID: "one", Label: "One"}}, Size: size}.Layout(gtx, th, &s)
					})),
					kit.Rigid(kit.Fixed(200, func(gtx layout.Context) layout.Dimensions {
						return ui.DateField{ID: "d", Placeholder: "Date", Size: size}.Layout(gtx, th, &d)
					})),
				)
			})
			h.Theme = th
			bh := h.ExpectID("b").Dy()
			for _, id := range []string{"f", "s/field", "d/field"} {
				if got := h.ExpectID(id).Dy(); abs(got-bh) > 1 {
					t.Errorf("%s is %dpx tall, the button %dpx", id, got, bh)
				}
			}
		})
	}
}
