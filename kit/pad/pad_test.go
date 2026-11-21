package pad_test

import (
	"image"
	"testing"
	"time"

	"github.com/hhhapz/giokit/gio/io/input"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/pad"
	"github.com/hhhapz/giokit/kittest"
)

// clock drives Pads frames by hand.
type clock struct {
	r   input.Router
	now time.Time
}

func (c *clock) frame(p *pad.Pads, d time.Duration) []pad.Event {
	c.now = c.now.Add(d)
	p.Frame(layout.Context{Now: c.now, Source: c.r.Source()})
	return p.Events()
}

func kinds(evs []pad.Event, b pad.Button) []pad.EventKind {
	var ks []pad.EventKind
	for _, e := range evs {
		if e.Button == b {
			ks = append(ks, e.Kind)
		}
	}
	return ks
}

// TestEventsAndRepeat checks presses, releases, auto-repeat timing and the
// frame Pads schedules for the next repeat.
func TestEventsAndRepeat(t *testing.T) {
	var f pad.Fake
	p := pad.NewSource(&f, nil)
	c := &clock{now: time.Unix(0, 0)}
	id := f.Connect("test", pad.PlayStation)
	c.frame(p, 0)

	f.Set(id, pad.South, true)
	if got := kinds(c.frame(p, time.Millisecond), pad.South); len(got) != 1 || got[0] != pad.Press {
		t.Fatalf("press: %v", got)
	}
	if p.Family() != pad.PlayStation {
		t.Errorf("family %d", p.Family())
	}
	if wake, ok := c.r.WakeupTime(); !ok || !wake.Equal(c.now.Add(pad.RepeatDelay)) {
		t.Errorf("next frame at %v (%v), want the repeat delay", wake, ok)
	}
	if got := kinds(c.frame(p, pad.RepeatDelay-time.Millisecond), pad.South); len(got) != 0 {
		t.Fatalf("repeat too early: %v", got)
	}
	if got := kinds(c.frame(p, time.Millisecond), pad.South); len(got) != 1 || got[0] != pad.Repeat {
		t.Fatalf("repeat: %v", got)
	}
	if got := kinds(c.frame(p, pad.RepeatInterval), pad.South); len(got) != 1 || got[0] != pad.Repeat {
		t.Fatalf("second repeat: %v", got)
	}
	f.Set(id, pad.South, false)
	if got := kinds(c.frame(p, time.Millisecond), pad.South); len(got) != 1 || got[0] != pad.Release {
		t.Fatalf("release: %v", got)
	}

	// Unplugging releases what was held.
	f.Set(id, pad.North, true)
	c.frame(p, time.Millisecond)
	f.Disconnect(id)
	if got := kinds(c.frame(p, time.Millisecond), pad.North); len(got) != 1 || got[0] != pad.Release {
		t.Fatalf("unplug: %v", got)
	}
	if len(p.Connected()) != 0 {
		t.Errorf("still connected: %v", p.Connected())
	}
}

// TestStickAsDPad checks the left stick presses the d-pad with hysteresis,
// and triggers press past half travel.
func TestStickAsDPad(t *testing.T) {
	var f pad.Fake
	p := pad.NewSource(&f, nil)
	c := &clock{now: time.Unix(0, 0)}
	id := f.Connect("test", pad.Xbox)
	for _, s := range []struct {
		x    float32
		want bool
	}{{0.3, false}, {0.6, true}, {0.4, true}, {0.3, false}, {0.4, false}} {
		f.SetAxis(id, pad.LeftX, s.x)
		c.frame(p, time.Millisecond)
		if got := p.Connected()[0].Buttons.Has(pad.Right); got != s.want {
			t.Errorf("stick at %.1f: right %v, want %v", s.x, got, s.want)
		}
	}
	f.SetAxis(id, pad.LeftY, -1)
	f.SetAxis(id, pad.RightTriggerAxis, 0.8)
	c.frame(p, time.Millisecond)
	if b := p.Connected()[0].Buttons; !b.Has(pad.Up) || !b.Has(pad.RightTrigger) {
		t.Errorf("buttons %b: want Up and RightTrigger", b)
	}
	p.StickAsDPad = false
	c.frame(p, time.Millisecond)
	if p.Connected()[0].Buttons.Has(pad.Up) {
		t.Error("stick pressed Up with StickAsDPad off")
	}
}

