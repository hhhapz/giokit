// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"image"
	"testing"

	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/io/semantic"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
)

// An area whose handler asks for no pointer gestures (keys only) keeps its
// semantic description.
func TestSemanticsOnGesturelessHandler(t *testing.T) {
	var (
		ops op.Ops
		r   Router
	)
	tag := new(int)
	area := clip.Rect(image.Rect(0, 0, 50, 50)).Push(&ops)
	event.Op(&ops, tag)
	semantic.DescriptionOp("keys").Add(&ops)
	area.Pop()
	r.Frame(&ops)
	r.Event(key.FocusFilter{Target: tag})
	r.Frame(&ops)
	var found bool
	for _, n := range r.AppendSemantics(nil) {
		if n.Desc.Description == "keys" {
			found = true
		}
	}
	if !found {
		t.Fatal("description on a key-only handler's area was dropped")
	}
}
