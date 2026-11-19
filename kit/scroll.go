package kit

import (
	"fmt"
	"image"
	"math"
	"strings"
	"time"

	"github.com/hhhapz/giokit/gio/gesture"
	"github.com/hhhapz/giokit/gio/io/pointer"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"
	"github.com/hhhapz/giokit/gio/unit"
	"github.com/hhhapz/giokit/gio/widget"
)

// Wheel smoothing. Gio turns each wheel notch into an instant jump of about
// 150px; List glides there instead, covering a fixed fraction of the
// remaining distance each frame.
const (
	// smoothDelta is the size, in pixels, below which a wheel delta counts
	// as touchpad motion: already smooth, so it applies at once.
	smoothDelta = 8
	// glideTau is the glide's time constant: a frame of length dt covers
	// 1-exp(-dt/glideTau) of the remaining distance. Theme.Motion scales it.
	glideTau = 70 * time.Millisecond
	// glideMaxFrame bounds the frame time a glide step assumes; a longer gap
	// (the first frame, a stall) steps as one glideFrame instead.
	glideMaxFrame = 50 * time.Millisecond
	glideFrame    = 16 * time.Millisecond
	// unboundedScroll is a scroll range limit that never binds.
	unboundedScroll = 1 << 30
)

// Continuity check: while a scroll gesture is in progress, every row must
// move by the distance the input asked for.
const (
	// settleFrames is how many frames after the last scroll input rows must
	// still hold still.
	settleFrames = 4
	// jumpTolerance is the error, in pixels, tolerated between the distance
	// asked for and the distance moved (rounding).
	jumpTolerance = 1
)

// Scrollbar geometry. The track is Theme.HitSize wide; the bar drawn in it
// is thinner and widens while hovered or dragged.
const (
	thumbMinLength unit.Dp = 24 // shortest thumb, however long the content
	thumbGlideJump unit.Dp = 12 // thumb moves larger than this glide
	barWidth       unit.Dp = 4
	barWidthActive unit.Dp = 6
	barEdgeInset   unit.Dp = 3 // between the bar and the list's right edge
	barEndInset    unit.Dp = 2 // trimmed from each end of the thumb
)

// List is the state of a th.List: Gio's widget.List plus smooth wheel
// scrolling, a gliding thumb and the continuity check behind Jumps. The zero
// value is ready to use. Like Gio's own widget state, a List identifies its
// input areas by address: do not copy it after first use.
//
// The embedded widget.List is a supported escape hatch. Reading or setting
// its Position between frames (to save and restore a scroll offset or jump
// to an item) works as in Gio, and th.List picks the change up on the next
// frame. Laying the embedded list out directly bypasses kit: no wheel
// smoothing, scrollbar or continuity check.
type List struct {
	widget.List
	wheel   gesture.Scroll
	pending float32   // wheel distance still to glide, in pixels
	last    time.Time // the previous frame's time, for the glide step
	thumbLo Tween     // the thumb's ends; they glide over jumps in the size
	thumbHi Tween     // estimate, which changes as items of different heights scroll in

	sizes     map[int]int // item heights this frame
	tops      rowTops     // row tops this frame
	prevTops  rowTops     // row tops last frame
	jumps     int
	scrolling int // frames left in the current scroll gesture

	tr listTrace // GIOKIT_TRACE only
}

// rowTops are the top edges, in pixels from the viewport's top, of the
// consecutive rows laid out from index first.
type rowTops struct {
	first int
	y     []int
	valid bool // recorded at least once
}

// at returns the top of row i, if it was laid out.
func (r *rowTops) at(i int) (int, bool) {
	j := i - r.first
	if j < 0 || j >= len(r.y) {
		return 0, false
	}
	return r.y[j], true
}

// listFrame is what one frame's input did to a List, for the continuity
// check and GIOKIT_TRACE.
type listFrame struct {
	n, viewH  int
	requested int     // pixels the wheel moved the list this frame
	dragged   bool    // the scrollbar moved it, by items: not checked
	wheel     int     // wheel distance read this frame
	pending   float32 // glide distance pending before this frame's step
	glide     int     // pixels glided this frame (part of requested)
	pos0      layout.Position
	before    layout.Position // after input, before layout
}

// Jumps counts frames where the content moved a different distance than the
// scroll asked for (a visible skip). Tests assert it stays 0; with
// GIOKIT_TRACE=1 each one is logged to stderr.
func (l *List) Jumps() int { return l.jumps }