type host struct{ focused bool }

func (h *host) Invalidate()   {}
func (h *host) Focused() bool { return h.focused }

// TestUnfocused checks a background window sees every button come up.
func TestUnfocused(t *testing.T) {
	var f pad.Fake
	h := &host{focused: true}
	p := pad.NewSource(&f, h)
	c := &clock{now: time.Unix(0, 0)}
	id := f.Connect("test", pad.Xbox)
	f.Set(id, pad.South, true)
	c.frame(p, 0)
	h.focused = false
	if got := kinds(c.frame(p, time.Millisecond), pad.South); len(got) != 1 || got[0] != pad.Release {
		t.Fatalf("blur: %v", got)
	}
	if got := kinds(c.frame(p, time.Second), pad.South); len(got) != 0 {
		t.Fatalf("events while blurred: %v", got)
	}
}

// ownFocusFake is a source that, like the web's, handles focus itself.
type ownFocusFake struct{ *pad.Fake }

func (ownFocusFake) OwnFocus() bool { return true }

// TestOwnFocus checks a source that handles focus itself (the web's) keeps
// its input while the host reports no focus.
func TestOwnFocus(t *testing.T) {
	f := &pad.Fake{}
	p := pad.NewSource(ownFocusFake{f}, &host{focused: false})
	c := &clock{now: time.Unix(0, 0)}
	id := f.Connect("test", pad.Xbox)
	f.Set(id, pad.South, true)
	if got := kinds(c.frame(p, 0), pad.South); len(got) != 1 || got[0] != pad.Press {
		t.Fatalf("press with the host unfocused: %v", got)
	}
}

// TestMap checks maps run in order and consume what they handle, BindIf
// passes a press on while disabled, and only Repeat bindings repeat.
func TestMap(t *testing.T) {
	var f pad.Fake
	p := pad.NewSource(&f, nil)
	c := &clock{now: time.Unix(0, 0)}
	id := f.Connect("test", pad.Xbox)
	var log []string
	menuOpen := false
	var menu, game pad.Map
	menu.BindIf(pad.South, "Choose", func() bool { return menuOpen }, func() { log = append(log, "menu") })
	game.Bind(pad.South, "Place", func() { log = append(log, "game") })
	game.BindRepeat(pad.Down, "Move", func() { log = append(log, "down") })
	run := func(d time.Duration) {
		c.frame(p, d)
		menu.Run(layout.Context{}, p)
		game.Run(layout.Context{}, p)
	}
	f.Set(id, pad.South, true)
	run(time.Millisecond)
	f.Set(id, pad.South, false)
	run(time.Millisecond)
	menuOpen = true
	f.Set(id, pad.South, true)
	run(time.Millisecond)
	run(pad.RepeatDelay) // South doesn't repeat
	f.Set(id, pad.Down, true)
	run(time.Millisecond)
	run(pad.RepeatDelay)
	want := []string{"game", "menu", "down", "down"}
	if len(log) != len(want) {
		t.Fatalf("ran %v, want %v", log, want)
	}
	for i := range want {
		if log[i] != want[i] {
			t.Fatalf("ran %v, want %v", log, want)
		}
	}
	if n := len(game.Bindings()); n != 2 {
		t.Errorf("%d game bindings", n)
	}
}

