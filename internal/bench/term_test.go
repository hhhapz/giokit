package bench

import (
	"image"
	"math/rand"
	"testing"

	"github.com/hhhapz/giokit/gio/font"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"
	"github.com/hhhapz/giokit/gio/unit"
	"github.com/hhhapz/giokit/gio/widget/material"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kittest"
)

// BenchmarkTerminalPerCell draws a terminal grid with one label per cell
// and a background on one cell in eight: the naive renderer's cost.
func BenchmarkTerminalPerCell(b *testing.B) {
	th := kit.Dark()
	mt := th.Material()
	r := rand.New(rand.NewSource(1))
	const cols, rows = 200, 50
	draw := func(gtx layout.Context) {
		cw, ch := gtx.Dp(8), gtx.Dp(17)
		for y := range rows {
			for x := range cols {
				cell := op.Offset(image.Pt(x*cw, y*ch)).Push(gtx.Ops)
				if r.Intn(8) == 0 {
					paint.FillShape(gtx.Ops, syntaxColors[r.Intn(len(syntaxColors))], clip.Rect{Max: image.Pt(cw, ch)}.Op())
				}
				l := material.Label(mt, unit.Sp(13), string(rune('!'+r.Intn(90))))
				l.Font = font.Font{Typeface: "Go Mono"}
				l.Color = syntaxColors[r.Intn(len(syntaxColors))]
				cellGtx := gtx
				cellGtx.Constraints.Min.X = 0
				l.Layout(cellGtx)
				cell.Pop()
			}
		}
	}
	h, err := kittest.New(1600, 900, 1, draw)
	if err != nil {
		b.Skip(err)
	}
	defer h.Release()
	measureFrames(b, h)
}
