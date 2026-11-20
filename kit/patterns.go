package kit

import (
	"image"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
)

// Group names a region: id for tests, label for screen readers, role
// (RoleGroup, RoleToolbar, RoleStatus...; 0 for none), sized to what w
// draws.
func Group(gtx layout.Context, id, label string, role Role, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	TagLabel(gtx, id, label)
	if role != 0 {
		role.Add(gtx.Ops)
	}
	call.Add(gtx.Ops)
	return dims
}
