// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"image"
	"math"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/hhhapz/giokit/gio/f32"
)

// wheelScale converts one Wayland pointer frame's axis distance to Gio scroll
// pixels. A classic wheel reports one axis event of ~10-15 units per click,
// where ~100px suits, so a frame carrying a discrete step is scaled by 10.
//
// giokit patch: a high-resolution wheel sends an axis event every 1/8 click
// (~1.9 units) and the discrete step only with the last one. Upstream scaled
// that last event too, so it scrolled 10× its neighbours: a ~17px skip once
// per click. Scale only frames that carry about a whole click per step;
// high-res sub-steps stay even. A wheel source gives those small steps a
// modest speed boost without changing touchpad motion.
func wheelScale(dist f32.Point, steps image.Point, wheel bool) f32.Point {
	const discreteScale = 10
	dist.X = scaleAxis(dist.X, steps.X, discreteScale, wheel)
	dist.Y = scaleAxis(dist.Y, steps.Y, discreteScale, wheel)
	return dist
}

func scaleAxis(d float32, steps int, scale float32, wheel bool) float32 {
	if steps == 0 {
		if wheel && !GiokitUpstreamWheel.Load() {
			return d * wheelMiniStepScale
		}
		return d
	}
	// A whole click is ~10-15 units; a high-res sub-step is ~1.9. Anything
	// under 5 per step is a sub-step.
	perStep := abs(d) / float32(max(steps, -steps))
	if perStep < 5 && !GiokitUpstreamWheel.Load() {
		if wheel {
			return d * wheelMiniStepScale
		}
		return d
	}
	return d * scale
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// GiokitUpstreamWheel restores upstream's wheel scaling (scale every frame
// with a discrete step) when set: for A/B testing the patch in one build
// (examples/scrolltest, key U). It is process-wide, applies to every
// window, and may be changed at any time from any goroutine; each Wayland
// scroll frame reads it when the frame is flushed.
var GiokitUpstreamWheel atomic.Bool

const defaultWheelMiniStepScale = 1.2

// wheelMiniStepScale multiplies high-resolution Wayland wheel sub-steps:
// 1.2, or GIOKIT_WHEEL_MINI_STEP_SCALE when that is a finite number above
// zero. It is read once, when the package is initialized.
var wheelMiniStepScale = func() float32 {
	if s, ok := parseWheelMiniStepScale(os.Getenv("GIOKIT_WHEEL_MINI_STEP_SCALE")); ok {
		return s
	}
	return defaultWheelMiniStepScale
}()

// parseWheelMiniStepScale parses a scale, accepting finite values above zero.
func parseWheelMiniStepScale(value string) (float32, bool) {
	v, err := strconv.ParseFloat(value, 32)
	if err != nil || v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, false
	}
	return float32(v), true
}
