package kit

import (
	"image"

	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/input"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
)

// FocusID declares that the focusable tag is the control id this frame. It
// does two things, and every focusable needs both:
//
//   - it names what holds focus, for kittest's FocusedID and inspectors;
//   - it records tag in the current KeyContext, so while tag is focused,
//     keys it leaves unclaimed resolve against the actions and keymaps of
//     the contexts enclosing it.
//
// kit's components call it. A custom focusable (a key.FocusFilter on its
// own tag: a slider, a split handle) calls it beside its Tag, inside the
// KeyContext whose actions should apply. If tag is registered more than
// once in a frame, the first registration wins: its id and its KeyContext.
func FocusID(gtx layout.Context, tag event.Tag, id string) {
	f := frameOf(gtx)
	if f == nil {
		return
	}
	if f.ids == nil {
		f.ids = map[event.Tag]string{}
	}
	if _, ok := f.ids[tag]; !ok {
		f.ids[tag] = Path(gtx, id)
	}
	f.track(tag)
}

// trackFocus records that tag can hold focus in the current KeyContext,
// without naming it: for focusables whose id is declared elsewhere, or
// in another context (a dialog tracks its focus inside its own context).
func trackFocus(gtx layout.Context, tag event.Tag) {
	if f := frameOf(gtx); f != nil {
		f.track(tag)
	}
}

// track records tag in the current node unless it is already recorded.
func (f *frame) track(tag event.Tag) {
	if _, ok := f.focus[tag]; ok {
		return
	}
	if f.focus == nil {
		f.focus = map[event.Tag]int{}
	}
	f.focus[tag] = f.cur
}

// FocusWithin lays out w inside a KeyContext of its own and reports
// whether something w recorded as focusable (with FocusID, a Focus or a
// Clickable) holds keyboard focus: CSS :focus-within. A tooltip uses it to
// show while its control is focused. Actions w declares at its top level
// apply while that focus is inside.
func FocusWithin(gtx layout.Context, w layout.Widget) (layout.Dimensions, bool) {
	f := frameOf(gtx)
	if f == nil {
		return w(gtx), false
	}
	node := -1
	dims := KeyContext(gtx, "focus-within", func(gtx layout.Context) layout.Dimensions {
		node = f.cur
		return w(gtx)
	})
	for tag, n := range f.focus {
		if f.inside(n, node) && gtx.Focused(tag) {
			return dims, true
		}
	}
	return dims, false
}

// inside reports whether node n is ancestor or one of its descendants.
func (f *frame) inside(n, ancestor int) bool {
	for ; n > 0; n = f.nodes[n].parent {
		if n == ancestor {
			return true
		}
	}
	return n == ancestor
}

// FocusedID is the id of the control holding keyboard focus in the last
// frame (as recorded by FocusID), or "". src is the window's input source.
func (w *Window) FocusedID(src input.Source) string {
	for tag, id := range w.fr.ids {
		if src.Focused(tag) {
			return id
		}
	}
	return ""
}

// GroupMembers maps each focus group's id to its members' ids, in layout
// order, as laid out in the last frame. kittest uses it to count a group as
// one Tab stop. The map and its slices are borrowed: they belong to the
// window and are reused by the next frame's Begin, so copy what you keep.
func (w *Window) GroupMembers() map[string][]string { return w.fr.members }

// GroupActive maps each focus group's id to its active member's id, when
// that member was laid out in the last frame. kittest reports it as the
// focused id, as a screen reader would. The map is borrowed, like
// GroupMembers'.
func (w *Window) GroupActive() map[string]string { return w.fr.active }

// FocusVisible decides whether a focused control draws its focus ring, as
// CSS :focus-visible does: focus given by a pointer hides the ring until
// the keyboard is used, and focus given by the keyboard shows it. The zero
// value shows the ring. Focus and Clickable use one; a custom focusable
// keeps one beside its tag.
type FocusVisible struct {
	hidden bool
}

// Hide hides the ring until Show, or until the control loses focus. Call
// it when a pointer gives the control focus.
func (v *FocusVisible) Hide() { v.hidden = true }

// Show shows the ring again. Call it when the keyboard acts on the control
// or gives it focus.
func (v *FocusVisible) Show() { v.hidden = false }

// Visible reports whether to draw the ring, given whether the control holds
// focus. Losing focus resets the state, so the next focus starts visible.
func (v *FocusVisible) Visible(focused bool) bool {
	if !focused {
		v.hidden = false
		return false
	}
	return !v.hidden
}

// ringShower is a focusable whose ring dispatch shows when the keyboard
// runs an action on it.
type ringShower interface {
	showRing()
}

// Focus makes an area keyboard-focusable: a Tab stop that can hold focus and
// receive actions. Anything can own one (a canvas, a terminal, a list). The
// zero value is ready to use. Focus is its own event tag, so it must not be
// copied or moved after its first Layout.
type Focus struct {
	ring FocusVisible
}

