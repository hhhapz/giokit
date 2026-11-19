package kit

import (
	"fmt"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/layout"
)

// Keymap binds key chords to functions. Bind once, Attach it every frame
// where it applies, and list Bindings to build menus, a help screen or a
// command palette. The zero value is an empty keymap.
//
//	v.keys.MustBind("Shortcut+1", "Go to Overview", func() { v.page = "overview" }) // once
//
//	func (v *view) Frame(gtx layout.Context) {
//		v.keys.Attach(gtx)
//		v.handle(gtx)
//		v.layout(gtx)
//	}
//
// Attached at the top level a keymap is window-wide; attached inside a
// KeyContext it applies only while focus is within that context, and wins
// over keymaps further out. Keys bubble like DOM events: the focused widget
// reads its keys while it is laid out, so it gets first pick (a focused text
// field keeps Ctrl+A); keys nobody claimed reach the keymaps after layout.
// While a text field (Theme.EditorField) has focus, chords that type text
// ("1", "?", "Shift+A") are left to it, so single-key shortcuts never eat
// typing.
//
// Chords: modifiers joined with "+", then a key: "Ctrl+K", "Shift+Ctrl+P",
// "Alt+Up", "F1", "Escape", "Shortcut+S", "Ctrl++". Shortcut is Cmd on macOS
// and Ctrl elsewhere.
//
// A Keymap belongs to the UI goroutine, and its bound functions run there.
type Keymap struct {
	binds []Binding
}

// Binding is one chord → function.
type Binding struct {
	// Chord is the chord in display form, as FormatChord writes it, whatever
	// spelling was passed to Bind: binding "ctrl+k" gives "Ctrl+K".
	Chord string
	Title string // human description, for menus / palettes / help
	// Action is the id of the Action this binding runs, when BindActions
	// made it; "" for bindings made with Bind.
	Action string
	Run    func() // what the chord does, on the UI goroutine

	keys    chord
	enabled func() bool // nil: always enabled
}

// live reports whether b takes its chord now. A disabled binding lets the
// key bubble on, as if it were unbound.
func (b Binding) live() bool { return b.enabled == nil || b.enabled() }

// chord is a parsed key chord: a key and exactly the modifiers held with it.
type chord struct {
	name key.Name
	mods key.Modifiers
}

// matches reports whether e is c pressed.
func (c chord) matches(e key.Event) bool { return e.Name == c.name && e.Modifiers == c.mods }

// filter asks Gio for c's key events.
func (c chord) filter() key.Filter { return key.Filter{Name: c.name, Required: c.mods} }

var modNames = map[string]key.Modifiers{
	"ctrl": key.ModCtrl, "control": key.ModCtrl, "shift": key.ModShift, "alt": key.ModAlt, "option": key.ModAlt,
	"cmd": key.ModCommand, "command": key.ModCommand, "super": key.ModSuper, "meta": key.ModSuper,
	"shortcut": key.ModShortcut, "mod": key.ModShortcut,
}

// keyNames are the named keys. Any other single character is its own key
// (",", "1", "K"), upper-cased as Gio names it.
var keyNames = map[string]key.Name{
	"esc": key.NameEscape, "escape": key.NameEscape, "enter": key.NameReturn, "return": key.NameReturn,
	"tab": key.NameTab, "space": key.NameSpace, "backspace": key.NameDeleteBackward, "delete": key.NameDeleteForward,
	"up": key.NameUpArrow, "down": key.NameDownArrow, "left": key.NameLeftArrow, "right": key.NameRightArrow,
	"home": key.NameHome, "end": key.NameEnd, "pageup": key.NamePageUp, "pagedown": key.NamePageDown,
}

// Gio names function keys F1 to F12 only; others are never delivered.
const (
	firstFunctionKey = 1
	lastFunctionKey  = 12
)

// ParseChord parses "Ctrl+Shift+P" into Gio's key name and modifiers.
// Names are case-insensitive. The plus key is written "+" ("Ctrl++"). It
// fails on an unknown modifier or key, a missing key, and function keys Gio
// does not deliver (F0, F13 and above).
func ParseChord(s string) (key.Name, key.Modifiers, error) {
	c, err := parseChord(s)
	return c.name, c.mods, err
}

