// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"fmt"
	"image"
	"math"
	"os"
	"strconv"
	"sync/atomic"

	"gioui.org/f32"
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
			return d * GiokitWheelMiniStepScale()
		}
		return d
	}
	// A whole click is ~10-15 units; a high-res sub-step is ~1.9. Anything
	// under 5 per step is a sub-step.
	perStep := abs(d) / float32(max(steps, -steps))
	if perStep < 5 && !GiokitUpstreamWheel.Load() {
		if wheel {
			return d * GiokitWheelMiniStepScale()
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

var wheelMiniStepScale = func() *atomic.Uint32 {
	scale := float32(defaultWheelMiniStepScale)
	if value, ok := os.LookupEnv("GIOKIT_WHEEL_MINI_STEP_SCALE"); ok {
		if parsed, err := parseWheelMiniStepScale(value); err == nil {
			scale = parsed
		}
	}
	v := new(atomic.Uint32)
	v.Store(math.Float32bits(scale))
	return v
}()

// GiokitWheelMiniStepScale returns the multiplier for high-resolution Wayland
// wheel sub-steps. The setting is process-wide and applies to every window.
// It defaults to 1.2; the GIOKIT_WHEEL_MINI_STEP_SCALE environment variable,
// read once when the package is initialized, overrides the default (an
// invalid value is ignored), and SetGiokitWheelMiniStepScale overrides both.
func GiokitWheelMiniStepScale() float32 {
	return math.Float32frombits(wheelMiniStepScale.Load())
}

// SetGiokitWheelMiniStepScale changes the multiplier for every window. It is
// safe to call at any time from any goroutine and takes effect from the next
// scroll frame. It affects Wayland wheel input only; classic wheel clicks and
// touchpads are unchanged. A scale that is not finite and positive is
// rejected with an error and leaves the setting as it was.
func SetGiokitWheelMiniStepScale(scale float32) error {
	if err := validateWheelMiniStepScale(scale); err != nil {
		return err
	}
	wheelMiniStepScale.Store(math.Float32bits(scale))
	return nil
}

func parseWheelMiniStepScale(value string) (float32, error) {
	v, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid wheel mini-step scale %q: %w", value, err)
	}
	scale := float32(v)
	return scale, validateWheelMiniStepScale(scale)
}

func validateWheelMiniStepScale(scale float32) error {
	if scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		return fmt.Errorf("wheel mini-step scale must be finite and greater than zero")
	}
	return nil
}
