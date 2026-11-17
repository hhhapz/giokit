package kit

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// Window holds the state that outlives a frame for one window: the key
// dispatch tree built during the last frame and what kittest needs to see of
// it. App.Run and kittest own one; a program driving Gio's loop itself calls
// Begin before its frame function and End after it.
//
//	gtx = w.Begin(gtx)
//	frame(gtx)
//	w.End(gtx)
type Window struct {
	fr     frame
	values map[string]any
}

// frame is the per-frame dispatch tree. Nodes are KeyContexts; node 0 is the
// window. It is rebuilt each frame and read by End, after layout.
type frame struct {
	nodes   []ctxNode
	cur     int
	focus   []focusRec
	modals  []int               // nodes of open modal dialogs, outermost first
	members map[string][]string // focus group id → member ids, for kittest
	active  map[string]string   // focus group id → active member id
	scope   string              // current Scope path, escaped
}

type ctxNode struct {
	parent   int
	name     string
	handlers []actionHandler
	keymaps  []*Keymap
}

type actionHandler struct {
	action ActionID
	run    func()
}

type focusRec struct {
	tag  event.Tag
	node int
}

const frameKey = "giokit.frame"

// Begin starts a frame: it registers the window bounds (Root) and installs
// the dispatch tree in gtx, which KeyContext, OnAction and Focus use.
func (w *Window) Begin(gtx C) C {
	Root(gtx)
	f := &w.fr
	f.nodes = append(f.nodes[:0], ctxNode{parent: -1})
	f.cur = 0
	f.focus = f.focus[:0]
	f.modals = f.modals[:0]
	f.scope = ""
	clear(f.members)
	clear(f.active)
	if gtx.Values == nil {
		if w.values == nil {
			w.values = map[string]any{}
		}
		gtx.Values = w.values
	}
	gtx.Values[frameKey] = f
	return gtx
}

func frameOf(gtx C) *frame {
	f, _ := gtx.Values[frameKey].(*frame)
	return f
}

// ActionID names an action. Components declare the actions they handle with
// OnAction; the keymap binds chords to them.
type ActionID string

// Built-in actions, bound by the default keymap.
const (
	ActMoveUp   ActionID = "ui.move_up"
	ActMoveDown ActionID = "ui.move_down"
	ActFirst    ActionID = "ui.first"
	ActLast     ActionID = "ui.last"
	ActExpand   ActionID = "ui.expand"   // Right: open a tree node, or go to its first child
	ActCollapse ActionID = "ui.collapse" // Left: close a tree node, or go to its parent
	ActConfirm  ActionID = "ui.confirm"  // Enter: pick the active item
	ActCancel   ActionID = "ui.cancel"   // Escape: close the innermost popup or dialog
)

type defaultBinding struct {
	name   key.Name
	mods   key.Modifiers
	action ActionID
}

// defaultKeys is the built-in keymap. Space is deliberately absent: some
// platforms may not deliver text for a key a filter claimed, and editors
// must keep typing spaces.
var defaultKeys = []defaultBinding{
	{key.NameUpArrow, 0, ActMoveUp},
	{key.NameDownArrow, 0, ActMoveDown},
	{key.NameHome, 0, ActFirst},
	{key.NameEnd, 0, ActLast},
	{key.NameRightArrow, 0, ActExpand},
	{key.NameLeftArrow, 0, ActCollapse},
	{key.NameReturn, 0, ActConfirm},
	{key.NameEnter, 0, ActConfirm},
	{key.NameEscape, 0, ActCancel},
}

// KeyContext lays out w inside a named dispatch node. Actions and keymaps
// declared inside apply while focus is anywhere within it; nested contexts
// are asked first. Popups lay out inside their opener's context, so a menu
// opened from an editor still reaches the editor's handlers.
func KeyContext(gtx C, name string, w W) D {
	f := frameOf(gtx)
	if f == nil {
		return w(gtx)
	}
	parent := f.cur
	f.nodes = append(f.nodes, ctxNode{parent: parent, name: name})
	f.cur = len(f.nodes) - 1
	defer func() { f.cur = parent }()
	return w(gtx)
}

// OnAction handles action while focus is inside the current KeyContext (or
// anywhere, at the top level). The nearest handler to the focus wins. run is
// called after layout; the frame is redrawn afterwards.
func OnAction(gtx C, action ActionID, run func()) {
	if f := frameOf(gtx); f != nil {
		n := &f.nodes[f.cur]
		n.handlers = append(n.handlers, actionHandler{action, run})
	}
}

// markModal makes the current context a modal one: with nothing focused,
// keys resolve from the innermost modal rather than the window.
func markModal(gtx C) {
	if f := frameOf(gtx); f != nil {
		f.modals = append(f.modals, f.cur)
	}
}

// trackFocus records that tag can hold focus in the current KeyContext, so
// actions resolve from here while it is focused. Focus, Clickable and the
// text fields call it; custom focusable widgets should too.
func trackFocus(gtx C, tag event.Tag) {
	if f := frameOf(gtx); f != nil {
		f.focus = append(f.focus, focusRec{tag: tag, node: f.cur})
	}
}

