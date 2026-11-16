// SPDX-License-Identifier: Unlicense OR MIT

package app

// GiokitTrace receives low-level window events for debugging when set
// (giokit patch: kit.App installs it under GIOKIT_TRACE). Keys and values
// alternate in kv. It is called on the window's event goroutine.
var GiokitTrace func(kind string, kv ...any)

func gtrace(kind string, kv ...any) {
	if f := GiokitTrace; f != nil {
		f(kind, kv...)
	}
}
