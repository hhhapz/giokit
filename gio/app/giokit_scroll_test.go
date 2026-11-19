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
	previous := GiokitWheelMiniStepScale()
	t.Cleanup(func() { _ = SetGiokitWheelMiniStepScale(previous) })
	if err := SetGiokitWheelMiniStepScale(defaultWheelMiniStepScale); err != nil {
		t.Fatal(err)
	}
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

func TestWheelMiniStepScaleConfiguration(t *testing.T) {
	previous := GiokitWheelMiniStepScale()
	t.Cleanup(func() { _ = SetGiokitWheelMiniStepScale(previous) })
	if err := SetGiokitWheelMiniStepScale(1.5); err != nil {
		t.Fatal(err)
	}
	if got := GiokitWheelMiniStepScale(); got != 1.5 {
		t.Fatalf("configured scale: got %v, want 1.5", got)
	}
	if got := wheelScale(f32.Pt(0, 2), image.Point{}, true).Y; got != 3 {
		t.Errorf("wheel sub-step: got %v, want 3", got)
	}
	if got := wheelScale(f32.Pt(0, 2), image.Point{}, false).Y; got != 2 {
		t.Errorf("touchpad: got %v, want 2", got)
	}
	if got := wheelScale(f32.Pt(0, 15), image.Pt(0, 1), true).Y; got != 150 {
		t.Errorf("classic wheel: got %v, want 150", got)
	}
	for _, invalid := range []float32{0, -1, float32(math.NaN()), float32(math.Inf(1))} {
		if err := SetGiokitWheelMiniStepScale(invalid); err == nil {
			t.Errorf("accepted invalid scale %v", invalid)
		}
		if got := GiokitWheelMiniStepScale(); got != 1.5 {
			t.Errorf("invalid scale changed setting to %v", got)
		}
	}
	for _, value := range []string{"0.8", "2"} {
		if _, err := parseWheelMiniStepScale(value); err != nil {
			t.Errorf("rejected environment value %q: %v", value, err)
		}
	}
	for _, value := range []string{"", "0", "NaN", "-1", "nonsense"} {
		if _, err := parseWheelMiniStepScale(value); err == nil {
			t.Errorf("accepted invalid environment value %q", value)
		}
	}
}
