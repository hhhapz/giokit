// Package forktrace adds Gio's low-level window events (wl_*, scroll_flush,
// gpu_frame, present) to GIOKIT_TRACE output, in order with kit's own
// lines. It needs giokit's Gio fork (third_party/gioui.org), which carries
// the hook; import it for its side effect from a program built on the fork:
//
//	import _ "github.com/hhhapz/giokit/kit/forktrace"
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
