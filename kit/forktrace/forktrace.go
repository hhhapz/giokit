// Package forktrace adds Gio's low-level window events (wl_*, scroll_flush,
// gpu_frame, present) to GIOKIT_TRACE output, in order with kit's own
// lines. It needs giokit's Gio fork (third_party/gioui.org), which carries
// the hook; import it for its side effect from a program built on the fork:
//
//	import _ "github.com/hhhapz/giokit/kit/forktrace"
//
// Importing it is the only installation step: when GIOKIT_TRACE is set, the
// package's init assigns kit.Tracef to the fork's app.GiokitTrace hook.
// Go runs that init before main, so the hook is in place before any window
// starts. The hook is startup-only configuration: it is read on window
// goroutines without synchronization, so nothing may reassign it once a
// window is running.
//
// kit itself builds on stock Gio and traces frame pacing without it.
package forktrace

import (
	"gioui.org/app"

	"github.com/hhhapz/giokit/kit"
)

func init() {
	if kit.TraceEnabled() {
		app.GiokitTrace = kit.Tracef
	}
}
