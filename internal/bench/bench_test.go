// Package bench holds rendering baselines for text-heavy UIs (editor,
// terminal) and video upload. They render through kittest's headless GPU
// context and skip when none is available.
//
//	go test -run x -bench . -benchtime 100x ./internal/bench
package bench

import (
	"image"
	"image/color"
	"math/rand"
	"testing"
	"time"

	"github.com/hhhapz/giokit/gio/font"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/unit"
	"github.com/hhhapz/giokit/gio/widget/material"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kittest"
)

var words = []string{"func", "return", "if", "err", "!=", "nil", "{", "}", "ctx", "(", ")", "for", "range", ":=", "value", "string", "\"hello\"", "42", "//", "comment", "struct", "map", "[]byte", "go", "defer"}

// syntaxColors stand in for an editor's syntax highlighting.
var syntaxColors = []color.NRGBA{
	{R: 0xc6, G: 0x78, B: 0xdd, A: 255},
	{R: 0x98, G: 0xc3, B: 0x79, A: 255},
	{R: 0xe5, G: 0xc0, B: 0x7b, A: 255},
	{R: 0x61, G: 0xaf, B: 0xef, A: 255},
	{R: 0xab, G: 0xb2, B: 0xbf, A: 255},
	{R: 0x5c, G: 0x63, B: 0x70, A: 255},
}

// tok is one coloured word of a line of code.
type tok struct {
	text  string
	color color.NRGBA
}

// textConfig is the workload of one text benchmark.
type textConfig struct {
	// unique replaces every word with random letters, so the shaper's cache
	// never hits: the cost of text seen for the first time.
	unique bool
	// scale is the pixels per dp of the offscreen window.
	scale float32
}

// Text benchmark dimensions.
const (
	textLines   = 5000 // lines of generated code
	visibleRows = 60   // lines drawn per frame
	lineHeight  = 16   // dp
)

// genLines generates n lines of 8 to 21 words, deterministic for a seed.
func genLines(n int, seed int64, unique bool) [][]tok {
	r := rand.New(rand.NewSource(seed))
	out := make([][]tok, n)
	for i := range out {
		k := 8 + r.Intn(14)
		for range k {
			w := words[r.Intn(len(words))]
			if unique {
				bs := make([]byte, 3+r.Intn(8))
				for q := range bs {
					bs[q] = byte('a' + r.Intn(26))
				}
				w = string(bs)
			}
			out[i] = append(out[i], tok{text: w + " ", color: syntaxColors[r.Intn(len(syntaxColors))]})
		}
	}
	return out
}

// mutation changes what the next frame draws: the lines, or the first
// visible line.
type mutation func(frame int, lines *[][]tok, top *int)

func static(int, *[][]tok, *int) {}

func scroll(frame int, _ *[][]tok, top *int) { *top = frame }

// allNew replaces every visible line each frame.
func allNew(cfg textConfig) mutation {
	return func(frame int, lines *[][]tok, _ *int) {
		*lines = genLines(visibleRows, int64(frame+100), cfg.unique)
	}
}

// runText draws visibleRows lines of coloured words, one label per word,
// and reports the time per frame including GPU work.
func runText(b *testing.B, cfg textConfig, mutate mutation) {
	th := kit.Dark()
	mt := th.Material()
	lines := genLines(textLines, 1, cfg.unique)
	top, frame := 0, 0
	draw := func(gtx layout.Context) {
		mutate(frame, &lines, &top)
		frame++
		lineH := gtx.Dp(lineHeight)
		for i := range visibleRows {
			ln := lines[(top+i)%len(lines)]
			row := op.Offset(image.Pt(0, i*lineH)).Push(gtx.Ops)
			x := 0
			for _, t := range ln {
				word := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
				l := material.Label(mt, unit.Sp(13), t.text)
				l.Font = font.Font{Typeface: "Go Mono"}
				l.Color = t.color
				l.MaxLines = 1
				wordGtx := gtx
				wordGtx.Constraints.Min.X = 0
				d := l.Layout(wordGtx)
				x += d.Size.X
				word.Pop()
			}
			row.Pop()
		}
	}
	h, err := kittest.New(1600, 1000, cfg.scale, draw)
	if err != nil {
		b.Skip(err)
	}
	defer h.Release()
	h.Settle()
	measureFrames(b, h)
}

// measureFrames settles b.N frames, waits for the GPU and reports
// milliseconds per frame.
func measureFrames(b *testing.B, h *kittest.Harness) {
	b.Helper()
	b.ResetTimer()
	t0 := time.Now()
	for range b.N {
		h.Settle()
	}
	_ = h.Pixel(image.Pt(1, 1)) // a readback waits for the GPU
	b.ReportMetric(float64(time.Since(t0).Microseconds())/float64(b.N)/1000, "ms/frame")
}

var (
	lowDPI       = textConfig{scale: 1}
	lowDPIUnique = textConfig{scale: 1, unique: true}
	hiDPI        = textConfig{scale: 2}
	hiDPIUnique  = textConfig{scale: 2, unique: true}
)

func BenchmarkStatic(b *testing.B)            { runText(b, lowDPI, static) }
func BenchmarkScroll(b *testing.B)            { runText(b, lowDPI, scroll) }
func BenchmarkAllNew(b *testing.B)            { runText(b, lowDPI, allNew(lowDPI)) }
func BenchmarkUniqueAllNew(b *testing.B)      { runText(b, lowDPIUnique, allNew(lowDPIUnique)) }
func BenchmarkUniqueScroll(b *testing.B)      { runText(b, lowDPIUnique, scroll) }
func BenchmarkHiDPIStatic(b *testing.B)       { runText(b, hiDPI, static) }
func BenchmarkHiDPIUniqueScroll(b *testing.B) { runText(b, hiDPIUnique, scroll) }
func BenchmarkHiDPIUniqueAllNew(b *testing.B) { runText(b, hiDPIUnique, allNew(hiDPIUnique)) }
