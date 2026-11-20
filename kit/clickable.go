package kit

import (
	"image"
	"time"

	"github.com/hhhapz/giokit/gio/gesture"
	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/io/pointer"
	"github.com/hhhapz/giokit/gio/io/semantic"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"
)

// Clickable is a pressable area that notifies whenever it reports a click.
// It is keyboard-focusable: Tab reaches it, Enter/Space click it. Inside a
// FocusGroup it is not a Tab stop of its own; the group is.
//
// kit owns this rather than wrapping widget.Clickable, because Gio puts every
// tag that asks for focus events into the Tab order and widget.Clickable
// always asks.
//
// The zero value is ready to use. A Clickable is its own event tag (and its
// FocusGroup refers to it by address), so it must not be copied or moved
// after its first Layout: keep it in a struct field or a Pool, not in a
// slice that grows. Like all kit state it belongs to the UI goroutine.
type Clickable struct {
	// FocusOnClick focuses the clickable when clicked, without showing the
	// focus ring until the keyboard is used (list items: so arrow keys work
	// right after a click).
	FocusOnClick bool
	click        gesture.Click
	requests     int
	pressedKey   key.Name
	hover        Tween
	anim         Tween         // free for the component drawing it (switch thumb, radio dot)
	ring         FocusVisible  // focused by a click: hide the ring
	disabled     bool          // laid out disabled last frame (Theme.Disable)
	group        *FocusGroup   // member of a group: no Tab stop, no Enter/Space
	mods         key.Modifiers // of the click Clicked last reported
}

// Modifiers are the keys held during the click Clicked last reported
// (Shift-click to extend a selection, Ctrl-click to toggle); none for a
// keyboard or programmatic click.
func (c *Clickable) Modifiers() key.Modifiers { return c.mods }

// FocusTag is the tag Gio routes this clickable's focus and key events to.
func (c *Clickable) FocusTag() event.Tag { return c }

// Click queues a programmatic click, reported by the next Clicked.
func (c *Clickable) Click() { c.requests++ }

// Hovered reports whether a pointer is over the clickable.
func (c *Clickable) Hovered() bool { return c.click.Hovered() }

// Pressed reports whether a pointer is pressing the clickable.
func (c *Clickable) Pressed() bool { return c.click.Pressed() }

func (c *Clickable) update(gtx layout.Context) bool {
	c.mods = 0
	if c.requests > 0 {
		c.requests = 0
		return true
	}
	for {
		e, ok := c.click.Update(gtx.Source)
		if !ok {
			break
		}
		if e.Kind == gesture.KindClick {
			c.mods = e.Modifiers
			return true
		}
	}
	if c.group != nil {
		return false // asking for focus events would make it a Tab stop
	}
	for {
		e, ok := gtx.Event(
			key.FocusFilter{Target: c},
			key.Filter{Focus: c, Name: key.NameReturn},
			key.Filter{Focus: c, Name: key.NameSpace},
		)
		if !ok {
			return false
		}
		switch e := e.(type) {
		case key.FocusEvent:
			if e.Focus {
				c.pressedKey = ""
			}
		case key.Event:
			if !gtx.Focused(c) || (e.Name != key.NameReturn && e.Name != key.NameSpace) {
				break
			}
			switch e.State {
			case key.Press:
				c.pressedKey = e.Name
			case key.Release:
				if c.pressedKey == e.Name { // pressed and released while focused
					c.pressedKey = ""
					return true
				}
			}
		}
	}
}

// Layout lays out w and makes its bounds the clickable's press and focus
// area, with no hover or press drawing of its own (Theme.Press adds those,
// and the id and role). Clicks that arrived since the last Clicked call
// are dropped: read them with Clicked in the handle pass, before Layout.
func (c *Clickable) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	for c.update(gtx) {
		// Drop it: a click Clicked didn't read before layout is stale.
	}
	if c.group == nil {
		trackFocus(gtx, c)
	}
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	semantic.EnabledOp(gtx.Enabled()).Add(gtx.Ops)
	c.click.Add(gtx.Ops)
	event.Op(gtx.Ops, c)
	call.Add(gtx.Ops)
	return dims
}

