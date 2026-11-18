package ui_test

import (
	"image"
	"slices"
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// A chord whose key is "+" draws a "+" cap, not empty caps.
func TestKbdPlusKey(t *testing.T) {
	th := kit.Dark()
	for chord, want := range map[string][]string{
		"Ctrl++":       {"Ctrl", "+"},
		"+":            {"+"},
		"Ctrl+Shift+K": {"Ctrl", "Shift", "K"},
	} {
		h := kittest.NewT(t, 300, 100, func(gtx layout.Context) {
			kit.TagArea(gtx, "kbd", func(gtx layout.Context) layout.Dimensions { return ui.Kbd{Chord: chord}.Layout(gtx, th) })
		})
		if got := h.TextsIn("kbd"); !slices.Equal(got, want) {
			t.Errorf("Kbd %q: caps %q, want %q", chord, got, want)
		}
	}
}

// The divider's highlight scales with the display: it was 3px wide at any
// scale.
func TestSplitHighlightScales(t *testing.T) {
	th := kit.Dark()
	var s ui.SplitState
	const scale = 3
	h := kittest.NewTAt(t, 300, 100, scale, func(gtx layout.Context) {
		ui.Split{ID: "split", A: kit.Grow, B: kit.Grow}.Layout(gtx, th, &s)
	})
	h.Tab(false)
	if got := h.FocusedID(); got != "split" {
		t.Fatalf("focused %q, want the divider", got)
	}
	h.Wait(time.Second)
	r := h.ExpectID("split")
	y := r.Min.Y + r.Dy()/2
	n := 0
	for x := r.Min.X; x < r.Max.X; x++ {
		if h.Pixel(image.Pt(x, y)) == th.Focus {
			n++
		}
	}
	line := max(1, int(float32(th.Stroke)*scale+0.5))
	if want := 3 * line; n != want {
		t.Errorf("highlight %dpx wide at scale %d, want %d", n, scale, want)
	}
}

// Keys a custom focusable leaves unclaimed resolve against the KeyContext
// around it: Enter on a focused slider or split divider runs the
// enclosing context's action.
func TestCustomFocusablesReachKeyContext(t *testing.T) {
	th := kit.Dark()
	var slider ui.SliderState
	var split ui.SplitState
	var confirmed string
	h := kittest.NewT(t, 300, 200, func(gtx layout.Context) {
		slider.Update(gtx)
		kit.Col(gtx,
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.KeyContext(gtx, "slider", func(gtx layout.Context) layout.Dimensions {
					kit.OnAction(gtx, kit.ActConfirm, func() { confirmed = "slider" })
					return ui.Slider{ID: "vol", Label: "Volume", Value: 0.5}.Layout(gtx, th, &slider)
				})
			}),
			kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return kit.KeyContext(gtx, "split", func(gtx layout.Context) layout.Dimensions {
					kit.OnAction(gtx, kit.ActConfirm, func() { confirmed = "split" })
					return ui.Split{ID: "split", A: kit.Grow, B: kit.Grow}.Layout(gtx, th, &split)
				})
			}),
		)
	})
	for _, id := range []string{"slider", "split"} {
		h.Tab(false)
		h.Key(key.NameReturn, 0)
		if confirmed != id {
			t.Errorf("Enter with %s focused (%q) ran %q", id, h.FocusedID(), confirmed)
		}
	}
}
