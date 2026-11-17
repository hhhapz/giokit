package kittest

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/io/input"

	"github.com/hhhapz/giokit/kit"
)

// TextContrast is one label's measured contrast.
type TextContrast struct {
	Text    string
	Bounds  image.Rectangle
	Fg, Bg  color.NRGBA
	Ratio   float64 // WCAG contrast ratio, 1..21
	LargePx int     // glyph-box height in px, for the large-text threshold
}

// Contrasts measures every enabled, visible label from the rendered pixels:
// the background is the most common color in the label's box, the text
// color the pixel that contrasts with it most (a glyph's solid core).
// Disabled labels (Theme.Disable) are exempt, as in WCAG.
func (h *Harness) Contrasts() []TextContrast {
	img := h.Image()
	nodes := h.nodes()
	byID := map[input.SemanticID]input.SemanticNode{}
	for _, n := range nodes {
		byID[n.ID] = n
	}
	images := h.kwin.Images()
	var out []TextContrast
	for _, n := range nodes {
		d := n.Desc
		if d.Label == "" || disabled(byID, n) {
			continue
		}
		if images[d.Description] {
			continue // an image label is alternative text, not painted glyphs
		}
		r := visible(byID, n, img.Rect)
		if r.Dx() < 2 || r.Dy() < 4 {
			continue
		}
		// Cut off by a scroll container (a row half scrolled out): a sliver
		// of glyphs says nothing about the text's contrast.
		if r.Dy() < d.Bounds.Intersect(img.Rect).Dy() {
			continue
		}
		// Covered (by a menu, a dialog's scrim): what shows there is
		// something else's pixels.
		c := r.Min.Add(r.Size().Div(2))
		if hit, ok := h.router.SemanticAt(f32.Pt(float32(c.X), float32(c.Y))); ok && hit != n.ID && !within(byID, n.ID, hit) {
			continue
		}
		hist := map[color.NRGBA]int{}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				hist[px(img, x, y)]++
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
		if ratio < 1.1 {
			continue // no ink: covered, or clipped by something that isn't a semantic area
		}
		if t := h.inkToken(fg, bg); t != nil {
			fg, ratio = *t, kit.Contrast(*t, bg)
		}
		out = append(out, TextContrast{Text: d.Label, Bounds: r, Fg: fg, Bg: bg, Ratio: ratio, LargePx: r.Dy()})
	}
	return out
}

func px(img *image.RGBA, x, y int) color.NRGBA {
	i := img.PixOffset(x, y)
	return color.NRGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], 255}
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
	bestErr := 0.03
	for _, tok := range []color.NRGBA{p.Fg, p.Muted, p.Dim, p.Accent, p.OnAccent, p.Good, p.Warn, p.Bad} {
		t := lin(tok)
		var num, den float64
		for i := range 3 {
			num += (f[i] - b[i]) * (t[i] - b[i])
			den += (t[i] - b[i]) * (t[i] - b[i])
		}
		if den < 1e-4 {
			continue
		}
		a := num / den
		if a < 0.6 || a > 1.02 {
			continue
		}
		var e float64
		for i := range 3 {
			d := f[i] - (b[i] + a*(t[i]-b[i]))
			e += d * d
		}
		if e = math.Sqrt(e); e < bestErr {
			tok := tok
			best, bestErr = &tok, e
		}
	}
	return best
}

func srgb(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// visible clips n's bounds to its ancestors' (a scrolled list's rows report
// bounds outside its viewport) and to the screen.
func visible(byID map[input.SemanticID]input.SemanticNode, n input.SemanticNode, screen image.Rectangle) image.Rectangle {
	r := n.Desc.Bounds.Intersect(screen)
	for p, ok := byID[n.ParentID]; ok; p, ok = byID[p.ParentID] {
		if b := p.Desc.Bounds; !b.Empty() {
			r = r.Intersect(b)
		}
		if p.ParentID == 0 {
			break
		}
	}
	return r
}

// lowContrast returns a low-contrast problem per label under MinContrast
// (MinContrastLarge for large text), or nothing while MinContrast is 0.
func (h *Harness) lowContrast() []Problem {
	if h.MinContrast == 0 {
		return nil
	}
	large := h.Metric.Dp(28) // ~ a 24px (18pt) font's line box: WCAG "large text"
	var out []Problem
	for _, c := range h.Contrasts() {
		want := h.MinContrast
		if c.LargePx >= large && h.MinContrastLarge > 0 {
			want = h.MinContrastLarge
		}
		if c.Ratio < want-0.005 {
			out = append(out, Problem{Kind: "low-contrast", Bounds: c.Bounds,
				Msg: fmt.Sprintf("%q is %.2f:1 (%s on %s), want %.1f:1", c.Text, c.Ratio, kit.Hex(c.Fg), kit.Hex(c.Bg), want)})
		}
	}
	return out
}

// disabled reports whether n or an ancestor is disabled.
func disabled(byID map[input.SemanticID]input.SemanticNode, n input.SemanticNode) bool {
	for ok := true; ok; n, ok = byID[n.ParentID] {
		if n.Desc.Disabled {
			return true
		}
		if n.ParentID == 0 {
			break
		}
	}
	return false
}
