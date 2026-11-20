// SPDX-License-Identifier: Unlicense OR MIT

package key

import (
	"strings"

	"github.com/hhhapz/giokit/gio/internal/keylayout"
)

// Code identifies a physical key in the typing area, whatever the keyboard
// layout types with it. Codes are named as in the web's KeyboardEvent.code,
// after the key's place on a US QWERTY keyboard: "KeyQ" is the key left of
// "KeyW" on every layout, typing Q on QWERTY, A on AZERTY and ' on Dvorak.
// There are codes for the letters ("KeyA"-"KeyZ"), the digits
// ("Digit0"-"Digit9"), "Backquote", "Minus", "Equal", "BracketLeft",
// "BracketRight", "Backslash", "Semicolon", "Quote", "Comma", "Period",
// "Slash", "IntlBackslash" and "Space".
//
// Bind by Code for keys chosen for where they sit, such as a game's WASD,
// and by Name for keys chosen for what they say, such as Ctrl+S. Platforms
// that don't report physical keys leave Event.Code empty; a [Filter] with a
// Code then matches the key whose Name it has on a US layout.
type Code string

// Name returns the key name the current keyboard layout types with the key,
// for showing it: "A" for KeyQ on AZERTY. Until the layout is known it is the
// US layout's name, and it is empty for an unknown code.
func (c Code) Name() Name { return Name(keylayout.Name(string(c))) }

// Valid reports whether c is one of the codes listed for [Code].
func (c Code) Valid() bool {
	_, ok := keylayout.US[string(c)]
	return ok
}

// PhysicalCode returns e's physical key: its Code, or, from platforms that
// don't report one, the key its Name has on a US layout.
func (e Event) PhysicalCode() Code {
	if e.Code != "" {
		return e.Code
	}
	return Code(keylayout.CodeFor(string(e.Name)))
}

// ParseCode returns the code s names, ignoring case ("keyq" is KeyQ).
func ParseCode(s string) (Code, bool) {
	if Code(s).Valid() {
		return Code(s), true
	}
	for c := range keylayout.US {
		if strings.EqualFold(c, s) {
			return Code(c), true
		}
	}
	return "", false
}