// TestNav checks the d-pad moves focus between real controls, South clicks
// the focused one and East runs back.
func TestNav(t *testing.T) {
	th := kit.Dark()
	var f pad.Fake
	p := pad.NewSource(&f, nil)
	var a, b kit.Clickable
	clicked, backs := "", 0
	h := kittest.NewT(t, 200, 200, func(gtx layout.Context) {
		p.Frame(gtx)
		pad.Nav(gtx, p, func() { backs++ })
		if a.Clicked(gtx) {
			clicked += "a"
		}
		if b.Clicked(gtx) {
			clicked += "b"
		}
		for i, c := range []*kit.Clickable{&a, &b} {
			id := [...]string{"a", "b"}[i]
			r := image.Rect(0, i*100, 200, i*100+80)
			st := op.Offset(r.Min).Push(gtx.Ops)
			gtx.Constraints = layout.Exact(r.Size())
			th.Press(gtx, c, id, func(gtx layout.Context, st kit.State) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
			st.Pop()
		}
	})
	if h == nil {
		t.Skip("no GPU")
	}
	h.Theme = th
	id := f.Connect("test", pad.Xbox)
	h.Settle()
	tap := func(btn pad.Button) {
		f.Set(id, btn, true)
		h.Settle()
		f.Set(id, btn, false)
		h.Settle()
	}
	tap(pad.Down) // nothing focused: the first control
	if got := h.FocusedID(); got != "a" {
		t.Fatalf("focus %q after the first Down, want a", got)
	}
	tap(pad.Down)
	if got := h.FocusedID(); got != "b" {
		t.Fatalf("focus %q after Down, want b", got)
	}
	tap(pad.South)
	tap(pad.Up)
	tap(pad.South)
	tap(pad.East)
	if clicked != "ba" || backs != 1 {
		t.Fatalf("clicked %q, backs %d; want ba, 1", clicked, backs)
	}
}

func TestLabels(t *testing.T) {
	for _, c := range []struct {
		f    pad.Family
		b    pad.Button
		want string
	}{
		{pad.Xbox, pad.South, "A"}, {pad.PlayStation, pad.South, "✕"}, {pad.Nintendo, pad.South, "B"},
		{pad.Nintendo, pad.East, "A"}, {pad.Generic, pad.Start, "MENU"}, {pad.PlayStation, pad.LeftTrigger, "L2"},
	} {
		if got := pad.Label(c.f, c.b); got != c.want {
			t.Errorf("Label(%d, %d) = %q, want %q", c.f, c.b, got, c.want)
		}
	}
	if got := pad.ASCIILabel(pad.PlayStation, pad.North); got != "TRIANGLE" {
		t.Errorf("ASCIILabel triangle = %q", got)
	}
}

// TestNintendoConfirm checks that with NintendoConfirm, Nintendo pads' A
// button (east on the pad) reports South, with labels following, while X
// and Y stay by position; and that it is off by default.
func TestNintendoConfirm(t *testing.T) {
	var f pad.Fake
	p := pad.NewSource(&f, nil)
	if p.NintendoConfirm {
		t.Fatal("NintendoConfirm on by default")
	}
	p.NintendoConfirm = true
	c := &clock{now: time.Unix(0, 0)}
	id := f.Connect("Pro Controller", pad.Nintendo)
	f.Set(id, pad.East, true)  // the A button
	f.Set(id, pad.North, true) // the X button, on top
	evs := c.frame(p, time.Millisecond)
	if got := kinds(evs, pad.South); len(got) != 1 || got[0] != pad.Press {
		t.Fatalf("A on a Nintendo pad: South events %v", got)
	}
	if got := kinds(evs, pad.North); len(got) != 1 || got[0] != pad.Press {
		t.Fatalf("X on a Nintendo pad: North events %v", got)
	}
	for b, want := range map[pad.Button]string{pad.South: "A", pad.East: "B", pad.West: "Y", pad.North: "X", pad.Start: "+"} {
		if got := p.Label(b); got != want {
			t.Errorf("Label(%d) = %q, want %q", b, got, want)
		}
	}
	p.NintendoConfirm = false
	if got := kinds(c.frame(p, time.Millisecond), pad.East); len(got) != 1 || got[0] != pad.Press {
		t.Fatalf("by position: East events %v", got)
	}
	if got := p.Label(pad.South); got != "B" {
		t.Errorf("by position: Label(South) = %q, want B", got)
	}
}
