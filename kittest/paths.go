package kittest

import "fmt"

// Path builds: the GPU caches a filled shape's vertex buffer by the
// operation that drew it, and the frame's ops are reset every frame, so a
// shape drawn straight into gtx.Ops is flattened and uploaded again on
// every frame it appears. Record such drawing in a kit.DrawCache instead
// (see pitfall "Custom shapes rebuilt on the GPU every frame"). These count
// the rebuilds, to catch drawing that should be cached: a scroll, drag or
// animation frame of a well-cached UI builds next to nothing.

// PathBuilds is how many filled shapes the harness's GPU has flattened and
// uploaded so far, over every frame drawn.
func (h *Harness) PathBuilds() int { return int(h.win.Stats().PathBuilds) }

// PathBuildsDuring runs act and returns the shapes built and frames drawn
// while it ran (including Settle's and strict mode's frames).
func (h *Harness) PathBuildsDuring(act func()) (builds, frames int) {
	b0, f0 := h.PathBuilds(), h.Frames
	act()
	return h.PathBuilds() - b0, h.Frames - f0
}

// ExpectPathBuilds runs act and fails the test if the frames it drew built
// more than max shapes each, on average. Use it on frames that move things
// rather than change them (scrolling, dragging, a sliding or fading
// panel, hover): those should replay recordings, so pass a small max (a
// few shapes for what really changes: a moving scrollbar thumb, the
// hovered control). Warm up first (one frame of the same kind), so the
// first recording of each cache isn't counted.
func (h *Harness) ExpectPathBuilds(max float64, act func()) {
	h.tb.Helper()
	h.requireTest("ExpectPathBuilds")
	builds, frames := h.PathBuildsDuring(act)
	if frames == 0 {
		h.tb.Errorf("ExpectPathBuilds: no frames drawn")
		return
	}
	if per := float64(builds) / float64(frames); per > max {
		h.tb.Error(fmt.Sprintf("%d shapes built over %d frames (%.1f per frame, want at most %g): "+
			"drawing straight into gtx.Ops is rebuilt on the GPU every frame; record it in a kit.DrawCache",
			builds, frames, per, max))
	}
}
