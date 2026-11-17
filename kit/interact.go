package kit

import (
	"image"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// Notify schedules a redraw (GPUI: cx.notify). Call it after mutating state
// mid-frame so everything drawn earlier in the frame catches up immediately.
func Notify(gtx C) { gtx.Execute(op.InvalidateCmd{}) }

// Clickable is a pressable area that notifies whenever it reports a click.
// It is keyboard-focusable: Tab reaches it, Enter/Space click it. Inside a
// FocusGroup it is not a Tab stop of its own; the group is.
//
// kit owns this rather than wrapping widget.Clickable, because Gio puts every
// tag that asks for focus events into the Tab order and widget.Clickable
// always asks.
type Clickable struct {
	// FocusOnClick focuses the clickable when clicked, without showing the
	// focus ring until the keyboard is used (list items: so arrow keys work
	// right after a click).
	FocusOnClick bool
	click        gesture.Click
	requests     int
	pressedKey   key.Name
	hover        Tween
	anim         Tween       // free for the component drawing it (switch thumb, radio dot)
	quiet        bool        // focused by a click: hide the ring
	disabled     bool        // laid out disabled last frame (Theme.Disable)
	group        *FocusGroup // member of a group: no Tab stop, no Enter/Space
}

// FocusTag is the tag Gio routes this clickable's focus and key events to.
func (c *Clickable) FocusTag() event.Tag { return c }

// Click queues a programmatic click, reported by the next Clicked.
func (c *Clickable) Click() { c.requests++ }

// Hovered reports whether a pointer is over the clickable.
func (c *Clickable) Hovered() bool { return c.click.Hovered() }

// Pressed reports whether a pointer is pressing the clickable.
func (c *Clickable) Pressed() bool { return c.click.Pressed() }

func (c *Clickable) update(gtx C) bool {
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

// Layout lays out w as the clickable's area, dropping clicks nobody read.
func (c *Clickable) Layout(gtx C, w W) D {
	for c.update(gtx) {
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

// Clicked reports a click since the last call. Call it in your handle pass,
// before the widget is laid out: Layout consumes and drops pending clicks.
//
// A clickable laid out disabled (Theme.Disable) reports nothing and doesn't
// even ask for events: asking is what makes Gio deliver clicks and put it
// in the Tab order.
func (c *Clickable) Clicked(gtx C) bool {
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
			c.Focus(gtx)
			c.quiet = true
		}
		Notify(gtx)
		return true
	}
	return false
}

// State is the live interaction state handed to a component's draw callback.
// It is all false inside Theme.Disable (which also dims the drawing).
type State struct {
	Hovered, Pressed bool
	Focused          bool    // keyboard focus (clicks don't focus): draw a ring
	Hover            float32 // Hovered, animated 0..1: lerp tints with it
}

// Press lays out w as a clickable area with a pointer cursor and semantic id.
// The callback's State is current for this frame; never read Hovered() earlier.
func (c *Clickable) Press(gtx C, id string, w func(gtx C, st State) D) D {
	return c.press(gtx, id, DurFast, w)
}

func (c *Clickable) press(gtx C, id string, d time.Duration, w func(gtx C, st State) D) D {
	c.disabled = !gtx.Enabled()
	if c.disabled {
		return c.Layout(gtx, func(gtx C) D { return disabledArea(gtx, id, func(gtx C) D { return w(gtx, State{}) }) })
	}
	return c.Layout(gtx, func(gtx C) D {
		Tag(gtx, id)
		var focused bool
		if g := c.group; g != nil {
			g.member(gtx, id, c)
			focused = g.active == c && g.focus.Focused(gtx)
		} else {
			focused = gtx.Focused(c)
			if !focused {
				c.quiet = false
			}
		}
		pointer.CursorPointer.Add(gtx.Ops)
		st := State{Hovered: c.Hovered(), Pressed: c.Pressed(), Focused: focused && !c.quiet}
		target := float32(0)
		if st.Hovered {
			target = 1
		}
		st.Hover = c.hover.To(gtx, target, d)
		return w(gtx, st)
	})
}

// Press makes w a pressable target with id: c's clicks, hover and focus,
// with hover animated at the theme's motion speed. w draws from the State.
// Every kit button, row and cell is built on it; so can yours (add a
// semantic role and label inside w).
func (t *Theme) Press(gtx C, c *Clickable, id string, w func(gtx C, st State) D) D {
	return c.press(gtx, id, t.Dur(DurFast), w)
}

// disabledArea lays out w with id and a disabled state on a nested area of
// its own. A disabled handler asks for no gestures, and Gio drops semantics
// from such areas: the id must not sit on the handler's area.
func disabledArea(gtx C, id string, w W) D {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	Tag(gtx, id)
	semantic.EnabledOp(false).Add(gtx.Ops)
	call.Add(gtx.Ops)
	return dims
}

// Disable lays out w disabled when disabled is true, and normally otherwise.
// Nothing inside gets input (clicks, hover, keys, wheel, drags), it leaves
// the Tab order, the semantics say disabled (screen readers, kittest), and it
// draws at the theme's DisabledAlpha. Wrap one control or a whole section:
//
//	th.Disable(gtx, !v.dirty, func(gtx C) D { return ui.Button{ID: "save", Text: "Save"}.Layout(gtx, th, &v.save) })
func (t *Theme) Disable(gtx C, disabled bool, w W) D {
	if !disabled || !gtx.Enabled() { // already disabled: don't dim twice
		return w(gtx)
	}
	gtx = gtx.Disabled()
	defer paint.PushOpacity(gtx.Ops, t.DisabledAlpha).Pop()
	return disabledArea(gtx, "", w) // the whole subtree reads as disabled
}

// Anim is a per-control animation slot for the component drawing c (a
// switch's thumb, a radio's dot): animate it with Tween.To or ToEase.
func (c *Clickable) Anim() *Tween { return &c.anim }

// Focus gives c keyboard focus (e.g. the first field of a dialog).
func (c *Clickable) Focus(gtx C) { c.quiet = false; gtx.Execute(key.FocusCmd{Tag: c}) }

// FocusQuiet gives c focus without showing the ring until the keyboard is
// used (returning focus to a menu's trigger after a mouse pick).
func (c *Clickable) FocusQuiet(gtx C) { c.Focus(gtx); c.quiet = true }

// Hover tracks whether the pointer is over an area, including its children.
type Hover struct{ g gesture.Hover }

func (h *Hover) Layout(gtx C, id string, w func(gtx C, hovered bool) D) D {
	hovered := h.g.Update(gtx.Source) // consume this frame's pointer events first
	m := op.Record(gtx.Ops)
	dims := w(gtx, hovered)
	call := m.Stop()
	area := clip.Rect(image.Rectangle{Max: dims.Size})
	defer area.Push(gtx.Ops).Pop()
	h.g.Add(gtx.Ops)
	// Gio drops semantics from areas whose handler has no click/scroll gesture,
	// so the id goes on a nested, handler-free area of the same size.
	defer area.Push(gtx.Ops).Pop()
	Tag(gtx, id)
	call.Add(gtx.Ops)
	return dims
}

// Tag attaches a semantic description to the current clip area: the stable id
// tests find widgets by, and what screen readers announce. Use "kind:recordID".
func Tag(gtx C, id string) {
	if id != "" {
		semantic.DescriptionOp(Path(gtx, id)).Add(gtx.Ops)
	}
}

// TagArea lays out w and gives its whole area the id, on a nested area of
// its own. Use it to name a region whose own handler takes no pointer
// gestures (a FocusGroup, a keyboard-only canvas): Gio drops semantics from
// such an area, so Tag on it would be lost.
func TagArea(gtx C, id string, w W) D {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	Tag(gtx, id)
	call.Add(gtx.Ops)
	return dims
}

// TagLabel gives an addressed control a human-readable semantic label. The id
// remains stable for automation; the label can change with localization or
// state. Use this for icon-only controls and custom-drawn controls.
func TagLabel(gtx C, id, label string) {
	Tag(gtx, id)
	if label != "" {
		semantic.LabelOp(label).Add(gtx.Ops)
	}
}

// Pool keeps per-item widget state keyed by a stable id (never a list index).
//
// Entries live until Delete, or until Sweep finds them untouched. Components
// whose data can grow without bound (logs, file trees, chat) call Sweep once
// per frame after calling Get for every live id; ids that left the data are
// then dropped after poolIdle frames instead of leaking.
type Pool[T any] struct {
	m     map[string]*T
	stamp map[string]uint32
	gen   uint32
}

// poolIdle is how many Sweep calls an entry may go untouched before it is
// dropped. The scan also runs only every poolIdle calls, so Sweep is O(1)
// amortized per frame.
const poolIdle = 64

func (p *Pool[T]) Get(id string) *T {
	if p.m == nil {
		p.m = map[string]*T{}
		p.stamp = map[string]uint32{}
	}
	v := p.m[id]
	if v == nil {
		v = new(T)
		p.m[id] = v
	}
	p.stamp[id] = p.gen
	return v
}

// Delete drops the state for id.
func (p *Pool[T]) Delete(id string) {
	delete(p.m, id)
	delete(p.stamp, id)
}

// Len is the number of ids with state.
func (p *Pool[T]) Len() int { return len(p.m) }

// Sweep drops entries that no Get touched during the last poolIdle sweeps.
// Only call it from code that calls Get for every live id each frame;
// otherwise state for live but unvisited ids (say, rows scrolled out of view)
// is lost.
func (p *Pool[T]) Sweep() {
	p.gen++
	if p.gen%poolIdle != 0 {
		return
	}
	for id, at := range p.stamp {
		if p.gen-at > poolIdle {
			p.Delete(id)
		}
	}
}

func (p *Pool[T]) Each(f func(id string, v *T)) {
	for k, v := range p.m {
		f(k, v)
	}
}