// End resolves the key presses nothing claimed during the frame against the
// dispatch tree: from the node holding focus up to the window, the nearest
// attached keymap binding or action handler runs. Widgets that read keys
// themselves during layout (editors) asked first, so they keep theirs.
func (w *Window) End(gtx C) {
	f := &w.fr
	var filters []event.Filter
	for _, b := range defaultKeys {
		filters = append(filters, key.Filter{Name: b.name, Required: b.mods})
	}
	for _, n := range f.nodes {
		for _, km := range n.keymaps {
			for _, b := range km.binds {
				filters = append(filters, key.Filter{Name: b.name, Required: b.mods})
			}
		}
	}
	for {
		e, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		if ke, ok := e.(key.Event); ok && ke.State == key.Press && f.dispatch(gtx, ke) {
			Notify(gtx)
		}
	}
}

func (f *frame) dispatch(gtx C, ke key.Event) bool {
	node := -1
	var focused *Focus
	for _, r := range f.focus {
		if gtx.Focused(r.tag) {
			node = r.node
			focused, _ = r.tag.(*Focus)
			break
		}
	}
	if node < 0 {
		node = 0
		if n := len(f.modals); n > 0 {
			node = f.modals[n-1]
		}
	}
	var action ActionID
	for _, b := range defaultKeys {
		if b.name == ke.Name && b.mods == ke.Modifiers {
			action = b.action
		}
	}
	for ; node >= 0; node = f.nodes[node].parent {
		n := &f.nodes[node]
		for _, km := range n.keymaps {
			for _, b := range km.binds {
				if b.name == ke.Name && b.mods == ke.Modifiers {
					b.Run()
					return true
				}
			}
		}
		if action == "" {
			continue
		}
		for _, h := range n.handlers {
			if h.action == action {
				h.run()
				if focused != nil {
					focused.quiet = false // the keyboard is in use: show the ring
				}
				return true
			}
		}
	}
	return false
}

// GroupMembers maps each focus group's id to its members' ids, as laid out
// in the last frame. kittest uses it to count a group as one Tab stop.
func (w *Window) GroupMembers() map[string][]string { return w.fr.members }

// GroupActive maps each focus group's id to its active member's id, when
// that member was laid out in the last frame. kittest reports it as the
// focused id, as a screen reader would.
func (w *Window) GroupActive() map[string]string { return w.fr.active }

// Focus makes an area keyboard-focusable: a Tab stop that can hold focus and
// receive actions. Anything can own one (a canvas, a terminal, a list).
type Focus struct {
	quiet bool
}

// Layout lays out w as the focusable area and records it in the current
// KeyContext, so actions resolve from here when it is focused.
func (f *Focus) Layout(gtx C, w W) D {
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
func (f *Focus) ask(gtx C) {
	for {
		e, ok := gtx.Event(key.FocusFilter{Target: f})
		if !ok {
			return
		}
		if fe, ok := e.(key.FocusEvent); ok && !fe.Focus {
			f.quiet = false
		}
	}
}

// track is ask plus recording f in the current KeyContext. Callers that
// place the area themselves (popups) then add event.Op(gtx.Ops, f).
func (f *Focus) track(gtx C) {
	f.ask(gtx)
	trackFocus(gtx, f)
}

// Focused reports keyboard focus with a visible ring: false when the focus
// came from a click (RequestQuiet).
func (f *Focus) Focused(gtx C) bool { return gtx.Focused(f) && !f.quiet }

// Has reports focus, ring or not.
func (f *Focus) Has(gtx C) bool { return gtx.Focused(f) }

// Request gives f keyboard focus.
func (f *Focus) Request(gtx C) { f.quiet = false; gtx.Execute(key.FocusCmd{Tag: f}) }

// RequestQuiet gives f focus without a ring until the keyboard is used.
// Pointer clicks use it: like CSS :focus-visible, a click hides the ring
// even when f already had keyboard focus.
func (f *Focus) RequestQuiet(gtx C) {
	if !gtx.Focused(f) {
		gtx.Execute(key.FocusCmd{Tag: f})
	}
	f.quiet = true
}

// FocusGroup makes a composite (list, tree, table, menu, toolbar) one Tab
// stop with an active item, which arrow keys move (WAI-ARIA "active
// descendant"). The group's container holds Gio focus, so the active item
// may be scrolled out of view and not laid out.
type FocusGroup struct {
	Active string // id of the active item
	focus  Focus
	id     string
	active *Clickable
}

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
func (g *FocusGroup) Layout(gtx C, id string, w W) D {
	g.id = Path(gtx, id)
	defer func() { g.active = nil }() // Join sets it again next frame
	return g.focus.Layout(gtx, w)
}

// Focused reports keyboard focus on the group (ring visible).
func (g *FocusGroup) Focused(gtx C) bool { return g.focus.Focused(gtx) }

// Has reports focus on the group, ring or not.
func (g *FocusGroup) Has(gtx C) bool { return g.focus.Has(gtx) }

// Focus gives the group keyboard focus.
func (g *FocusGroup) Focus(gtx C) { g.focus.Request(gtx) }

// FocusHandle is the group's focus: a popup focuses it on open
// (Popup.FocusOnOpen), and keys the group leaves unclaimed are read with
// it as the key.Filter's Focus.
func (g *FocusGroup) FocusHandle() *Focus { return &g.focus }

// FocusQuiet gives the group focus without a ring until the keyboard is used.
func (g *FocusGroup) FocusQuiet(gtx C) { g.focus.RequestQuiet(gtx) }

// member records id as laid out in the group this frame.
func (g *FocusGroup) member(gtx C, id string, c *Clickable) {
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
