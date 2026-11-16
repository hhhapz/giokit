package kit

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
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

// Artwork draws square, cropped artwork or a theme-colored placeholder.
// The label describes the image to semantic readers and headless tests.
func (t *Theme) Artwork(gtx C, im *Image, id, label string, size unit.Dp) D {
	px := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	r := gtx.Dp(t.Radius)
	defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Max}, r).Push(gtx.Ops).Pop()
	TagLabel(gtx, id, label)
	semantic.Image.Add(gtx.Ops)
	if im != nil && im.ready {
		return widget.Image{Src: im.op, Fit: widget.Cover, Position: layout.Center}.Layout(gtx)
	}
	paint.FillShape(gtx.Ops, t.AccentBg, clip.Rect{Max: gtx.Constraints.Max}.Op())
	return D{Size: gtx.Constraints.Max}
}
