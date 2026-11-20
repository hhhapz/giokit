package kittest

import (
	"fmt"
	"image"
	"time"
	"unicode/utf8"

	"github.com/hhhapz/giokit/gio/f32"
	"github.com/hhhapz/giokit/gio/io/input"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/io/pointer"

	"github.com/hhhapz/giokit/kit"
)

// Pointer events carry times from the harness's input clock, never the wall
// clock, so gestures that compare event times (gesture.Click counting a
// double click) see the same input on every machine. Each new gesture starts
// at least gestureGap after the previous one, and no earlier than the
// virtual clock (Now) since the harness started; the events of one gesture
// follow eventGap apart.
const (
	gestureGap = time.Second      // longer than any double-click window
	eventGap   = time.Millisecond // press to release, double-click halves, drag steps
)

// gestureStart returns the time of a new gesture's first event.
func (h *Harness) gestureStart() time.Duration {
	h.inputTime = max(h.inputTime+gestureGap, h.Now.Sub(epoch))
	return h.inputTime
}

// nextEvent returns the time of the next event of the current gesture.
func (h *Harness) nextEvent() time.Duration {
	h.inputTime += eventGap
	return h.inputTime
}

// Move sends a pointer move to p (pixels).
func (h *Harness) Move(p image.Point) {
	h.tb.Helper()
	h.cursor = f32.Pt(float32(p.X), float32(p.Y))
	h.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: h.cursor, Time: h.gestureStart()})
	h.after(fmt.Sprintf("move to %v", p))
}

// Click moves to p, then presses and releases the primary button.
func (h *Harness) Click(p image.Point) {
	h.tb.Helper()
	h.Move(p)
	h.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: h.cursor, Time: h.nextEvent()},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: h.cursor, Time: h.nextEvent()},
	)
	h.after(fmt.Sprintf("click at %v", p))
}

// ClickMods clicks at p holding modifiers (Shift-click to extend, Ctrl-click
// to toggle).
func (h *Harness) ClickMods(p image.Point, mods key.Modifiers) {
	h.tb.Helper()
	h.Move(p)
	h.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: h.cursor, Modifiers: mods, Time: h.nextEvent()},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: h.cursor, Modifiers: mods, Time: h.nextEvent()},
	)
	h.after(fmt.Sprintf("%v-click at %v", mods, p))
}

// DoubleClick clicks twice at p in quick succession (gesture.Click reports
// NumClicks 2 for the second).
func (h *Harness) DoubleClick(p image.Point) {
	h.tb.Helper()
	h.Move(p)
	press := pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: h.cursor}
	release := pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: h.cursor}
	for range 2 {
		press.Time = h.nextEvent()
		release.Time = h.nextEvent()
		h.router.Queue(press, release)
	}
	h.after(fmt.Sprintf("double click at %v", p))
}

// RightClick moves to p, then presses and releases the secondary button.
func (h *Harness) RightClick(p image.Point) {
	h.tb.Helper()
	h.Move(p)
	h.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonSecondary, Position: h.cursor, Time: h.nextEvent()},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: h.cursor, Time: h.nextEvent()},
	)
	h.after(fmt.Sprintf("right-click at %v", p))
}

// Scroll scrolls by dy pixels (positive = down) with the pointer at p.
func (h *Harness) Scroll(p image.Point, dy float32) {
	h.tb.Helper()
	h.Move(p)
	h.router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: h.cursor, Scroll: f32.Pt(0, dy), Time: h.nextEvent()})
	h.after(fmt.Sprintf("scroll %v at %v", dy, p))
}

// Drag presses at from, moves to to in a few steps, and releases there.
func (h *Harness) Drag(from, to image.Point) {
	h.tb.Helper()
	h.Move(from)
	h.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: h.cursor, Time: h.nextEvent()})
	h.Settle()
	const steps = 4
	for i := 1; i <= steps; i++ {
		p := from.Add(to.Sub(from).Mul(i).Div(steps))
		h.cursor = f32.Pt(float32(p.X), float32(p.Y))
		h.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: h.cursor, Time: h.nextEvent()})
		h.Settle()
	}
	h.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: h.cursor, Time: h.nextEvent()})
	h.after(fmt.Sprintf("drag %v→%v", from, to))
}

// DragID drags the widget tagged id by (dx, dy) pixels from the center of
// its visible part.
func (h *Harness) DragID(id string, dx, dy int) {
	h.tb.Helper()
	c := Center(h.mustVisible(id))
	h.Drag(c, c.Add(image.Pt(dx, dy)))
}

// ScrollID scrolls the region tagged id by dy pixels.
func (h *Harness) ScrollID(id string, dy float32) {
	h.tb.Helper()
	h.Scroll(Center(h.mustVisible(id)), dy)
}

// Hover moves to the center of the visible part of the widget tagged id.
func (h *Harness) Hover(id string) {
	h.tb.Helper()
	h.Move(Center(h.mustVisible(id)))
}

// ClickID clicks the center of the visible part of the widget tagged id.
// A widget clipped out of view (laid out past the end of a scrolled list)
// fails the test rather than clicking whatever covers that point.
func (h *Harness) ClickID(id string) {
	h.tb.Helper()
	h.Click(Center(h.mustVisible(id)))
}

