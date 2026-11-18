package kit

import (
	"fmt"
	"strings"
	"time"

	"gioui.org/layout"
)

// slowItem is the item layout time GIOKIT_TRACE reports as slow.
const slowItem = 2 * time.Millisecond

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

// trace logs this frame's list state under GIOKIT_TRACE. It writes a "list"
// line on every frame that scrolled (or is still settling) or whose items
// changed, plus events for items entering, leaving or changing height, and
// for slow item layouts. A skip the continuity check saw is "LIST_SKIP".
func (l *List) trace(id string, fr listFrame) {
	tr := &l.tr
	if tr.known == nil {
		tr.known, tr.visible = map[int]int{}, map[int]bool{}
	}
	p, viewH := l.Position, fr.viewH
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
		if c := tr.cost[i]; c > slowItem {
			tracef("list_slow_item", "id", id, "i", i, "build_ms", ms(c), "h", h)
		}
	}
	tr.visible = vis
	if fr.requested == 0 && !fr.dragged && fr.wheel == 0 && fr.pending == 0 && l.scrolling == 0 && !changed && fr.pos0 == p {
		return
	}
	start, end := viewport(p, fr.n, viewH)
	kind := "list"
	if tr.skipped {
		kind = "LIST_SKIP"
	}
	var moved any = "-"
	if tr.common {
		moved = tr.moved
	}
	pos0, before := fr.pos0, fr.before
	tracef(kind, "id", id, "req", fr.requested, "drag", fr.dragged, "wheel", fr.wheel, "pend", fr.pending, "glide", fr.glide,
		"moved", moved, "uneven", tr.uneven,
		// pos: last frame -> after input -> after layout (First/Offset)
		"pos", fmt.Sprintf("%d/%d->%d/%d->%d/%d", pos0.First, pos0.Offset, before.First, before.Offset, p.First, p.Offset),
		"count", p.Count, "off_last", p.OffsetLast, "len", p.Length, "before_end", p.BeforeEnd,
		"view", viewH, "laid", len(l.sizes), "rows", strings.TrimSuffix(rows.String(), ","),
		"vp", fmt.Sprintf("%.4f-%.4f", start, end), "settle", l.scrolling)
}

// timed wraps a list element to record each item's layout time in tr.cost,
// and to report an item laid out twice in one frame. sizes are the heights
// recorded so far this frame.
func (tr *listTrace) timed(id string, sizes map[int]int, element layout.ListElement) layout.ListElement {
	if tr.cost == nil {
		tr.cost = map[int]time.Duration{}
	}
	clear(tr.cost)
	return func(gtx layout.Context, i int) layout.Dimensions {
		prev, again := sizes[i]
		start := time.Now()
		d := element(gtx, i)
		tr.cost[i] += time.Since(start)
		if again {
			tracef("list_relayout", "id", id, "i", i, "h", d.Size.Y, "prev_h", prev)
		}
		return d
	}
}
