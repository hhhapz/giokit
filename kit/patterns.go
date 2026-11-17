package kit

import (
	"image"

	"gioui.org/op"
	"gioui.org/op/clip"
)

// Group names a region: id for tests, label for screen readers, sized to
// what w draws.
func Group(gtx C, id, label string, w W) D {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	TagLabel(gtx, id, label)
	call.Add(gtx.Ops)
	return dims
}
