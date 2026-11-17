package kit

import (
	"image"

	"gioui.org/op/paint"
)

// Image holds an uploaded image operation across frames. Set is called when
// decoded content changes, not during every layout. Decode off the UI goroutine.
type Image struct {
	op    paint.ImageOp
	ready bool
}

func (im *Image) Set(src image.Image) {
	im.ready = src != nil
	if src != nil {
		im.op = paint.NewImageOp(src)
	}
}

// Op is the uploaded image, and whether one is set (a nil Image has none).
func (im *Image) Op() (paint.ImageOp, bool) {
	if im == nil || !im.ready {
		return paint.ImageOp{}, false
	}
	return im.op, true
}