// Clicked reports a click since the last call, and consumes it: each click
// is reported once. A click also focuses the group (inside a FocusGroup) or
// the clickable (with FocusOnClick), and schedules a redraw. Call it in your
// handle pass, before the widget is laid out: Layout consumes and drops
// pending clicks. It reports at most one click per call.
//
// A clickable laid out disabled (Theme.Disable) reports nothing and doesn't
// even ask for events: asking is what makes Gio deliver clicks and put it
// in the Tab order.
func (c *Clickable) Clicked(gtx layout.Context) bool {
	if c.disabled {
		if gtx.Focused(c) { // focused just before it was disabled
			gtx.Execute(key.FocusCmd{})
			Notify(gtx)
		}
		return false
	}
	if c.update(gtx) {
		if c.group != nil {
			c.group.focus.RequestQuiet(gtx)
		} else if c.FocusOnClick && !gtx.Focused(c) {
			c.FocusQuiet(gtx)
		}
		Notify(gtx)
		return true
	}
	return false
}

// State is the live interaction state handed to a component's draw callback.
// It is all false inside Theme.Disable (which also dims the drawing).
type State struct {
	Hovered, Pressed bool // the pointer is over the area; a press is held on it
	// Focused reports focus with a visible ring: draw one. Focus a click
	// gave (FocusOnClick, a FocusGroup) leaves it false until the keyboard
	// is used.
	Focused bool
	Hover   float32 // Hovered, animated 0..1: lerp tints with it
}

// press lays out w as c's area with a pointer cursor and semantic id,
// animating hover over d.
func (c *Clickable) press(gtx layout.Context, id string, d time.Duration, w func(gtx layout.Context, st State) layout.Dimensions) layout.Dimensions {
	c.disabled = !gtx.Enabled()
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		Tag(gtx, id)
		if c.disabled {
			semantic.EnabledOp(false).Add(gtx.Ops)
			return w(gtx, State{})
		}
		FocusID(gtx, c, id)
		var focused bool
		if g := c.group; g != nil {
			g.member(gtx, id, c)
			focused = g.active == c && g.focus.RingVisible(gtx)
		} else {
			focused = c.ring.Visible(gtx.Focused(c))
		}
		pointer.CursorPointer.Add(gtx.Ops)
		st := State{Hovered: c.Hovered(), Pressed: c.Pressed(), Focused: focused}
		target := float32(0)
		if st.Hovered {
			target = 1
		}
		st.Hover = c.hover.To(gtx, target, d)
		return w(gtx, st)
	})
}

// Press makes w a pressable target with id: c's clicks, hover and focus,
// with hover animated at the theme's motion speed (none at Motion 0). w
// draws from the State, which is current for this frame; never read
// c.Hovered() earlier. Every kit button, row and cell is built on it; so
// can yours (add a semantic role and label inside w).
func (t *Theme) Press(gtx layout.Context, c *Clickable, id string, w func(gtx layout.Context, st State) layout.Dimensions) layout.Dimensions {
	return c.press(gtx, id, t.Dur(DurFast), w)
}

// disabledArea lays out w on an area of its own that the semantics call
// disabled, so everything inside reads as disabled.
func disabledArea(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return area(gtx, w, func() { semantic.EnabledOp(false).Add(gtx.Ops) })
}

// Disable lays out w disabled when disabled is true, and normally otherwise.
// Nothing inside gets input (clicks, hover, keys, wheel, drags), it leaves
// the Tab order, the semantics say disabled (screen readers, kittest), and it
// draws at the theme's DisabledAlpha. Wrap one control or a whole section:
//
//	th.Disable(gtx, !v.dirty, func(gtx layout.Context) layout.Dimensions { return ui.Button{ID: "save", Text: "Save"}.Layout(gtx, th, &v.save) })
func (t *Theme) Disable(gtx layout.Context, disabled bool, w layout.Widget) layout.Dimensions {
	if !disabled || !gtx.Enabled() { // already disabled: don't dim twice
		return w(gtx)
	}
	gtx = gtx.Disabled()
	defer paint.PushOpacity(gtx.Ops, t.DisabledAlpha).Pop()
	return disabledArea(gtx, w) // the whole subtree reads as disabled
}

// Anim is a per-control animation slot for the component drawing c (a
// switch's thumb, a radio's dot): animate it with Tween.To or ToEase.
func (c *Clickable) Anim() *Tween { return &c.anim }

// Focus gives c keyboard focus, with its ring shown (e.g. the first field
// of a dialog).
func (c *Clickable) Focus(gtx layout.Context) {
	c.ring.Show()
	gtx.Execute(key.FocusCmd{Tag: c})
}

// FocusQuiet gives c focus without showing the ring until the keyboard is
// used (returning focus to a menu's trigger after a mouse pick).
func (c *Clickable) FocusQuiet(gtx layout.Context) {
	c.Focus(gtx)
	c.ring.Hide()
}

func (c *Clickable) showRing() { c.ring.Show() }