func parseChord(s string) (chord, error) {
	// Split off the key: everything after the last "+", unless the key is
	// "+" itself.
	keyPart, modPart, hasMods := s, "", false
	switch {
	case s == "+":
	case strings.HasSuffix(s, "++"): // "Ctrl++"
		keyPart, modPart, hasMods = "+", s[:len(s)-2], true
	default:
		if i := strings.LastIndex(s, "+"); i >= 0 {
			keyPart, modPart, hasMods = s[i+1:], s[:i], true
		}
	}
	var c chord
	if hasMods {
		for _, p := range strings.Split(modPart, "+") {
			m, ok := modNames[strings.ToLower(strings.TrimSpace(p))]
			if !ok {
				return chord{}, fmt.Errorf("kit: chord %q: unknown modifier %q", s, p)
			}
			c.mods |= m
		}
	}
	k := strings.TrimSpace(keyPart)
	if k == "" {
		return chord{}, fmt.Errorf("kit: chord %q: missing key", s)
	}
	if n, ok := keyNames[strings.ToLower(k)]; ok {
		c.name = n
		return c, nil
	}
	if len(k) >= 2 && (k[0] == 'F' || k[0] == 'f') && strings.Trim(k[1:], "0123456789") == "" {
		n, err := strconv.Atoi(k[1:])
		if err != nil || n < firstFunctionKey || n > lastFunctionKey {
			return chord{}, fmt.Errorf("kit: chord %q: no function key %q (F%d to F%d)", s, k, firstFunctionKey, lastFunctionKey)
		}
		c.name = key.Name("F" + strconv.Itoa(n))
		return c, nil
	}
	if len([]rune(k)) == 1 {
		c.name = key.Name(strings.ToUpper(k))
		return c, nil
	}
	return chord{}, fmt.Errorf("kit: chord %q: unknown key %q", s, k)
}

// Bind binds the chord that spec writes (see Keymap) to run, under title.
// Rebinding a chord replaces its binding. It fails if spec does not parse.
func (k *Keymap) Bind(spec, title string, run func()) error {
	c, err := parseChord(spec)
	if err != nil {
		return err
	}
	k.add(Binding{Title: title, Run: run, keys: c})
	return nil
}

// add adds b, replacing the binding for the same chord.
func (k *Keymap) add(b Binding) {
	b.Chord = FormatChord(b.keys.name, b.keys.mods)
	for i := range k.binds {
		if k.binds[i].keys == b.keys {
			k.binds[i] = b
			return
		}
	}
	k.binds = append(k.binds, b)
}

// MustBind is Bind for chords written in code; it panics on a typo.
func (k *Keymap) MustBind(spec, title string, run func()) {
	if err := k.Bind(spec, title, run); err != nil {
		panic(err)
	}
}

// Bindings lists every binding in the order bound. It returns a copy:
// changing it does not change k.
func (k *Keymap) Bindings() []Binding { return slices.Clone(k.binds) }

// Hint returns the display chord of the binding with this title ("" if
// none). Titles are display strings, which may be translated or repeated;
// for bindings made by BindActions prefer HintFor.
func (k *Keymap) Hint(title string) string {
	for _, b := range k.binds {
		if b.Title == title {
			return b.Chord
		}
	}
	return ""
}

// HintFor returns the display chord bound to the action with this id by
// BindActions ("" if none), for showing beside a button or menu item:
// ui.Kbd{Chord: km.HintFor("file.save")}.Layout(gtx, th).
func (k *Keymap) HintFor(actionID string) string {
	for _, b := range k.binds {
		if b.Action != "" && b.Action == actionID {
			return b.Chord
		}
	}
	return ""
}

// appendFilters appends the key filters for k's chords to fs. While typing
// (a text editor has focus) it leaves out chords that type text, so those
// keys reach the editor as text.
func (k *Keymap) appendFilters(fs []event.Filter, typing bool) []event.Filter {
	for _, b := range k.binds {
		if typing && b.keys.types() {
			continue
		}
		fs = append(fs, b.keys.filter())
	}
	return fs
}