// List is a vertical scrolling list with a thin overlay scrollbar (it never
// steals width, so rows stay aligned with headers). id names the scroll
// region; the scrollbar is <id>/scrollbar (thumb <id>/thumb).
//
// Unlike material.List it applies scrollbar drags before laying out the
// list, so the frame that handles a drag also shows it.
func (t *Theme) List(gtx layout.Context, l *List, id string, n int, item layout.ListElement) layout.Dimensions {
	l.Axis = layout.Vertical
	fr := listFrame{n: n, pos0: l.Position}

	// Input first (rule 1), against last frame's layout.
	fr.wheel, fr.requested = l.updateWheel(gtx)
	fr.pending = l.pending
	fr.glide = l.glide(gtx, t)
	fr.requested += fr.glide
	fr.before = l.Position
	fr.dragged = l.updateScrollbar(gtx, gtx.Dp(t.HitSize), n)

	dims := l.layoutItems(gtx, id, n, item)
	fr.viewH = dims.Size.Y
	l.checkContinuity(fr.requested, fr.dragged)
	if traceEnabled {
		l.trace(id, fr)
	}
	if fr.glide != 0 && l.Position.First == fr.before.First && l.Position.Offset == fr.before.Offset-fr.glide {
		l.pending = 0 // clamped at an end: stop gliding
	}
	l.drawScrollbar(gtx, t, id, n, dims.Size)
	return dims
}

// updateWheel reads this frame's wheel input and returns its distance. A
// small delta moves the list at once and is also returned as moved; a larger
// one joins the pending glide.
//
// The wheel handler is the list's ancestor, so it only sees scrolls no item
// claimed (a focused slider keeps its wheel): input bubbles up. Its range
// stops at the list's ends, so a list that can't move further passes the
// wheel on to its parent.
func (l *List) updateWheel(gtx layout.Context) (dist, moved int) {
	up, down := unboundedScroll, unboundedScroll
	if l.Position.First == 0 && l.Position.Offset <= 0 {
		up = 0
	}
	if !l.Position.BeforeEnd && l.Position.Length > 0 {
		down = 0
	}
	dist = l.wheel.Update(gtx.Metric, gtx.Source, gtx.Now, gesture.Vertical,
		pointer.ScrollRange{}, pointer.ScrollRange{Min: -up, Max: down})
	d := float32(dist)
	if d == 0 {
		return 0, 0
	}
	if d < 0 {
		l.Position.BeforeEnd = true // leave a ScrollToEnd tail, as Gio's own scroll does
	}
	if d > -smoothDelta && d < smoothDelta && l.pending == 0 {
		l.Position.Offset += dist
		return dist, dist
	}
	l.pending += d
	return dist, 0
}

// glide moves the list by this frame's share of the pending wheel distance
// and returns that distance in pixels. The tail moves 1px per frame rather
// than snapping; with Motion 0 the whole distance applies at once.
func (l *List) glide(gtx layout.Context, th *Theme) int {
	dt := gtx.Now.Sub(l.last)
	firstFrame := l.last.IsZero()
	l.last = gtx.Now
	if l.pending == 0 {
		return 0
	}
	if firstFrame || dt <= 0 || dt > glideMaxFrame {
		dt = glideFrame
	}
	// The extra nanosecond keeps the divisor nonzero when Motion scales the
	// time constant to 0; that case takes the whole distance below anyway.
	step := l.pending * float32(1-math.Exp(-float64(dt)/float64(th.Dur(glideTau)+1)))
	if th.Motion == 0 {
		step = l.pending
	} else if step < 1 && step > -1 {
		step = max(-1, min(1, l.pending))
	}
	glide := int(step)
	if glide == 0 {
		// Less than a pixel is left: drop the sub-pixel remainder.
		l.pending = 0
	}
	l.pending -= float32(glide)
	l.Position.Offset += glide
	if glide < 0 {
		l.Position.BeforeEnd = true
	}
	gtx.Execute(op.InvalidateCmd{})
	return glide
}

