package kit

import (
	"image"
	"time"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/paint"
	"github.com/hhhapz/giokit/gio/unit"
)

// Easing maps linear progress 0..1 to eased progress.
type Easing func(t float32) float32

// Linear is constant-speed progress: no easing.
func Linear(t float32) float32 { return t }

// EaseOut starts fast and decelerates (cubic). It is Tween's default.
func EaseOut(t float32) float32 {
	u := 1 - t
	return 1 - u*u*u
}

// EaseInOut accelerates, then decelerates (cubic): for motion between two
// resting places, such as a sliding indicator.
func EaseInOut(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	u := -2*t + 2
	return 1 - u*u*u/2
}

// EaseOutBack overshoots slightly before settling: good for toggles and pops.
func EaseOutBack(t float32) float32 {
	const c1, c3 = 1.70158, 2.70158
	u := t - 1
	return 1 + c3*u*u*u + c1*u*u
}

// Tween animates a float toward a target. Animation in giokit is
// immediate-mode: call To every frame with the value you want, and it
// returns the value to draw this frame. There are no callbacks and nothing
// to start or stop.
//
//	x := s.slide.To(gtx, target, th.Dur(kit.DurNormal))
//
// A moving tween requests an immediate redraw; kittest's Settle chases those
// on a virtual clock until it finishes (compare Loop).
//
// The zero value snaps to the first target it sees (no animation on first
// appearance). A Tween holds no references and may be copied.
type Tween struct {
	from, to float32
	start    time.Time
	dur      time.Duration
	ease     Easing // the current transition's; nil is EaseOut
	init     bool
}

// To returns the value for this frame while moving toward target over d
// (EaseOut). Call it every frame, inside layout.
func (tw *Tween) To(gtx layout.Context, target float32, d time.Duration) float32 {
	return tw.ToEase(gtx, target, d, EaseOut)
}

// ToEase is To with an explicit easing. A new target starts a transition
// from the current value with easing e; while the target is unchanged, d and
// e are ignored and the running transition continues.
func (tw *Tween) ToEase(gtx layout.Context, target float32, d time.Duration, e Easing) float32 {
	if !tw.init {
		tw.from, tw.to, tw.ease, tw.init = target, target, e, true
		return target
	}
	if target != tw.to {
		tw.from, tw.to = tw.value(gtx.Now), target
		tw.start, tw.dur, tw.ease = gtx.Now, d, e
	}
	v := tw.value(gtx.Now)
	if tw.Running(gtx) {
		gtx.Execute(op.InvalidateCmd{})
	}
	return v
}

// From restarts the animation from v toward the current target over d (to
// replay an entrance animation). It uses the easing of the most recent
// target change, or of the first To or ToEase call if the target has not
// changed since (EaseOut before any call).
func (tw *Tween) From(gtx layout.Context, v float32, d time.Duration) {
	tw.from, tw.start, tw.dur, tw.init = v, gtx.Now, d, true
}

// Value is the tween's value at this frame without changing its target.
func (tw *Tween) Value(gtx layout.Context) float32 { return tw.value(gtx.Now) }

// Running reports whether the tween is still moving at this frame's time.
func (tw *Tween) Running(gtx layout.Context) bool {
	return tw.dur > 0 && gtx.Now.Before(tw.start.Add(tw.dur))
}

func (tw *Tween) value(now time.Time) float32 {
	if tw.dur <= 0 || !now.Before(tw.start.Add(tw.dur)) {
		return tw.to
	}
	t := float32(now.Sub(tw.start)) / float32(tw.dur)
	if t < 0 {
		t = 0
	}
	e := tw.ease
	if e == nil {
		e = EaseOut
	}
	return tw.from + (tw.to-tw.from)*e(t)
}

// Loop returns the phase (0..1) of a continuous animation with the given
// period, and schedules the next frame. With period 0 (Motion = 0) it returns
// 0 and schedules nothing: draw a static frame.
//
// Unlike a Tween it is never finished, so it must not keep kittest's Settle
// busy. It asks for a redraw at "this frame's time + 1ns". In a real window
// that moment has passed by the time Gio checks, so Gio animates at vsync,
// the same pacing Tween's immediate requests get; a later time would switch
// Gio to timer pacing (docs/pitfalls.md #20). Settle compares against its
// virtual clock, sees a future time and doesn't chase it, so a UI with a
// spinner still settles.
func Loop(gtx layout.Context, period time.Duration) float32 {
	if period <= 0 {
		return 0
	}
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Nanosecond)})
	return float32(gtx.Now.UnixNano()%int64(period)) / float32(period)
}

// transitionSlide is how far a Transition's new content slides up as it
// fades in.
const transitionSlide unit.Dp = 10

// Transition plays an entrance animation (fade + short slide) whenever key
// changes: use it for page or tab switches. Only the new page is drawn, so
// input never reaches a page that is leaving. The zero value is ready and
// shows its first page without animating. A Transition holds no event
// tags and may be copied.
type Transition struct {
	key  string
	t    Tween
	seen bool
}

// Layout draws w, animating it in if key differs from the previous frame.
func (tr *Transition) Layout(gtx layout.Context, th *Theme, key string, w layout.Widget) layout.Dimensions {
	if !tr.seen {
		tr.seen, tr.key = true, key
	} else if key != tr.key {
		tr.key = key
		tr.t.From(gtx, 0, th.Dur(DurNormal))
	}
	p := tr.t.To(gtx, 1, th.Dur(DurNormal))
	if p >= 1 {
		return w(gtx)
	}
	defer paint.PushOpacity(gtx.Ops, p).Pop()
	dy := gtx.Dp(transitionSlide * unit.Dp(1-p))
	defer op.Offset(image.Pt(0, dy)).Push(gtx.Ops).Pop()
	return w(gtx)
}
