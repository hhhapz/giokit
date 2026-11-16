package bench

import (
	"fmt"
	"image/color"
	"math/rand"
	"testing"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kittest"
)

var words = []string{"func", "return", "if", "err", "!=", "nil", "{", "}", "ctx", "(", ")", "for", "range", ":=", "value", "string", "\"hello\"", "42", "//", "comment", "struct", "map", "[]byte", "go", "defer"}
var palette = []color.NRGBA{{0xc6, 0x78, 0xdd, 255}, {0x98, 0xc3, 0x79, 255}, {0xe5, 0xc0, 0x7b, 255}, {0x61, 0xaf, 0xef, 255}, {0xab, 0xb2, 0xbf, 255}, {0x5c, 0x63, 0x70, 255}}

type tok struct {
	s string
	c color.NRGBA
}

var unique bool
var scale float32 = 1

func genLines(n int, seed int64) [][]tok {
	r := rand.New(rand.NewSource(seed))
	out := make([][]tok, n)
	for i := range out {
		k := 8 + r.Intn(14)
		for j := 0; j < k; j++ {
			w := words[r.Intn(len(words))]
			if unique {
				bs := make([]byte, 3+r.Intn(8))
				for q := range bs {
					bs[q] = byte('a' + r.Intn(26))
				}
				w = string(bs)
			}
			out[i] = append(out[i], tok{w + " ", palette[r.Intn(len(palette))]})
		}
	}
	return out
}

func run(b *testing.B, name string, mutate func(frame int, lines *[][]tok, top *int)) {
	th := kit.Dark()
	mt := th.Material()
	lines := genLines(5000, 1)
	top, frame := 0, 0
	const rows = 60
	draw := func(gtx layout.Context) {
		mutate(frame, &lines, &top)
		frame++
		lineH := gtx.Dp(16)
		for i := 0; i < rows; i++ {
			ln := lines[(top+i)%len(lines)]
			st := op.Offset(imagePt(0, i*lineH)).Push(gtx.Ops)
			x := 0
			for _, t := range ln {
				s2 := op.Offset(imagePt(x, 0)).Push(gtx.Ops)
				l := material.Label(mt, unit.Sp(13), t.s)
				l.Font = font.Font{Typeface: "Go Mono"}
				l.Color = t.c
				l.MaxLines = 1
				gtx2 := gtx
				gtx2.Constraints.Min.X = 0
				d := l.Layout(gtx2)
				x += d.Size.X
				s2.Pop()
			}
			st.Pop()
		}
	}
	h, err := kittest.New(1600, 1000, scale, draw)
	if err != nil {
		b.Skip(err)
	}
	defer h.Release()
	h.Settle()
	b.ResetTimer()
	t0 := time.Now()
	for i := 0; i < b.N; i++ {
		h.Settle()
	}
	_ = h.Pixel(imagePt(1, 1)) // force GPU sync
	b.ReportMetric(float64(time.Since(t0).Microseconds())/float64(b.N)/1000, "ms/frame")
	_ = fmt.Sprint
}

func BenchmarkStatic(b *testing.B) { run(b, "static", func(int, *[][]tok, *int) {}) }
func BenchmarkScroll(b *testing.B) {
	run(b, "scroll", func(f int, _ *[][]tok, top *int) { *top = f })
}
func BenchmarkAllNew(b *testing.B) {
	run(b, "allnew", func(f int, l *[][]tok, _ *int) { *l = genLines(60, int64(f+100)) })
}

func BenchmarkUniqueAllNew(b *testing.B) {
	unique = true
	defer func() { unique = false }()
	run(b, "u", func(f int, l *[][]tok, _ *int) { *l = genLines(60, int64(f+100)) })
}
func BenchmarkUniqueScroll(b *testing.B) {
	unique = true
	defer func() { unique = false }()
	run(b, "u", func(f int, _ *[][]tok, top *int) { *top = f })
}
func BenchmarkHiDPIStatic(b *testing.B) {
	scale = 2
	defer func() { scale = 1 }()
	run(b, "h", func(int, *[][]tok, *int) {})
}
func BenchmarkHiDPIUniqueScroll(b *testing.B) {
	scale, unique = 2, true
	defer func() { scale, unique = 1, false }()
	run(b, "h", func(f int, _ *[][]tok, top *int) { *top = f })
}
func BenchmarkHiDPIUniqueAllNew(b *testing.B) {
	scale, unique = 2, true
	defer func() { scale, unique = 1, false }()
	run(b, "h", func(f int, l *[][]tok, _ *int) { *l = genLines(60, int64(f+100)) })
}
