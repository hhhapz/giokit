package kit

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/hhhapz/giokit/gio/f32"
	"github.com/hhhapz/giokit/gio/io/input"
	"github.com/hhhapz/giokit/gio/io/pointer"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/unit"
)

// scrollScript drives th.List frame by frame with raw wheel events and
// returns the position after each frame, so the smoothing can be compared
// exactly across refactors. events maps a frame number to the pointer
// events queued before that frame; their times are filled in.
func scrollScript(th *Theme, frames int, events map[int][]pointer.Event) string {
	var (
		router input.Router
		ops    op.Ops
		l      List
		b      strings.Builder
	)
	now := time.Unix(0, 0)
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(50, 150)})
	for f := range frames {
		for _, e := range events[f] {
			e.Source, e.Time = pointer.Mouse, time.Duration(f)*16*time.Millisecond
			router.Queue(e)
		}
		ops.Reset()
		gtx := layout.Context{
			Ops:         &ops,
			Now:         now.Add(time.Duration(f) * 16 * time.Millisecond),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(200, 300)),
			Source:      router.Source(),
		}
		th.List(gtx, &l, "list", 100, func(gtx layout.Context, i int) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 20+i%3*7)}
		})
		router.Frame(&ops)
		fmt.Fprintf(&b, "%d/%d ", l.Position.First, l.Position.Offset)
	}
	fmt.Fprintf(&b, "jumps=%d", l.Jumps())
	return b.String()
}

// TestListSmoothingUnchanged pins the wheel smoothing frame by frame: a
// notch glides over several frames, small deltas apply at once, and the
// glide stops at the ends. Scroll feel must not change as a side effect of
// refactoring; update the golden values only for an intended change.
func TestListSmoothingUnchanged(t *testing.T) {
	wheel := func(dy float32) []pointer.Event {
		return []pointer.Event{{Kind: pointer.Scroll, Position: f32.Pt(50, 150), Scroll: f32.Pt(0, dy)}}
	}
	thumb := func(k pointer.Kind, y float32) []pointer.Event {
		return []pointer.Event{{Kind: k, Position: f32.Pt(195, y), Buttons: pointer.ButtonPrimary}}
	}
	events := map[int][]pointer.Event{
		1: wheel(120), 2: wheel(120), 12: wheel(3), 13: wheel(-5), 20: wheel(-120),
		30: wheel(-400), 40: wheel(4000),
		44: thumb(pointer.Press, 290), 45: thumb(pointer.Move, 200), 46: thumb(pointer.Move, 150),
		47: thumb(pointer.Release, 150), 48: wheel(60),
	}
	for _, tc := range []struct {
		name   string
		motion float32
		want   string
	}{
		{"motion", 1, "0/0 1/4 2/21 4/2 5/2 5/24 6/7 7/1 7/12 7/21 8/1 8/7 8/12 8/15 8/17 8/19 8/21 8/22 8/23 8/24 8/1 7/10 6/15 6/3 5/28 5/21 5/15 5/10 5/6 5/3 2/0 0/0 0/0 0/0 0/0 0/0 0/0 0/0 0/0 0/0 30/7 54/9 73/20 88/24 88/24 61/6 44/22 44/22 45/0 45/9 45/16 46/2 46/7 46/11 46/14 46/16 46/18 46/20 46/21 46/22 jumps=0"},
		{"no motion", 0, "0/0 4/19 8/31 8/31 8/31 8/31 8/31 8/31 8/31 8/31 8/31 8/31 9/0 8/29 8/29 8/29 8/29 8/29 8/29 8/29 4/17 4/17 4/17 4/17 4/17 4/17 4/17 4/17 4/17 4/17 0/0 0/0 0/0 0/0 0/0 0/0 0/0 0/0 0/0 0/0 88/24 88/24 88/24 88/24 88/24 61/6 44/22 44/22 47/1 47/1 47/1 47/1 47/1 47/1 47/1 47/1 47/1 47/1 47/1 47/1 jumps=0"},
	} {
		th := Light()
		th.Motion = tc.motion
		if got := scrollScript(th, 60, events); got != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, got, tc.want)
		}
	}
}