// ClickIDMods clicks the widget tagged id holding modifiers.
func (h *Harness) ClickIDMods(id string, mods key.Modifiers) {
	h.tb.Helper()
	h.ClickMods(Center(h.mustVisible(id)), mods)
}

// DoubleClickID double-clicks the widget tagged id.
func (h *Harness) DoubleClickID(id string) {
	h.tb.Helper()
	h.DoubleClick(Center(h.mustVisible(id)))
}

// RightClickID right-clicks the widget tagged id (context menus).
func (h *Harness) RightClickID(id string) {
	h.tb.Helper()
	h.RightClick(Center(h.mustVisible(id)))
}

// Key presses and releases a key. Tab and Shift+Tab move focus unless the
// UI handled them. As in app.Window, a key counts as handled when delivering
// it requested a redraw: a handler that consumes Tab without calling Notify
// (or otherwise invalidating) lets focus move as well.
func (h *Harness) Key(name key.Name, mods key.Modifiers) {
	h.tb.Helper()
	press := key.Event{Name: name, Modifiers: mods, State: key.Press}
	if dir, ok := tabDirection(name, mods); ok {
		h.router.Queue(input.SystemEvent{Event: press})
		if _, handled := h.router.WakeupTime(); !handled {
			h.router.MoveFocus(dir)
		}
	} else {
		h.router.Queue(press)
	}
	h.router.Queue(key.Event{Name: name, Modifiers: mods, State: key.Release})
	h.after(fmt.Sprintf("key %v%v", mods, name))
}

// KeyCode presses and releases the physical key code, which the simulated
// keyboard layout types as name: KeyCode("KeyQ", "A", 0) is the top-left
// letter on AZERTY. It tests bindings by place (see key.Code) on other
// layouts; Key sends no code, so such bindings see the US layout.
func (h *Harness) KeyCode(code key.Code, name key.Name, mods key.Modifiers) {
	h.tb.Helper()
	h.router.Queue(
		key.Event{Name: name, Code: code, Modifiers: mods, State: key.Press},
		key.Event{Name: name, Code: code, Modifiers: mods, State: key.Release},
	)
	h.after(fmt.Sprintf("key %v%v (%v)", mods, name, code))
}

// KeyDown presses a key and holds it: a held modifier ("Ctrl" while
// testing a hint that shows during the hold). Release it with KeyUp.
func (h *Harness) KeyDown(name key.Name, mods key.Modifiers) {
	h.tb.Helper()
	h.router.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	h.after(fmt.Sprintf("key down %v%v", mods, name))
}

// KeyUp releases a key KeyDown pressed.
func (h *Harness) KeyUp(name key.Name, mods key.Modifiers) {
	h.tb.Helper()
	h.router.Queue(key.Event{Name: name, Modifiers: mods, State: key.Release})
	h.after(fmt.Sprintf("key up %v%v", mods, name))
}

// Keys presses and releases each key in turn, delivering them all to the
// same frame, as a real window does when keys arrive faster than frames
// (key repeat, a fast typist, a busy UI goroutine). Key settles between
// keys; Keys tests a handler against a batch. Tab and Shift+Tab are
// delivered to the UI but do not move focus.
func (h *Harness) Keys(names ...key.Name) {
	h.tb.Helper()
	for _, name := range names {
		h.router.Queue(
			key.Event{Name: name, State: key.Press},
			key.Event{Name: name, State: key.Release},
		)
	}
	h.after(fmt.Sprintf("keys %v", names))
}

// tabDirection reports the focus move an unhandled key makes, if any.
func tabDirection(name key.Name, mods key.Modifiers) (key.FocusDirection, bool) {
	switch {
	case name == key.NameTab && mods == 0:
		return key.FocusForward, true
	case name == key.NameTab && mods == key.ModShift:
		return key.FocusBackward, true
	}
	return 0, false
}

// Chord presses a chord written like a kit.Keymap binding ("Ctrl+1",
// "Shortcut+K", "Escape").
func (h *Harness) Chord(chord string) {
	h.tb.Helper()
	n, m, err := kit.ParseChord(chord)
	if err != nil {
		h.fatal(fmt.Sprintf("Chord %q: %v", chord, err))
	}
	h.Key(n, m)
}

// Tab moves focus forward (or backward with back=true).
func (h *Harness) Tab(back bool) {
	h.tb.Helper()
	if back {
		h.Key(key.NameTab, key.ModShift)
	} else {
		h.Key(key.NameTab, 0)
	}
}

// Type inserts text into the focused editor, replacing its selection.
func (h *Harness) Type(s string) {
	h.tb.Helper()
	// As app.Window's EditorInsert does: replace the selection, then put the
	// caret after the inserted text (without the second event the caret
	// stayed in front of text typed mid-field).
	sel := h.router.EditorState().Selection.Range
	start := min(sel.Start, sel.End) + utf8.RuneCountInString(s)
	h.router.Queue(
		key.EditEvent{Range: sel, Text: s},
		key.SelectionEvent{Start: start, End: start},
	)
	h.after(fmt.Sprintf("type %q", s))
}

// Cursor is the pointer cursor the UI requests at the current pointer position.
func (h *Harness) Cursor() pointer.Cursor { return h.router.Cursor() }
