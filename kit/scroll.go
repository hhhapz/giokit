package kit

import (
	"fmt"
	"image"
	"math"
	"strings"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
)

// List is the state of a th.List: Gio's widget.List plus smooth wheel
// scrolling. (Gio turns each wheel notch into an instant ~150px jump; List
// glides there instead. Small touchpad deltas apply immediately.)
type List struct {
	widget.List
	wheel   gesture.Scroll
	pending float32
	last    time.Time
	thumbLo Tween // the thumb glides too: size estimates change as items of
	thumbHi Tween // different heights scroll in, which made it jump

	// continuity check: did the content move exactly as far as we asked?
	sizes     map[int]int // item heights this frame
	prevY     map[int]int // row tops last frame
	jumps     int
	scrolling int // frames left in the current scroll gesture

	tr listTrace // GIOKIT_TRACE only
}

// listTrace is the per-frame state GIOKIT_TRACE logs for a List.
type listTrace struct {
	known   map[int]int           // last measured height of each item, ever
	cost    map[int]time.Duration // item layout time this frame
	visible map[int]bool          // items on screen last frame
	moved   int                   // continuity result this frame
	common  bool
	uneven  string // per-row deltas when they disagree
	skipped bool
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
func (t *Theme) List(gtx C, l *List, id string, n int, item layout.ListElement) D {
	l.Axis = layout.Vertical
	hit := gtx.Dp(16)

	// Wheel input first (rule 1). Our wheel handler is the list's ancestor,
	// so it only sees scrolls no item claimed (a focused slider keeps its
	// wheel): input bubbles up. Its range stops at the list's ends, so a
	// list that can't move further passes the wheel on to its parent.
	requested := 0
	pos0 := l.Position
	up, down := 1<<30, 1<<30
	if l.Position.First == 0 && l.Position.Offset <= 0 {
		up = 0
	}
	if !l.Position.BeforeEnd && l.Position.Length > 0 {
		down = 0
	}
	dist := l.wheel.Update(gtx.Metric, gtx.Source, gtx.Now, gesture.Vertical,
		pointer.ScrollRange{}, pointer.ScrollRange{Min: -up, Max: down})
	if d := float32(dist); d != 0 {
		if d < 0 {
			l.Position.BeforeEnd = true // leave a ScrollToEnd tail, as Gio's own scroll does
		}
		if d > -8 && d < 8 && l.pending == 0 {
			l.Position.Offset += dist // tiny deltas are already smooth
			requested += dist
		} else {
			l.pending += d
		}
	}
	wheel, pend0 := dist, l.pending
	glide := 0
	if l.pending != 0 {
		dt := gtx.Now.Sub(l.last)
		if l.last.IsZero() || dt <= 0 || dt > 50*time.Millisecond {
			dt = 16 * time.Millisecond
		}
		step := l.pending * float32(1-math.Exp(-float64(dt)/float64(t.Dur(70*time.Millisecond)+1)))
		if t.Motion == 0 {
			step = l.pending
		} else if step < 1 && step > -1 { // tail: 1px per frame, no snap
			step = max(-1, min(1, l.pending))
		}
		glide = int(step)
		if glide == 0 {
			glide = int(l.pending)
			l.pending = 0
		}
		l.pending -= float32(glide)
		l.Position.Offset += glide
		requested += glide
		if glide < 0 {
			l.Position.BeforeEnd = true
		}
		gtx.Execute(op.InvalidateCmd{})
	}
	l.last = gtx.Now
	before := l.Position

	// Handle scrollbar input first, against last frame's viewport.
	{
		gtx := gtx
		gtx.Constraints.Max.X = hit
		start, end := viewport(l.Position, n, gtx.Constraints.Max.Y)
		l.Scrollbar.Update(gtx, layout.Vertical, start, end)
		if d := l.Scrollbar.ScrollDistance(); d != 0 {
			l.List.ScrollBy(d * float32(n))
			requested = -1 << 30 // scrollbar drags move by items: skip the check
		}
	}

	// layout.List runs with input disabled, so its own scroll handler never
	// registers and can't grab the wheel (it would jump a whole notch at
	// once); items get the real input source back.
	src := gtx.Source
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	l.wheel.Add(gtx.Ops)
	Tag(gtx, id)
	if l.sizes == nil {
		l.sizes = map[int]int{}
	}
	clear(l.sizes)
	if traceEnabled {
		if l.tr.cost == nil {
			l.tr.cost = map[int]time.Duration{}
		}
		clear(l.tr.cost)
	}
	dims := l.List.Layout(gtx.Disabled(), n, func(gtx C, i int) D {
		gtx.Source = src
		var t0 time.Time
		if traceEnabled {
			t0 = time.Now()
		}
		d := item(gtx, i)
		if traceEnabled {
			l.tr.cost[i] += time.Since(t0)
			if _, again := l.sizes[i]; again {
				tracef("list_relayout", "id", id, "i", i, "h", d.Size.Y, "prev_h", l.sizes[i])
			}
		}
		l.sizes[i] = d.Size.Y
		return d
	})
	area.Pop()
	l.checkContinuity(id, requested)
	if traceEnabled {
		l.trace(id, n, dims.Size.Y, requested, wheel, pend0, glide, pos0, before)
	}
	if glide != 0 && l.Position.First == before.First && l.Position.Offset == before.Offset-glide {
		l.pending = 0 // clamped at an end: stop gliding
	}
	start, end := viewport(l.Position, n, dims.Size.Y)
	if n == 0 || (start <= 0 && end >= 1) {
		return dims
	}

	// Track: the full height along the right edge, 16dp wide (a usable target);
	// it draws nothing but the thumb.
	h := dims.Size.Y
	track := image.Rect(dims.Size.X-hit, 0, dims.Size.X, h)
	minLen := gtx.Dp(24)
	lo, hi := start*float32(h), end*float32(h)
	d := time.Duration(0) // track scrolling exactly...
	if jump := float32(gtx.Dp(12)); !l.Scrollbar.Dragging() &&
		(abs32(lo-l.thumbLo.Value(gtx)) > jump || abs32(hi-l.thumbHi.Value(gtx)) > jump) {
		d = t.Dur(DurFast) // ...but glide over estimate jumps
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
	w, c := gtx.Dp(4), t.Border
	if active {
		w, c = gtx.Dp(6), t.Dim
	} else if l.Scrollbar.TrackHovered() {
		c = LerpColor(t.Border, t.Dim, 0.5)
	}
	bar := image.Rect(dims.Size.X-gtx.Dp(3)-w, y0+gtx.Dp(2), dims.Size.X-gtx.Dp(3), y1-gtx.Dp(2))
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
	return dims
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
// them by the wrong amount, is a visible skip.
func (l *List) checkContinuity(id string, requested int) {
	y := map[int]int{}
	pos := -l.Position.Offset
	for i := l.Position.First; ; i++ {
		h, ok := l.sizes[i]
		if !ok {
			break
		}
		y[i] = pos
		pos += h
	}
	prev := l.prevY
	l.prevY = y
	l.tr.moved, l.tr.common, l.tr.uneven, l.tr.skipped = 0, false, "", false
	// Check while a scroll gesture is in progress: frames with input, and a
	// few after (rows must hold still then: content changing size mid-scroll
	// is exactly the skip). Outside gestures, content may legitimately move.
	if requested != 0 {
		l.scrolling = 4
	} else if l.scrolling > 0 {
		l.scrolling--
	}
	if l.scrolling == 0 || requested == -1<<30 || prev == nil {
		return
	}
	moved, first, uneven := 0, true, false
	for i, yi := range y {
		p, ok := prev[i]
		if !ok {
			continue
		}
		d := p - yi // positive: content moved up (scrolled down)
		if first {
			moved, first = d, false
		} else if d != moved {
			uneven = true
		}
	}
	if first {
		return // no row in common (a jump of more than a screen)
	}
	l.tr.moved, l.tr.common = moved, true
	if uneven && traceEnabled {
		var b strings.Builder
		for i := l.Position.First; ; i++ {
			yi, ok := y[i]
			if !ok {
				break
			}
			if p, ok := prev[i]; ok {
				fmt.Fprintf(&b, "%d:%+d,", i, p-yi)
			}
		}
		l.tr.uneven = strings.TrimSuffix(b.String(), ",")
	}
	atTop := l.Position.First == 0 && l.Position.Offset == 0
	atEnd := !l.Position.BeforeEnd
	wrong := moved != requested && !(atTop && moved > requested) && !(atEnd && moved < requested)
	if uneven || (wrong && (moved-requested > 1 || moved-requested < -1)) {
		l.jumps++
		l.tr.skipped = true
	}
}

// trace logs this frame's list state under GIOKIT_TRACE. It writes a "list"
// line on every frame that scrolled (or is still settling) or whose items
// changed, plus events for items entering, leaving or changing height, and
// for slow item layouts. A skip the continuity check saw is "LIST_SKIP".
func (l *List) trace(id string, n, viewH, requested, wheel int, pend0 float32, glide int, pos0, before layout.Position) {
	tr := &l.tr
	if tr.known == nil {
		tr.known, tr.visible = map[int]int{}, map[int]bool{}
	}
	p := l.Position
	// Visible rows and their tops, from First down.
	vis := map[int]bool{}
	var rows strings.Builder
	y := -p.Offset
	for i := p.First; ; i++ {
		h, ok := l.sizes[i]
		if !ok || y >= viewH {
			break
		}
		vis[i] = true
		fmt.Fprintf(&rows, "%d@%d+%d,", i, y, h)
		y += h
	}
	changed := false
	for i := range vis {
		if !tr.visible[i] {
			changed = true
			_, seen := tr.known[i]
			tracef("list_enter", "id", id, "i", i, "h", l.sizes[i], "first_time", !seen,
				"build_ms", ms(tr.cost[i]), "prev_h", tr.known[i])
		}
	}
	for i := range tr.visible {
		if !vis[i] {
			changed = true
			tracef("list_leave", "id", id, "i", i, "h", tr.known[i])
		}
	}
	// Height changes, for every item laid out (visible or in the margin).
	for i, h := range l.sizes {
		if old, ok := tr.known[i]; ok && old != h {
			changed = true
			tracef("list_resize", "id", id, "i", i, "from", old, "to", h, "visible", vis[i],
				"above_first", i < p.First)
		}
		tr.known[i] = h
		if c := tr.cost[i]; c > 2*time.Millisecond {
			tracef("list_slow_item", "id", id, "i", i, "build_ms", ms(c), "h", h)
		}
	}
	tr.visible = vis
	if requested == 0 && wheel == 0 && pend0 == 0 && l.scrolling == 0 && !changed && pos0 == p {
		return
	}
	start, end := viewport(p, n, viewH)
	kind := "list"
	if tr.skipped {
		kind = "LIST_SKIP"
	}
	tracef(kind, "id", id, "req", requested, "wheel", wheel, "pend", pend0, "glide", glide,
		"moved", map[bool]any{true: tr.moved, false: "-"}[tr.common], "uneven", tr.uneven,
		// pos: last frame -> after input -> after layout (First/Offset)
		"pos", fmt.Sprintf("%d/%d->%d/%d->%d/%d", pos0.First, pos0.Offset, before.First, before.Offset, p.First, p.Offset),
		"count", p.Count, "off_last", p.OffsetLast, "len", p.Length, "before_end", p.BeforeEnd,
		"view", viewH, "laid", len(l.sizes), "rows", strings.TrimSuffix(rows.String(), ","),
		"vp", fmt.Sprintf("%.4f-%.4f", start, end), "settle", l.scrolling)
}