// updateScrollbar applies scrollbar drags and track clicks against last
// frame's viewport, before the list is laid out, and reports whether they
// moved the list. hit is the track width in pixels.
func (l *List) updateScrollbar(gtx layout.Context, hit, n int) bool {
	gtx.Constraints.Max.X = hit
	start, end := viewport(l.Position, n, gtx.Constraints.Max.Y)
	l.Scrollbar.Update(gtx, layout.Vertical, start, end)
	d := l.Scrollbar.ScrollDistance()
	if d == 0 {
		return false
	}
	l.List.ScrollBy(d * float32(n))
	return true
}

// layoutItems lays out the visible items and records their heights in
// l.sizes. layout.List runs with input disabled, so its own scroll handler
// never registers and can't grab the wheel (it would jump a whole notch at
// once); items get the real input source back.
func (l *List) layoutItems(gtx layout.Context, id string, n int, item layout.ListElement) layout.Dimensions {
	src := gtx.Source
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	defer area.Pop()
	l.wheel.Add(gtx.Ops)
	Tag(gtx, id)
	if l.sizes == nil {
		l.sizes = map[int]int{}
	}
	clear(l.sizes)
	element := func(gtx layout.Context, i int) layout.Dimensions {
		gtx.Source = src
		d := item(gtx, i)
		l.sizes[i] = d.Size.Y
		return d
	}
	if traceEnabled {
		element = l.tr.timed(id, l.sizes, element)
	}
	return l.List.Layout(gtx.Disabled(), n, element)
}

// drawScrollbar draws the thumb and registers the scrollbar's input areas,
// unless all of the content is visible. The track spans the full height
// along the right edge, Theme.HitSize wide (a usable target); it draws
// nothing but the thumb.
func (l *List) drawScrollbar(gtx layout.Context, th *Theme, id string, n int, size image.Point) {
	start, end := viewport(l.Position, n, size.Y)
	if n == 0 || (start <= 0 && end >= 1) {
		return
	}
	h := size.Y
	track := image.Rect(size.X-gtx.Dp(th.HitSize), 0, size.X, h)
	minLen := gtx.Dp(thumbMinLength)
	lo, hi := start*float32(h), end*float32(h)
	d := time.Duration(0) // track scrolling exactly...
	if jump := float32(gtx.Dp(thumbGlideJump)); !l.Scrollbar.Dragging() &&
		(abs32(lo-l.thumbLo.Value(gtx)) > jump || abs32(hi-l.thumbHi.Value(gtx)) > jump) {
		d = th.Dur(DurFast) // ...but glide over estimate jumps
		if traceEnabled {
			tracef("list_thumb_jump", "id", id, "lo", lo, "hi", hi, "was_lo", l.thumbLo.Value(gtx), "was_hi", l.thumbHi.Value(gtx), "track", h)
		}
	}
	y0, y1 := int(l.thumbLo.To(gtx, lo, d)), int(l.thumbHi.To(gtx, hi, d))
	if y1-y0 < minLen {
		mid := (y0 + y1) / 2
		y0, y1 = max(0, mid-minLen/2), min(h, mid+minLen/2)
	}
	thumb := image.Rect(track.Min.X, y0, track.Max.X, y1)

	active := l.Scrollbar.IndicatorHovered() || l.Scrollbar.Dragging()
	w, c := gtx.Dp(barWidth), th.Border
	if active {
		w, c = gtx.Dp(barWidthActive), th.Dim
	} else if l.Scrollbar.TrackHovered() {
		c = LerpColor(th.Border, th.Dim, 0.5)
	}
	edge, end2 := gtx.Dp(barEdgeInset), gtx.Dp(barEndInset)
	bar := image.Rect(size.X-edge-w, y0+end2, size.X-edge, y1-end2)
	paint.FillShape(gtx.Ops, c, clip.UniformRRect(bar, w/2).Op(gtx.Ops))

	// Same layering as material: drag on the track, a pass-through click
	// layer on top (track clicks), and the thumb (hover).
	ta := clip.Rect(track).Push(gtx.Ops)
	l.Scrollbar.AddDrag(gtx.Ops)
	Tag(gtx, Part(id, "scrollbar"))
	pass := pointer.PassOp{}.Push(gtx.Ops)
	tc := clip.Rect(track).Push(gtx.Ops)
	l.Scrollbar.AddTrack(gtx.Ops)
	tc.Pop()
	tha := clip.Rect(thumb).Push(gtx.Ops)
	l.Scrollbar.AddIndicator(gtx.Ops)
	Tag(gtx, Part(id, "thumb"))
	tha.Pop()
	pass.Pop()
	ta.Pop()
}

