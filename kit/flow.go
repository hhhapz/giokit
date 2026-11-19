package kit

import (
	"image"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/unit"
)

// Flow lays children out left to right, wrapping onto new lines when the
// width runs out, like words in a paragraph (diff tokens, chips, tags).
// Each line is as tall as its tallest child; children top-align. hgap and
// vgap separate children on a line and the lines; they are used as given,
// not scaled by Density: pass th.Space(kit.SpaceN) for themed spacing.
func Flow(gtx layout.Context, hgap, vgap unit.Dp, children ...layout.Widget) layout.Dimensions {
	maxW := gtx.Constraints.Max.X
	hg, vg := gtx.Dp(hgap), gtx.Dp(vgap)
	cgtx := gtx
	cgtx.Constraints = layout.Constraints{Max: image.Pt(maxW, gtx.Constraints.Max.Y)}
	x, y, lineH, width := 0, 0, 0, 0
	for _, w := range children {
		m := op.Record(gtx.Ops)
		d := w(cgtx)
		call := m.Stop()
		if x > 0 && x+d.Size.X > maxW {
			x, y, lineH = 0, y+lineH+vg, 0
		}
		off := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		off.Pop()
		x += d.Size.X + hg
		lineH = max(lineH, d.Size.Y)
		width = max(width, x-hg)
	}
	return layout.Dimensions{Size: image.Pt(max(width, gtx.Constraints.Min.X), max(y+lineH, gtx.Constraints.Min.Y))}
}
