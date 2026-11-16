package kit

import (
	"image"
	"math"
	"time"

	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Animation in giokit is immediate-mode: you call a tween every frame with the
// value you want, and it returns the value to draw this frame. No callbacks,
// no animation objects to start or stop.
//
//	x := s.slide.To(gtx, target, th.Dur(kit.DurNormal))
//
// Two kinds of redraw requests, and kittest relies on the difference:
//   - finite animations (Tween) request an immediate redraw while moving;
//     kittest's Settle chases those on a virtual clock until they finish.
//   - continuous animations (Loop: spinners) schedule the next frame for just
//     after the current frame time: vsync-paced in a window, not chased by
//     Settle (a UI showing a spinner still settles).

// Easing maps linear progress 0..1 to eased progress.
type Easing func(t float32) float32

var (
	Linear    Easing = func(t float32) float32 { return t }
	EaseOut   Easing = func(t float32) float32 { u := 1 - t; return 1 - u*u*u }
	EaseInOut Easing = func(t float32) float32 {
		if t < 0.5 {
			return 4 * t * t * t
		}
		u := -2*t + 2
		return 1 - u*u*u/2
	}
	// EaseOutBack overshoots slightly: good for toggles and pops.
	EaseOutBack Easing = func(t float32) float32 {
		const c1, c3 = 1.70158, 2.70158
		u := t - 1
		return 1 + c3*u*u*u + c1*u*u
	}
)

// Tween animates a float toward a target. Its zero value snaps to the first
// target it sees (no animation on first appearance).
type Tween struct {
	from, to float32
	start    time.Time
	dur      time.Duration
	ease     Easing
	init     bool
}

// To returns the value for this frame while moving toward target over d
// (EaseOut). Call it every frame, inside layout.
func (a *Tween) To(gtx C, target float32, d time.Duration) float32 {
	return a.ToEase(gtx, target, d, EaseOut)
}

// ToEase is To with an explicit easing.
func (a *Tween) ToEase(gtx C, target float32, d time.Duration, e Easing) float32 {
	if !a.init {
		a.from, a.to, a.init = target, target, true
		return target
	}
	if target != a.to {
		a.from, a.to = a.value(gtx.Now), target
		a.start, a.dur, a.ease = gtx.Now, d, e
	}
	v := a.value(gtx.Now)
	if a.Running(gtx) {
		gtx.Execute(op.InvalidateCmd{})
	}
	return v
}

// From restarts the animation from v toward the current target (e.g. replay
// an entrance animation).
func (a *Tween) From(gtx C, v float32, d time.Duration) {
	a.from, a.start, a.dur, a.ease, a.init = v, gtx.Now, d, EaseOut, true
}

// Value is the tween's value at this frame without changing its target.
func (a *Tween) Value(gtx C) float32 { return a.value(gtx.Now) }

// Running reports whether the tween is still moving.
func (a *Tween) Running(gtx C) bool {
	return a.dur > 0 && gtx.Now.Before(a.start.Add(a.dur))
}

func (a *Tween) value(now time.Time) float32 {
	if a.dur <= 0 || !now.Before(a.start.Add(a.dur)) {
		return a.to
	}
	t := float32(now.Sub(a.start)) / float32(a.dur)
	if t < 0 {
		t = 0
	}
	e := a.ease
	if e == nil {
		e = EaseOut
	}
	return a.from + (a.to-a.from)*e(t)
}

// Loop returns the phase (0..1) of a continuous animation with the given
// period, and schedules the next frame. With period 0 (Motion = 0) it returns
// 0 and schedules nothing: draw a static frame.
//
// It asks for a redraw at "this frame's time + 1ns". In a real window that
// moment has passed by the time Gio checks, so Gio animates at vsync, the
// same pacing Tween's immediate requests get. (Asking for now+16ms made Gio
// switch to timer pacing and back whenever a tween ran at the same time; the
// window stalled for 150–200ms when a spinner scrolled in or out of view.)
// kittest's Settle compares against its virtual clock, sees a future time
// and doesn't chase it, so a UI with a spinner still settles.
func Loop(gtx C, period time.Duration) float32 {
	if period <= 0 {
		return 0
	}
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Nanosecond)})
	return float32(gtx.Now.UnixNano()%int64(period)) / float32(period)
}

// Transition plays an entrance animation (fade + short slide) whenever key
// changes: use it for page or tab switches. Only the new page is drawn, so
// input never reaches a page that is leaving.
type Transition struct {
	key  string
	t    Tween
	seen bool
}

// Layout draws w, animating it in if key differs from the previous frame.
func (tr *Transition) Layout(gtx C, th *Theme, key string, w W) D {
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
	dy := gtx.Dp(unit.Dp(10 * (1 - p)))
	defer op.Offset(image.Pt(0, dy)).Push(gtx.Ops).Pop()
	return w(gtx)
}

func sin01(phase float32) float32 { return float32(math.Sin(float64(phase) * 2 * math.Pi)) }
