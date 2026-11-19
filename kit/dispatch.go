package kit

import (
	"image"
	"time"

	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/io/pointer"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
)

// Window holds the state that outlives a frame for one window: the key
// dispatch tree built during the last frame and what kittest needs to see of
// it. App.Run and kittest own one; a program driving Gio's loop itself calls
// Begin before its frame function and End after it.
//
//	gtx = w.Begin(gtx)
//	frame(gtx)
//	w.End(gtx)
//
// The zero value is ready to use. A Window belongs to the goroutine that
// runs its window's frames; one Window serves one window.
type Window struct {
	fr     frame
	values map[string]any
}

// frame is the per-frame dispatch tree. Nodes are KeyContexts; node 0 is the
// window. It is rebuilt each frame and read by End, after layout.
type frame struct {
	nodes   []ctxNode
	cur     int
	focus   map[event.Tag]int   // focusable tag → its KeyContext node (FocusID)
	modals  []int               // nodes of open modal dialogs, outermost first
	members map[string][]string // focus group id → member ids, for kittest
	active  map[string]string   // focus group id → active member id
	scope   string              // current Scope path, escaped

	size     image.Point          // the window, in pixels (Begin's constraints)
	pointers []pointerSeen        // this frame's pointer events at the root, window coordinates
	root     clip.Stack           // the window area, pushed for the whole frame (Begin to End)
	ids      map[event.Tag]string // focusable tag → id path, for FocusedID
	roles    map[string]Role      // id path → role (SetRole)
	// typing is set when a text editor laid out this frame holds focus:
	// keymaps then leave plain printable keys to it (see Keymap).
	typing bool
}

// pointerSeen is a pointer event as the root saw it: popups compare it with
// the same event seen by their anchor to learn where the anchor is.
type pointerSeen struct {
	at  time.Duration
	pos image.Point
}

// ctxNode is one KeyContext: what it handles and the keymaps attached in it.
type ctxNode struct {
	parent   int
	name     string
	handlers []actionHandler
	keymaps  []*Keymap
}

// actionHandler is one OnAction registration.
type actionHandler struct {
	action ActionID
	run    func()
}

const frameKey = "giokit.frame"

// Begin starts a frame: it registers the window bounds (Root) and installs
// the dispatch tree in gtx, which KeyContext, OnAction and Focus use. It
// resets what the last frame recorded (roles, focus groups, focus ids), so
// maps the window returned earlier are reused from here on. Every Begin
// needs a matching End on the same gtx.
func (w *Window) Begin(gtx layout.Context) layout.Context {
	f := &w.fr
	f.size = gtx.Constraints.Max
	f.pointers = f.pointers[:0]
	for {
		e, ok := gtx.Event(pointer.Filter{Target: rootTag, Kinds: pointerKinds})
		if !ok {
			break
		}
		if pe, ok := e.(pointer.Event); ok {
			f.pointers = append(f.pointers, pointerSeen{pe.Time, pe.Position.Round()})
		}
	}
	// The window area stays pushed until End, so it encloses every area of
	// the frame and sees every pointer event (Root alone would be a sibling
	// underneath the page).
	f.root = clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, rootTag)
	f.nodes = append(f.nodes[:0], ctxNode{parent: -1})
	f.cur = 0
	clear(f.focus)
	f.modals = f.modals[:0]
	f.scope = ""
	clear(f.members)
	clear(f.active)
	clear(f.ids)
	clear(f.roles)
	f.typing = false
	if gtx.Values == nil {
		if w.values == nil {
			w.values = map[string]any{}
		}
		gtx.Values = w.values
	}
	gtx.Values[frameKey] = f
	return gtx
}

func frameOf(gtx layout.Context) *frame {
	f, _ := gtx.Values[frameKey].(*frame)
	return f
}

// ActionID names a key action: something the focused control does
// (move, expand, confirm), resolved through the KeyContexts around the
// focus. Components declare the actions they handle with OnAction, and the
// default keymap binds chords to them.
//
// It is distinct from Action, a named application command listed in menus
// and palettes and bound to chords with Keymap.BindActions.
type ActionID string

// Built-in actions, bound by the default keymap.
const (
	ActMoveUp     ActionID = "ui.move_up"
	ActMoveDown   ActionID = "ui.move_down"
	ActFirst      ActionID = "ui.first"
	ActLast       ActionID = "ui.last"
	ActExpand     ActionID = "ui.expand"      // Right: open a tree node, or go to its first child
	ActCollapse   ActionID = "ui.collapse"    // Left: close a tree node, or go to its parent
	ActConfirm    ActionID = "ui.confirm"     // Enter: pick the active item
	ActCancel     ActionID = "ui.cancel"      // Escape: close the innermost popup or dialog
	ActExtendUp   ActionID = "ui.extend_up"   // Shift+Up: grow a multiple selection upward
	ActExtendDown ActionID = "ui.extend_down" // Shift+Down: grow it downward
	ActSelectAll  ActionID = "ui.select_all"  // Ctrl/Cmd+A: select every item
	// ActContextMenu is Shift+F10: open the context menu of the focused
	// control (the keyboard's right click).
	ActContextMenu ActionID = "ui.context_menu"
)

