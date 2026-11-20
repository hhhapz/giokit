// SPDX-License-Identifier: Unlicense OR MIT

// Package keylayout tracks which key name the current keyboard layout gives
// each physical key. Platform backends report whole layouts where they can;
// the input router fills in the rest from unmodified key presses. The
// io/key package reads it through key.Code.
package keylayout

import (
	"maps"
	"sync"
)

// US is the key name at each physical key on a US QWERTY layout, which is
// also how codes are named ("KeyQ" is the key QWERTY types Q with).
var US = map[string]string{
	"KeyA": "A", "KeyB": "B", "KeyC": "C", "KeyD": "D", "KeyE": "E", "KeyF": "F", "KeyG": "G",
	"KeyH": "H", "KeyI": "I", "KeyJ": "J", "KeyK": "K", "KeyL": "L", "KeyM": "M", "KeyN": "N",
	"KeyO": "O", "KeyP": "P", "KeyQ": "Q", "KeyR": "R", "KeyS": "S", "KeyT": "T", "KeyU": "U",
	"KeyV": "V", "KeyW": "W", "KeyX": "X", "KeyY": "Y", "KeyZ": "Z",
	"Digit0": "0", "Digit1": "1", "Digit2": "2", "Digit3": "3", "Digit4": "4",
	"Digit5": "5", "Digit6": "6", "Digit7": "7", "Digit8": "8", "Digit9": "9",
	"Backquote": "`", "Minus": "-", "Equal": "=", "BracketLeft": "[", "BracketRight": "]",
	"Backslash": `\`, "Semicolon": ";", "Quote": "'", "Comma": ",", "Period": ".", "Slash": "/",
	"IntlBackslash": `\`, "Space": "Space",
}

// usCode is US inverted, for naming the physical key of an event that came
// without one. IntlBackslash has no US key of its own.
var usCode = func() map[string]string {
	m := make(map[string]string, len(US))
	for c, n := range US {
		if c != "IntlBackslash" {
			m[n] = c
		}
	}
	return m
}()

var (
	mu    sync.RWMutex
	names = map[string]string{}
)

// Set records that the current layout types name with the key at code.
func Set(code, name string) {
	if _, ok := US[code]; !ok || name == "" {
		return
	}
	mu.Lock()
	names[code] = name
	mu.Unlock()
}

// Learn records what a key press shows the current layout types with code.
// Where that isn't what was thought, the layout has changed or was never
// reported, so a common layout that agrees with the press takes over every
// key: one press is enough to relabel them all.
func Learn(code, name string) {
	if _, ok := US[code]; !ok || name == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	cur, ok := names[code]
	if !ok {
		cur = US[code]
	}
	if cur != name {
		for _, l := range common {
			if l[code] == name {
				clear(names)
				maps.Copy(names, l)
				return
			}
		}
	}
	names[code] = name
}

// common are layouts Learn recognises, most likely first; each is US with
// changes. Dead keys are left out: they make no key events to learn from.
var common = []map[string]string{
	US,
	with(map[string]string{ // German QWERTZ
		"KeyY": "Z", "KeyZ": "Y", "Minus": "ß", "BracketLeft": "Ü", "BracketRight": "+",
		"Semicolon": "Ö", "Quote": "Ä", "Backslash": "#", "Slash": "-", "IntlBackslash": "<",
	}),
	with(map[string]string{ // French AZERTY
		"KeyQ": "A", "KeyW": "Z", "KeyA": "Q", "KeyZ": "W", "Semicolon": "M", "KeyM": ",",
		"Comma": ";", "Period": ":", "Slash": "!", "Quote": "Ù", "Backslash": "*", "BracketRight": "$",
		"Digit1": "&", "Digit2": "É", "Digit3": `"`, "Digit4": "'", "Digit5": "(", "Digit6": "-",
		"Digit7": "È", "Digit8": "_", "Digit9": "Ç", "Digit0": "À", "Minus": ")", "Equal": "=",
		"Backquote": "²", "IntlBackslash": "<",
	}),
	with(map[string]string{ // Dvorak
		"Minus": "[", "Equal": "]", "KeyQ": "'", "KeyW": ",", "KeyE": ".", "KeyR": "P", "KeyT": "Y",
		"KeyY": "F", "KeyU": "G", "KeyI": "C", "KeyO": "R", "KeyP": "L", "BracketLeft": "/",
		"BracketRight": "=", "KeyS": "O", "KeyD": "E", "KeyF": "U", "KeyG": "I", "KeyH": "D",
		"KeyJ": "H", "KeyK": "T", "KeyL": "N", "Semicolon": "S", "Quote": "-", "KeyZ": ";",
		"KeyX": "Q", "KeyC": "J", "KeyV": "K", "KeyB": "X", "KeyN": "B", "Comma": "W",
		"Period": "V", "Slash": "Z",
	}),
	with(map[string]string{ // Colemak
		"KeyE": "F", "KeyR": "P", "KeyT": "G", "KeyY": "J", "KeyU": "L", "KeyI": "U", "KeyO": "Y",
		"KeyP": ";", "KeyS": "R", "KeyD": "S", "KeyF": "T", "KeyG": "D", "KeyJ": "N", "KeyK": "E",
		"KeyL": "I", "Semicolon": "O", "KeyN": "K",
	}),
	with(map[string]string{ // Workman
		"KeyW": "D", "KeyE": "R", "KeyR": "W", "KeyT": "B", "KeyY": "J", "KeyU": "F", "KeyI": "U",
		"KeyO": "P", "KeyP": ";", "KeyD": "H", "KeyF": "T", "KeyH": "Y", "KeyJ": "N", "KeyK": "E",
		"KeyL": "O", "Semicolon": "I", "KeyC": "M", "KeyV": "C", "KeyB": "V", "KeyN": "K", "KeyM": "L",
	}),
}

