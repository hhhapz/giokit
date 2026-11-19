// SPDX-License-Identifier: Unlicense OR MIT

package app

// GiokitTrace receives low-level window events for debugging when set.
// Keys and values alternate in kv. giokit patch.
//
// It is process-wide and read without synchronization from every window's
// goroutines, possibly concurrently, so the function must be safe for
// concurrent use. Set it once at program start, before the first window is
// created, and do not change it afterwards: a later assignment is a data
// race. giokit's kit/forktrace package sets it from its init function when
// GIOKIT_TRACE is enabled, which Go's initialization order places before
// main runs.
var GiokitTrace func(kind string, kv ...any)

func gtrace(kind string, kv ...any) {
	if f := GiokitTrace; f != nil {
		f(kind, kv...)
	}
}
