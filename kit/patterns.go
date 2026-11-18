package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// Group names a region: id for tests, label for screen readers, sized to
// what w draws. It records no role: a caller whose region is a semantic
// group, toolbar or status sets that role on id with SetRole.
func Group(gtx layout.Context, id, label string, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	TagLabel(gtx, id, label)
	call.Add(gtx.Ops)
	return dims
}