// defaultBinding binds a chord to a built-in action.
type defaultBinding struct {
	keys   chord
	action ActionID
}

// defaultKeys is the built-in keymap. Space is deliberately absent: some
// platforms may not deliver text for a key a filter claimed, and editors
// must keep typing spaces.
var defaultKeys = []defaultBinding{
	{keys: chord{name: key.NameUpArrow}, action: ActMoveUp},
	{keys: chord{name: key.NameDownArrow}, action: ActMoveDown},
	{keys: chord{name: key.NameHome}, action: ActFirst},
	{keys: chord{name: key.NameEnd}, action: ActLast},
	{keys: chord{name: key.NameRightArrow}, action: ActExpand},
	{keys: chord{name: key.NameLeftArrow}, action: ActCollapse},
	{keys: chord{name: key.NameReturn}, action: ActConfirm},
	{keys: chord{name: key.NameEnter}, action: ActConfirm},
	{keys: chord{name: key.NameEscape}, action: ActCancel},
	{keys: chord{name: key.NameUpArrow, mods: key.ModShift}, action: ActExtendUp},
	{keys: chord{name: key.NameDownArrow, mods: key.ModShift}, action: ActExtendDown},
	{keys: chord{name: "A", mods: key.ModShortcut}, action: ActSelectAll},
	{keys: chord{name: key.NameF10, mods: key.ModShift}, action: ActContextMenu},
}

// KeyContext lays out w inside a named dispatch node. Actions and keymaps
// declared inside apply while focus is anywhere within it; nested contexts
// are asked first. Popups lay out inside their opener's context, so a menu
// opened from an editor still reaches the editor's handlers.
func KeyContext(gtx layout.Context, name string, w layout.Widget) layout.Dimensions {
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
func OnAction(gtx layout.Context, action ActionID, run func()) {
	if f := frameOf(gtx); f != nil {
		n := &f.nodes[f.cur]
		n.handlers = append(n.handlers, actionHandler{action, run})
	}
}

// markModal makes the current context a modal one: with nothing focused,
// keys resolve from the innermost modal rather than the window.
func markModal(gtx layout.Context) {
	if f := frameOf(gtx); f != nil {
		f.modals = append(f.modals, f.cur)
	}
}

// Notify schedules a redraw (GPUI: cx.notify). Call it after mutating state
// mid-frame so everything drawn earlier in the frame catches up immediately.
func Notify(gtx layout.Context) { gtx.Execute(op.InvalidateCmd{}) }

// End resolves the key presses nothing claimed during the frame against the
// dispatch tree: from the node holding focus up to the window, the nearest
// attached keymap binding or action handler runs. Widgets that read keys
// themselves during layout (editors) asked first, so they keep theirs.
func (w *Window) End(gtx layout.Context) {
	f := &w.fr
	f.root.Pop()
	var filters []event.Filter
	for _, b := range defaultKeys {
		filters = append(filters, b.keys.filter())
	}
	for _, n := range f.nodes {
		for _, km := range n.keymaps {
			filters = km.appendFilters(filters, f.typing)
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

// dispatch runs the binding or action handler for ke nearest the focus,
// and reports whether one ran. A binding whose action is disabled is
// skipped, so the key keeps bubbling outward.
func (f *frame) dispatch(gtx layout.Context, ke key.Event) bool {
	node := -1
	var focused event.Tag
	for tag, n := range f.focus {
		if gtx.Focused(tag) {
			node, focused = n, tag
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
		if b.keys.matches(ke) {
			action = b.action
		}
	}
	for ; node >= 0; node = f.nodes[node].parent {
		n := &f.nodes[node]
		for _, km := range n.keymaps {
			if f.typing && ke.Modifiers&^key.ModShift == 0 && editorKey(ke.Name) {
				break // text for the focused editor, never a shortcut
			}
			if b, ok := km.match(ke); ok {
				b.Run()
				return true
			}
		}
		if action == "" {
			continue
		}
		for _, h := range n.handlers {
			if h.action == action {
				h.run()
				if r, ok := focused.(ringShower); ok {
					r.showRing() // the keyboard is in use: show the ring
				}
				return true
			}
		}
	}
	return false
}
