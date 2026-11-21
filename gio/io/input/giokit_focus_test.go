// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"image"
	"testing"

	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
)

// TestMoveFocusCmd checks key.MoveFocusCmd moves focus like Router.MoveFocus.
func TestMoveFocusCmd(t *testing.T) {
	ops := new(op.Ops)
	r := new(Router)
	handlers := []image.Rectangle{image.Rect(0, 0, 10, 10), image.Rect(0, 20, 10, 30)}
	for i, b := range handlers {
		cl := clip.Rect(b).Push(ops)
		event.Op(ops, &handlers[i])
		cl.Pop()
		events(r, -1, key.FocusFilter{Target: &handlers[i]})
	}
	r.Frame(ops)
	r.Source().Execute(key.MoveFocusCmd{Dir: key.FocusForward})
	assertFocus(t, r, &handlers[0])
	r.Source().Execute(key.MoveFocusCmd{Dir: key.FocusDown})
	assertFocus(t, r, &handlers[1])
	r.Source().Execute(key.MoveFocusCmd{Dir: key.FocusBackward})
	assertFocus(t, r, &handlers[0])
}
