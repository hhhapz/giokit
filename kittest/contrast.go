package kittest

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"gioui.org/f32"

	"github.com/hhhapz/giokit/kit"
)

// TextContrast is one label's measured contrast.
type TextContrast struct {
	Text    string          // the label's text
	Bounds  image.Rectangle // the label's bounds in pixels
	Fg, Bg  color.NRGBA     // the measured text and background colours
	Ratio   float64         // WCAG contrast ratio, 1..21
	LargePx int             // visible label-box height in px, for the large-text threshold
}

// Contrasts measures every enabled, visible label from the rendered pixels:
// the background is the most common color in the label's box, the text
// color the pixel that contrasts with it most (a glyph's solid core).
// Disabled labels (Theme.Disable) are exempt, as in WCAG.
func (h *Harness) Contrasts() []TextContrast {
	h.tb.Helper()
	img := h.Image()
	t := h.semantics()
	images := h.kwin.Images()
	var out []TextContrast
	for _, n := range t.nodes {
		d := n.Desc
		if d.Label == "" || t.disabled(n) {
			continue
		}
		if images[d.Description] {
			continue // an image label is alternative text, not painted glyphs
		}
		r := t.visible(n, img.Rect)
		if r.Dx() < minInkBox.X || r.Dy() < minInkBox.Y {
			continue
		}
		// Cut off by a scroll container (a row half scrolled out): a sliver
		// of glyphs says nothing about the text's contrast.
		if r.Dy() < d.Bounds.Intersect(img.Rect).Dy() {
			continue
		}
		// Covered (by a menu, a dialog's scrim): what shows there is
		// something else's pixels.
		mid := Center(r)
		if hit, ok := h.router.SemanticAt(f32.Pt(float32(mid.X), float32(mid.Y))); ok && hit != n.ID && !t.within(n.ID, hit) {
			continue
		}
		hist := map[color.NRGBA]int{}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				hist[opaque(img, x, y)]++
			}
		}
		var bg color.NRGBA
		best := -1
		for c, k := range hist {
			if k > best {
				bg, best = c, k
			}
		}
		fg, ratio := bg, 1.0
		for c := range hist {
			if cr := kit.Contrast(c, bg); cr > ratio {
				fg, ratio = c, cr
			}
		}
		if ratio < minInkContrast {
			continue // no ink: covered, or clipped by something that isn't a semantic area
		}
		if tok := h.inkToken(fg, bg); tok != nil {
			fg, ratio = *tok, kit.Contrast(*tok, bg)
		}
		out = append(out, TextContrast{Text: d.Label, Bounds: r, Fg: fg, Bg: bg, Ratio: ratio, LargePx: r.Dy()})
	}
	return out
}

// minInkBox is the smallest visible label box, in pixels, that Contrasts
// measures: a narrower or shorter box holds no measurable glyph. It is a
// var only because image.Point cannot be a constant.
var minInkBox = image.Pt(2, 4)

// Contrast measurement thresholds.
const (
	// minInkContrast is the ratio below which a box has no ink to judge:
	// its most contrasting pixel is that close to its background (covered
	// or clipped).
	minInkContrast = 1.1
	// inkMinCoverage and inkMaxCoverage bound how far along the line from
	// the background to a theme colour an antialiased pixel may lie to be
	// read as that colour's ink (1 is the colour itself; a little over 1
	// allows GPU rounding).
	inkMinCoverage = 0.6
	inkMaxCoverage = 1.02
	// inkMaxError is the largest linear-light distance from that line a
	// pixel may have and still be read as the colour's ink.
	inkMaxError = 0.03
	// largeTextDp approximates WCAG "large text" (18pt, i.e. 24px, regular)
	// by its line box: kit's label boxes are about 28dp tall at that size.
	// Bold text from 14pt also counts as large in WCAG; the audit cannot see
	// weight in pixels, so it holds bold text to the normal threshold.
	largeTextDp = 28
	// contrastSlack forgives rounding in the measured ratio.
	contrastSlack = 0.005
	// inkMinSpan is the smallest squared linear-light distance between a
	// theme colour and the background for inkToken to try that colour: a
	// colour this close to the background leaves no line to project onto.
	inkMinSpan = 1e-4
)

// opaque is the colour of img at (x, y) with full alpha: the composited
// window shows it that way.
func opaque(img *image.RGBA, x, y int) color.NRGBA {
	i := img.PixOffset(x, y)
	return color.NRGBA{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: 255}
}

// inkToken recognizes antialiased ink. Thin glyphs at small sizes may never
// cover a whole pixel, so the most contrasting pixel is a blend of the text
// color and the background. If it lies on the (linear-light) line from bg to
// a theme color, at least 60% of the way, that color is the ink. Faint text
// (a token at low alpha) lies nearer bg and is judged as drawn.
func (h *Harness) inkToken(fg, bg color.NRGBA) *color.NRGBA {
	if h.Theme == nil {
		return nil
	}
	p := h.Theme.Palette
	lin := func(c color.NRGBA) [3]float64 {
		return [3]float64{srgb(c.R), srgb(c.G), srgb(c.B)}
	}
	f, b := lin(fg), lin(bg)
	var best *color.NRGBA
	bestErr := inkMaxError
	for _, tok := range []color.NRGBA{p.Fg, p.Muted, p.Dim, p.Accent, p.OnAccent, p.Good, p.Warn, p.Bad} {
		t := lin(tok)
		var num, den float64
		for i := range 3 {
			num += (f[i] - b[i]) * (t[i] - b[i])
			den += (t[i] - b[i]) * (t[i] - b[i])
		}
		if den < inkMinSpan {
			continue
		}
		a := num / den
		if a < inkMinCoverage || a > inkMaxCoverage {
			continue
		}
		var e float64
		for i := range 3 {
			d := f[i] - (b[i] + a*(t[i]-b[i]))
			e += d * d
		}
		if e = math.Sqrt(e); e < bestErr {
			best, bestErr = &tok, e
		}
	}
	return best
}

// srgb converts an sRGB channel to linear light (IEC 61966-2-1).
func srgb(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// lowContrast returns a low-contrast problem per label under MinContrast
// (MinContrastLarge for large text), or nothing while MinContrast is 0.
func (h *Harness) lowContrast() []Problem {
	if h.MinContrast == 0 {
		return nil
	}
	large := h.Metric.Dp(largeTextDp)
	var out []Problem
	for _, c := range h.Contrasts() {
		want := h.MinContrast
		if c.LargePx >= large && h.MinContrastLarge > 0 {
			want = h.MinContrastLarge
		}
		if c.Ratio < want-contrastSlack {
			out = append(out, Problem{Kind: ProblemLowContrast, Bounds: c.Bounds,
				Msg: fmt.Sprintf("%q is %.2f:1 (%s on %s), want %.1f:1", c.Text, c.Ratio, kit.Hex(c.Fg), kit.Hex(c.Bg), want)})
		}
	}
	return out
}
