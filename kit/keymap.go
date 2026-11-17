package kit

import (
	"fmt"
	"runtime"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
)

// Keymap binds key chords to functions. Bind once, Attach it every frame
// where it applies, and list Bindings to build menus, a help screen or a
// command palette.
//
//	km.MustBind("Shortcut+1", "Go to Overview", func() { v.page = "overview" })
//	func (v *view) Frame(gtx kit.C) { v.keys.Attach(gtx); v.handle(gtx); v.layout(gtx) }
//
// Attached at the top level a keymap is window-wide; attached inside a
// KeyContext it applies only while focus is within that context, and wins
// over keymaps further out. Keys bubble like DOM events: the focused widget
// reads its keys while it is laid out, so it gets first pick (a focused text
// field keeps Ctrl+A); keys nobody claimed reach the keymaps after layout.
//
// Chords: modifiers joined with "+", then a key: "Ctrl+K", "Shift+Ctrl+P",
// "Alt+Up", "F1", "Escape", "Shortcut+S". Shortcut is Cmd on macOS and Ctrl
// elsewhere.
type Keymap struct {
	binds []Binding
}

// Binding is one chord → action.
type Binding struct {
	Chord string // as written in Bind, normalized ("Ctrl+K")
	Title string // human description, for menus / palettes / help
	Run   func()

	name key.Name
	mods key.Modifiers
}

var modNames = map[string]key.Modifiers{
	"ctrl": key.ModCtrl, "control": key.ModCtrl, "shift": key.ModShift, "alt": key.ModAlt, "option": key.ModAlt,
	"cmd": key.ModCommand, "command": key.ModCommand, "super": key.ModSuper, "meta": key.ModSuper,
	"shortcut": key.ModShortcut, "mod": key.ModShortcut,
}

var keyNames = map[string]key.Name{
	"esc": key.NameEscape, "escape": key.NameEscape, "enter": key.NameReturn, "return": key.NameReturn,
	"tab": key.NameTab, "space": key.NameSpace, "backspace": key.NameDeleteBackward, "delete": key.NameDeleteForward,
	"up": key.NameUpArrow, "down": key.NameDownArrow, "left": key.NameLeftArrow, "right": key.NameRightArrow,
	"home": key.NameHome, "end": key.NameEnd, "pageup": key.NamePageUp, "pagedown": key.NamePageDown,
	",": ",", ".": ".", "/": "/", "-": "-", "=": "=", "[": "[", "]": "]", ";": ";", "'": "'", "`": "`",
}

// ParseChord parses "Ctrl+Shift+P" into Gio's key name and modifiers.
func ParseChord(chord string) (key.Name, key.Modifiers, error) {
	parts := strings.Split(chord, "+")
	if strings.HasSuffix(chord, "++") { // "Ctrl++"
		parts = append(parts[:len(parts)-2], "+")
	}
	var mods key.Modifiers
	for _, p := range parts[:len(parts)-1] {
		m, ok := modNames[strings.ToLower(strings.TrimSpace(p))]
		if !ok {
			return "", 0, fmt.Errorf("kit: chord %q: unknown modifier %q", chord, p)
		}
		mods |= m
	}
	k := strings.TrimSpace(parts[len(parts)-1])
	if k == "" {
		return "", 0, fmt.Errorf("kit: chord %q: missing key", chord)
	}
	if n, ok := keyNames[strings.ToLower(k)]; ok {
		return n, mods, nil
	}
	if len(k) >= 2 && (k[0] == 'F' || k[0] == 'f') && strings.Trim(k[1:], "0123456789") == "" {
		return key.Name("F" + k[1:]), mods, nil
	}
	if len([]rune(k)) == 1 {
		return key.Name(strings.ToUpper(k)), mods, nil
	}
	return "", 0, fmt.Errorf("kit: chord %q: unknown key %q", chord, k)
}

// Bind adds a binding. Rebinding a chord replaces its action.
func (k *Keymap) Bind(chord, title string, run func()) error {
	n, m, err := ParseChord(chord)
	if err != nil {
		return err
	}
	b := Binding{Chord: FormatChord(n, m), Title: title, Run: run, name: n, mods: m}
	for i := range k.binds {
		if k.binds[i].name == n && k.binds[i].mods == m {
			k.binds[i] = b
			return nil
		}
	}
	k.binds = append(k.binds, b)
	return nil
}

// MustBind is Bind for chords written in code; it panics on a typo.
func (k *Keymap) MustBind(chord, title string, run func()) {
	if err := k.Bind(chord, title, run); err != nil {
		panic(err)
	}
}

// Bindings lists every binding in the order bound.
func (k *Keymap) Bindings() []Binding { return k.binds }

// Hint returns the display chord bound to the action with this title ("" if
// none), for showing beside a button or menu item: ui.Kbd{Chord: km.Hint("Save")}.Layout(gtx, th).
func (k *Keymap) Hint(title string) string {
	for _, b := range k.binds {
		if b.Title == title {
			return b.Chord
		}
	}
	return ""
}

// Attach makes k's bindings live for this frame in the current KeyContext
// (window-wide at the top level). Call it anywhere in the frame: keys are
// resolved after layout, from the focused widget outward. Without a
// kit.Window (plain Gio), it reads unclaimed keys directly, so call it last.
func (k *Keymap) Attach(gtx C) {
	if f := frameOf(gtx); f != nil {
		n := &f.nodes[f.cur]
		n.keymaps = append(n.keymaps, k)
		return
	}
	if len(k.binds) == 0 {
		return
	}
	filters := make([]event.Filter, len(k.binds))
	for i, b := range k.binds {
		filters[i] = key.Filter{Name: b.name, Required: b.mods}
	}
	for {
		e, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		ke, isKey := e.(key.Event)
		if !isKey || ke.State != key.Press {
			continue
		}
		for _, b := range k.binds {
			if b.name == ke.Name && b.mods == ke.Modifiers {
				b.Run()
				Notify(gtx)
				break
			}
		}
	}
}

// FormatChord renders a chord for display, with platform names
// (Shortcut → Cmd on macOS, Ctrl elsewhere).
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
	}
	return strings.Join(append(parts, name), "+")
}
