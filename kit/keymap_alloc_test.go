package kit

import (
	"testing"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
)

// TestKeymapFiltersAllocateOnce: collecting key filters for a frame reuses
// the filters boxed at bind time, so an idle frame with keymaps attached
// does not box a key.Filter per binding per frame.
func TestKeymapFiltersAllocateOnce(t *testing.T) {
	var k Keymap
	for _, spec := range []string{"N", "F2", "Ctrl+K", "Escape", "Space"} {
		k.MustBind(spec, spec, func() {})
	}
	buf := k.appendFilters(nil, false)
	if n := testing.AllocsPerRun(50, func() { buf = k.appendFilters(buf[:0], false) }); n > 0 {
		t.Fatalf("appendFilters allocates %.0f times, want 0", n)
	}

	var w Window
	gtx := w.Begin(layout.Context{Ops: new(op.Ops)})
	k.Attach(gtx)
	w.End(gtx)
	if n := testing.AllocsPerRun(50, func() { w.End(gtx) }); n > 0 {
		t.Fatalf("Window.End allocates %.0f times with a keymap attached, want 0", n)
	}
}
