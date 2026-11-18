package kit

import (
	"slices"
	"time"
)

const (
	// lateFrameFactor flags a frame interval this many times the median as
	// LATE_FRAME.
	lateFrameFactor = 1.5
	// lateFrameMinSamples is how many intervals the median needs before
	// late frames are reported.
	lateFrameMinSamples = 9
	// idleGap separates idle periods from stutter: a longer interval means
	// nothing asked for frames, and is left out of the statistics.
	idleGap = time.Second
	// pacingWindow is how often a pacing summary line is written.
	pacingWindow = time.Second
)

// framePacing collects frame timings for GIOKIT_TRACE's LATE_FRAME and
// pacing lines.
type framePacing struct {
	last   time.Time       // previous frame's timestamp
	window time.Time       // start of the current summary window
	gaps   []time.Duration // intervals between frames in this window
	builds []time.Duration // frame build times in this window
}

// frame records a frame presented at now that took build to build.
func (p *framePacing) frame(now time.Time, build time.Duration) {
	if !p.last.IsZero() {
		gap := now.Sub(p.last)
		if len(p.gaps) >= lateFrameMinSamples {
			med := median(p.gaps)
			if float64(gap) > float64(med)*lateFrameFactor && gap < idleGap {
				tracef("LATE_FRAME", "gap_ms", ms(gap), "median_ms", ms(med), "build_ms", ms(build))
			}
		}
		if gap < idleGap {
			p.gaps = append(p.gaps, gap)
		}
	}
	p.builds = append(p.builds, build)
	p.last = now
	if p.window.IsZero() {
		p.window = now
	}
	if now.Sub(p.window) >= pacingWindow && len(p.gaps) > 0 {
		tracef("pacing", "frames", len(p.builds),
			"gap_med_ms", ms(median(p.gaps)), "gap_max_ms", ms(slices.Max(p.gaps)),
			"build_med_ms", ms(median(p.builds)), "build_max_ms", ms(slices.Max(p.builds)))
		p.window, p.gaps, p.builds = now, p.gaps[:0], p.builds[:0]
	}
}

// median returns the upper median of a non-empty d, leaving d unchanged.
func median(d []time.Duration) time.Duration {
	s := slices.Sorted(slices.Values(d))
	return s[len(s)/2]
}
