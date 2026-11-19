// Package forktrace adds Gio's low-level window events (wl_*, scroll_flush,
// gpu_frame, present) to GIOKIT_TRACE output, in order with kit's own
// lines, through the fork's app.GiokitTrace hook. Import it for its side
// effect:
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
// Without it, kit still traces its own frame pacing.
package forktrace

import (
	"github.com/hhhapz/giokit/gio/app"

	"github.com/hhhapz/giokit/kit"
)

func init() {
	if kit.TraceEnabled() {
		app.GiokitTrace = kit.Tracef
	}
}
