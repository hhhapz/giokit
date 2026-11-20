// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"image"
	"math"
	"testing"

	"github.com/hhhapz/giokit/gio/f32"
)

// A high-resolution wheel click: 8 axis frames of 1.875, discrete on the last.
// Every frame scrolls 20% more than raw, including the last one (upstream
// scaled only the last one ×10).
func TestWheelScaleHighResEven(t *testing.T) {
	previous := wheelMiniStepScale
	t.Cleanup(func() { wheelMiniStepScale = previous })
	wheelMiniStepScale = defaultWheelMiniStepScale
	for i := range 8 {
		steps := image.Point{}
		if i == 7 {
			steps.Y = 1
		}
		for _, sign := range []float32{1, -1} {
			if v := wheelScale(f32.Pt(0, sign*1.875), steps.Mul(int(sign)), true).Y; v != sign*2.25 {
				t.Fatalf("sub-step %d (sign %v): %v, want %v", i, sign, v, sign*2.25)
			}
		}
	}
	// Classic wheel: one event of 15 per click, scaled to a ~150px step.
	if v := wheelScale(f32.Pt(0, -15), image.Pt(0, -1), true).Y; v != -150 {
		t.Errorf("classic click: %v, want -150", v)
	}
	// Touchpad: no discrete steps, untouched.
	if v := wheelScale(f32.Pt(0, 3), image.Point{}, false).Y; v != 3 {
		t.Errorf("touchpad: %v, want 3", v)
	}
	if v := wheelScale(f32.Pt(0, 3), image.Point{}, true).Y; math.Abs(float64(v)-3.6) > 1e-5 {
		t.Errorf("wheel mini-step without discrete marker: %v, want 3.6", v)
	}
	// Upstream mode reproduces the bug, for the A/B bench.
	GiokitUpstreamWheel.Store(true)
	defer GiokitUpstreamWheel.Store(false)
	if v := wheelScale(f32.Pt(0, 1.875), image.Pt(0, 1), true).Y; v != 18.75 {
		t.Errorf("upstream mode: %v, want 18.75", v)
	}
	if v := wheelScale(f32.Pt(0, 1.875), image.Point{}, true).Y; v != 1.875 {
		t.Errorf("upstream unmarked sub-step: %v, want 1.875", v)
	}
}

func TestWheelMiniStepScaleParse(t *testing.T) {
	for _, value := range []string{"0.8", "2"} {
		if _, ok := parseWheelMiniStepScale(value); !ok {
			t.Errorf("rejected environment value %q", value)
		}
	}
	for _, value := range []string{"", "0", "NaN", "Inf", "-1", "nonsense"} {
		if _, ok := parseWheelMiniStepScale(value); ok {
			t.Errorf("accepted invalid environment value %q", value)
		}
	}
}
