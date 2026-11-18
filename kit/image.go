package kit

import (
	"image"

	"gioui.org/op/paint"
)

// Image holds an uploaded image operation across frames, so the image is
// uploaded once rather than on every layout. Decode off the UI goroutine;
// call Set on it, on the UI goroutine, when the decoded content changes.
// The zero value holds no image. An Image holds no event tags: a copy
// refers to the same uploaded image until either copy is Set again.
type Image struct {
	op    paint.ImageOp
	ready bool
}

// Set replaces the image with src, converting it to an image operation
// (src must not change afterwards). Set(nil) clears the image and drops the
// reference to the old one, so it can be freed.
func (im *Image) Set(src image.Image) {
	if src == nil {
		im.op, im.ready = paint.ImageOp{}, false
		return
	}
	im.op, im.ready = paint.NewImageOp(src), true
}

// Op is the uploaded image, and whether one is set. A nil *Image has none,
// so an optional image needs no nil check before Op.
func (im *Image) Op() (paint.ImageOp, bool) {
	if im == nil || !im.ready {
		return paint.ImageOp{}, false
	}
	return im.op, true
}