// Layout lays out w as the focusable area and records it in the current
// KeyContext, so actions resolve from here when it is focused.
func (f *Focus) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	f.track(gtx)
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, f)
	call.Add(gtx.Ops)
	return dims
}

// ask consumes f's focus events, which also makes it focusable this frame.
func (f *Focus) ask(gtx layout.Context) {
	for {
		e, ok := gtx.Event(key.FocusFilter{Target: f})
		if !ok {
			return
		}
		if fe, ok := e.(key.FocusEvent); ok && !fe.Focus {
			f.ring.Show()
		}
	}
}

// track is ask plus recording f in the current KeyContext. Callers that
// place the area themselves (popups) then add event.Op(gtx.Ops, f).
func (f *Focus) track(gtx layout.Context) {
	f.ask(gtx)
	trackFocus(gtx, f)
}

func (f *Focus) showRing() { f.ring.Show() }

// RingVisible reports whether f holds keyboard focus and should draw its
// ring: false while f is unfocused, and while focus that came from a
// pointer (RequestQuiet) has not yet seen the keyboard.
func (f *Focus) RingVisible(gtx layout.Context) bool { return f.ring.Visible(gtx.Focused(f)) }

// HasFocus reports whether f holds keyboard focus, ring or not.
func (f *Focus) HasFocus(gtx layout.Context) bool { return gtx.Focused(f) }

// Request gives f keyboard focus, with its ring shown.
func (f *Focus) Request(gtx layout.Context) {
	f.ring.Show()
	gtx.Execute(key.FocusCmd{Tag: f})
}

// RequestQuiet gives f focus without a ring until the keyboard is used.
// Pointer clicks use it: like CSS :focus-visible, a click hides the ring
// even when f already had keyboard focus.
func (f *Focus) RequestQuiet(gtx layout.Context) {
	if !gtx.Focused(f) {
		gtx.Execute(key.FocusCmd{Tag: f})
	}
	f.ring.Hide()
}

// FocusGroup makes a composite (list, tree, table, menu, toolbar) one Tab
// stop with an active item, which arrow keys move (WAI-ARIA "active
// descendant"). The group's container holds Gio focus, so the active item
// may be scrolled out of view and not laid out. The zero value is ready to
// use; like Focus, a FocusGroup must not be copied after its first Layout.
type FocusGroup struct {
	Active string // id of the active item
	focus  Focus
	id     string
	active *Clickable
}

// RingVisible reports keyboard focus on the group with its ring shown (see
// Focus.RingVisible).
func (g *FocusGroup) RingVisible(gtx layout.Context) bool { return g.focus.RingVisible(gtx) }

// HasFocus reports focus on the group, ring or not.
func (g *FocusGroup) HasFocus(gtx layout.Context) bool { return g.focus.HasFocus(gtx) }

// Join makes c a member of the group for this frame: no Tab stop of its own,
// clicks focus the group. When active is true, c is the active item and
// draws the focus ring while the group is focused. Call it in the handle
// pass, before c.Clicked.
func (g *FocusGroup) Join(c *Clickable, active bool) {
	c.group = g
	if active {
		g.active = c
	} else if g.active == c {
		g.active = nil
	}
}

// Layout lays out w as the group's single focusable area. Something must
// name that area with the same id, so tests and screen readers can name the
// group and report its active item: kit's lists do through List; a custom
// group wraps its content in TagArea(gtx, id, ...).
func (g *FocusGroup) Layout(gtx layout.Context, id string, w layout.Widget) layout.Dimensions {
	g.id = Path(gtx, id)
	FocusID(gtx, &g.focus, id)
	defer func() { g.active = nil }() // Join sets it again next frame
	return g.focus.Layout(gtx, w)
}

// Focus gives the group keyboard focus.
func (g *FocusGroup) Focus(gtx layout.Context) { g.focus.Request(gtx) }

// FocusHandle is the group's focus: a popup focuses it on open
// (Popup.FocusOnOpen), and keys the group leaves unclaimed are read with
// it as the key.Filter's Focus.
func (g *FocusGroup) FocusHandle() *Focus { return &g.focus }

// FocusQuiet gives the group focus without a ring until the keyboard is used.
func (g *FocusGroup) FocusQuiet(gtx layout.Context) { g.focus.RequestQuiet(gtx) }

// member records id as laid out in the group this frame.
func (g *FocusGroup) member(gtx layout.Context, id string, c *Clickable) {
	f := frameOf(gtx)
	if f == nil || g.id == "" {
		return
	}
	if f.members == nil {
		f.members = map[string][]string{}
		f.active = map[string]string{}
	}
	p := Path(gtx, id)
	f.members[g.id] = append(f.members[g.id], p)
	if g.active == c {
		f.active[g.id] = p
	}
}