// viewport estimates the visible fraction [start, end] of a list's content
// from its position (as material.List does).
func viewport(lp layout.Position, n, size int) (start, end float32) {
	if n == 0 || lp.Length <= 0 {
		return 0, 1
	}
	length := float32(lp.Length)
	elem := length / float32(n)
	clamp := func(v float32) float32 { return max(0, min(1, v)) }
	start = clamp((float32(lp.First)*elem + float32(lp.Offset)) / length)
	end = clamp((float32(lp.First+lp.Count)*elem + float32(lp.OffsetLast)) / length)
	if vis := float32(size) / length; vis < 1 && end-start != vis {
		// spread the estimate error toward the far end
		err := vis - (end - start)
		start, end = clamp(start-err*start), clamp(end+err*(1-end))
	}
	return start, end
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// checkContinuity compares where each row visible in both frames was and is.
// A clean scroll moves every row by the same amount, the amount asked for
// (less if clamped at an end). Rows moving by different amounts, or all of
// them by the wrong amount, is a visible skip. requested is the wheel
// distance this frame; dragged reports a scrollbar move, which moves by
// items and so is not checked.
func (l *List) checkContinuity(requested int, dragged bool) {
	// Reuse last frame's buffer for this frame's tops: no per-frame garbage.
	l.tops, l.prevTops = l.prevTops, l.tops
	tops, prev := &l.tops, &l.prevTops
	tops.first, tops.y, tops.valid = l.Position.First, tops.y[:0], true
	pos := -l.Position.Offset
	for i := l.Position.First; ; i++ {
		h, ok := l.sizes[i]
		if !ok {
			break
		}
		tops.y = append(tops.y, pos)
		pos += h
	}
	l.tr.moved, l.tr.common, l.tr.uneven, l.tr.skipped = 0, false, "", false
	// Check while a scroll gesture is in progress: frames with input, and a
	// few after (rows must hold still then: content changing size mid-scroll
	// is exactly the skip). Outside gestures, content may legitimately move.
	if requested != 0 || dragged {
		l.scrolling = settleFrames
	} else if l.scrolling > 0 {
		l.scrolling--
	}
	if l.scrolling == 0 || dragged || !prev.valid {
		return
	}
	moved, common, uneven := 0, false, false
	for j, yi := range tops.y {
		p, ok := prev.at(tops.first + j)
		if !ok {
			continue
		}
		d := p - yi // positive: content moved up (scrolled down)
		if !common {
			moved, common = d, true
		} else if d != moved {
			uneven = true
		}
	}
	if !common {
		return // no row in common (a jump of more than a screen)
	}
	l.tr.moved, l.tr.common = moved, true
	if uneven && traceEnabled {
		l.tr.uneven = rowDeltas(tops, prev)
	}
	atTop := l.Position.First == 0 && l.Position.Offset == 0
	atEnd := !l.Position.BeforeEnd
	wrong := moved != requested && !(atTop && moved > requested) && !(atEnd && moved < requested)
	if uneven || (wrong && (moved-requested > jumpTolerance || moved-requested < -jumpTolerance)) {
		l.jumps++
		l.tr.skipped = true
	}
}

// rowDeltas lists how far each row in both frames moved, as "row:delta,…".
func rowDeltas(tops, prev *rowTops) string {
	var b strings.Builder
	for j, yi := range tops.y {
		i := tops.first + j
		if p, ok := prev.at(i); ok {
			if b.Len() > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "%d:%+d", i, p-yi)
		}
	}
	return b.String()
}

// ScrollIntoView scrolls the least distance that shows item i whole, using
// last frame's layout (rows of similar height). Call it from a handler; the
// next frame shows the result. A negative i scrolls to the top; an i past
// the end scrolls to the end when the list is next laid out.
func (l *List) ScrollIntoView(i int) {
	i = max(i, 0)
	p := &l.Position
	l.pending = 0
	if p.Count == 0 {
		p.First, p.Offset = i, 0
		return
	}
	last := p.First + p.Count - 1
	switch {
	case i < p.First || (i == p.First && p.Offset > 0):
		p.First, p.Offset = i, 0
	case i > last || (i == last && p.OffsetLast < 0):
		p.First += i - last
		p.Offset -= p.OffsetLast // OffsetLast < 0: the hidden part of the last row
	default:
		return
	}
	p.BeforeEnd = true
}