// with is US with changes.
func with(changes map[string]string) map[string]string {
	l := maps.Clone(US)
	maps.Copy(l, changes)
	return l
}

// Name returns the key name the current layout gives the key at code,
// falling back to the US layout's for keys not yet reported.
func Name(code string) string {
	mu.RLock()
	n, ok := names[code]
	mu.RUnlock()
	if ok {
		return n
	}
	return US[code]
}

// CodeFor guesses the physical key for a key name, as on a US layout, for
// events from platforms that don't report physical keys.
func CodeFor(name string) string { return usCode[name] }

// Reset forgets every reported name, for a layout change.
func Reset() {
	mu.Lock()
	clear(names)
	mu.Unlock()
}

// Evdev names the physical key for a Linux evdev key code, which X11 and
// Wayland keycodes are offset from by 8. Windows scan codes for the same
// keys have the same values.
func Evdev(code uint32) string {
	if int(code) < len(evdev) {
		return evdev[code]
	}
	return ""
}

var evdev = [...]string{
	2: "Digit1", 3: "Digit2", 4: "Digit3", 5: "Digit4", 6: "Digit5", 7: "Digit6", 8: "Digit7",
	9: "Digit8", 10: "Digit9", 11: "Digit0", 12: "Minus", 13: "Equal",
	16: "KeyQ", 17: "KeyW", 18: "KeyE", 19: "KeyR", 20: "KeyT", 21: "KeyY", 22: "KeyU",
	23: "KeyI", 24: "KeyO", 25: "KeyP", 26: "BracketLeft", 27: "BracketRight",
	30: "KeyA", 31: "KeyS", 32: "KeyD", 33: "KeyF", 34: "KeyG", 35: "KeyH", 36: "KeyJ",
	37: "KeyK", 38: "KeyL", 39: "Semicolon", 40: "Quote", 41: "Backquote", 43: "Backslash",
	44: "KeyZ", 45: "KeyX", 46: "KeyC", 47: "KeyV", 48: "KeyB", 49: "KeyN", 50: "KeyM",
	51: "Comma", 52: "Period", 53: "Slash", 57: "Space", 86: "IntlBackslash",
}

// EvdevCodes lists the evdev key codes Evdev names, for querying a whole layout.
func EvdevCodes() []uint32 {
	var cs []uint32
	for c, n := range evdev {
		if n != "" {
			cs = append(cs, uint32(c))
		}
	}
	return cs
}

// Mac names the physical key for a macOS virtual key code (kVK_*).
func Mac(code uint16) string {
	if int(code) < len(mac) {
		return mac[code]
	}
	return ""
}

var mac = [...]string{
	0x00: "KeyA", 0x01: "KeyS", 0x02: "KeyD", 0x03: "KeyF", 0x04: "KeyH", 0x05: "KeyG",
	0x06: "KeyZ", 0x07: "KeyX", 0x08: "KeyC", 0x09: "KeyV", 0x0A: "IntlBackslash", 0x0B: "KeyB",
	0x0C: "KeyQ", 0x0D: "KeyW", 0x0E: "KeyE", 0x0F: "KeyR", 0x10: "KeyY", 0x11: "KeyT",
	0x12: "Digit1", 0x13: "Digit2", 0x14: "Digit3", 0x15: "Digit4", 0x16: "Digit6", 0x17: "Digit5",
	0x18: "Equal", 0x19: "Digit9", 0x1A: "Digit7", 0x1B: "Minus", 0x1C: "Digit8", 0x1D: "Digit0",
	0x1E: "BracketRight", 0x1F: "KeyO", 0x20: "KeyU", 0x21: "BracketLeft", 0x22: "KeyI", 0x23: "KeyP",
	0x25: "KeyL", 0x26: "KeyJ", 0x27: "Quote", 0x28: "KeyK", 0x29: "Semicolon", 0x2A: "Backslash",
	0x2B: "Comma", 0x2C: "Slash", 0x2D: "KeyN", 0x2E: "KeyM", 0x2F: "Period", 0x31: "Space",
	0x32: "Backquote",
}