// types reports whether c belongs to a focused editor: a key that types
// or edits text, with no modifier but Shift ("1", "?", "Shift+A", "Space",
// "Left", "Shift+Home", "Return").
func (c chord) types() bool { return c.mods&^key.ModShift == 0 && editorKey(c.name) }

// editorKey reports whether a focused editor owns the key name when no
// modifier but Shift is held: printable characters, Space, and the keys
// that move the caret, select or delete. Escape, Tab and the function keys
// are not, so shortcuts bound to them work while typing.
func editorKey(n key.Name) bool {
	switch n {
	case key.NameSpace, key.NameLeftArrow, key.NameRightArrow, key.NameUpArrow,
		key.NameDownArrow, key.NameHome, key.NameEnd, key.NamePageUp, key.NamePageDown,
		key.NameDeleteBackward, key.NameDeleteForward, key.NameReturn, key.NameEnter:
		return true
	case key.NameEscape, key.NameCommand:
		return false
	}
	return len([]rune(string(n))) == 1
}

// match returns the enabled binding for the key press e.
func (k *Keymap) match(e key.Event) (Binding, bool) {
	for _, b := range k.binds {
		if b.keys.matches(e) && b.live() {
			return b, true
		}
	}
	return Binding{}, false
}

// Attach makes k's bindings live for this frame in the current KeyContext
// (window-wide at the top level). Call it anywhere in the frame: keys are
// resolved after layout, from the focused widget outward.
//
// Without a kit.Window (plain Gio) there is no dispatch tree, so Attach
// reads the unclaimed keys itself, at the point it is called: call it last
// in the frame.
func (k *Keymap) Attach(gtx layout.Context) {
	if f := frameOf(gtx); f != nil {
		n := &f.nodes[f.cur]
		n.keymaps = append(n.keymaps, k)
		return
	}
	if len(k.binds) == 0 {
		return
	}
	filters := k.appendFilters(nil, false)
	for {
		e, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		ke, isKey := e.(key.Event)
		if !isKey || ke.State != key.Press {
			continue
		}
		if b, ok := k.match(ke); ok {
			b.Run()
			Notify(gtx)
		}
	}
}

// FormatChord renders a chord for display, with platform names
// (Shortcut → Cmd on macOS, Ctrl elsewhere). Keys Gio names with symbols
// the bundled fonts lack (⌦, ⇱) are written as words, as ParseChord
// spells them, so the result parses back.
func FormatChord(n key.Name, m key.Modifiers) string {
	var parts []string
	mac := runtime.GOOS == "darwin" || runtime.GOOS == "ios"
	if m.Contain(key.ModCtrl) {
		parts = append(parts, "Ctrl")
	}
	if m.Contain(key.ModAlt) {
		if mac {
			parts = append(parts, "Option")
		} else {
			parts = append(parts, "Alt")
		}
	}
	if m.Contain(key.ModShift) {
		parts = append(parts, "Shift")
	}
	if m.Contain(key.ModCommand) {
		parts = append(parts, "Cmd")
	}
	if m.Contain(key.ModSuper) {
		parts = append(parts, "Super")
	}
	name := string(n)
	switch n {
	case key.NameEscape:
		name = "Esc"
	case key.NameReturn:
		name = "Enter"
	case key.NameUpArrow:
		name = "Up"
	case key.NameDownArrow:
		name = "Down"
	case key.NameLeftArrow:
		name = "Left"
	case key.NameRightArrow:
		name = "Right"
	case key.NameDeleteForward:
		name = "Delete"
	case key.NameDeleteBackward:
		name = "Backspace"
	case key.NameEnter:
		name = "Enter"
	case key.NameHome:
		name = "Home"
	case key.NameEnd:
		name = "End"
	case key.NamePageUp:
		name = "PageUp"
	case key.NamePageDown:
		name = "PageDown"
	}
	return strings.Join(append(parts, name), "+")
}
