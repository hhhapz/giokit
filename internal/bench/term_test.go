package bench

import (
	"math/rand"
	"testing"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kittest"
)

func BenchmarkTerminalPerCell(b *testing.B) {
	th := kit.Dark()
	mt := th.Material()
	r := rand.New(rand.NewSource(1))
	const cols, rows = 200, 50
	draw := func(gtx layout.Context) {
		cw, ch := gtx.Dp(8), gtx.Dp(17)
		for y := 0; y < rows; y++ {
			for x := 0; x < cols; x++ {
				st := op.Offset(imagePt(x*cw, y*ch)).Push(gtx.Ops)
				if r.Intn(8) == 0 { // some cells have a bg
					paint.FillShape(gtx.Ops, palette[r.Intn(len(palette))], clip.Rect{Max: imagePt(cw, ch)}.Op())
				}
				l := material.Label(mt, unit.Sp(13), string(rune('!'+r.Intn(90))))
				l.Font = font.Font{Typeface: "Go Mono"}
				l.Color = palette[r.Intn(len(palette))]
				gtx2 := gtx
				gtx2.Constraints.Min.X = 0
				l.Layout(gtx2)
				st.Pop()
			}
		}
	}
	h, err := kittest.New(1600, 900, 1, draw)
	if err != nil {
		b.Skip(err)
	}
	defer h.Release()
	b.ResetTimer()
	t0 := time.Now()
	for i := 0; i < b.N; i++ {
		h.Settle()
	}
	_ = h.Pixel(imagePt(1, 1))
	b.ReportMetric(float64(time.Since(t0).Microseconds())/float64(b.N)/1000, "ms/frame")
}
