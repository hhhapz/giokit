package kit

import (
	"fmt"
	"image/color"
	"math"
)

// OKLCH is a perceptual color: L lightness 0..1, C chroma (~0..0.37), H hue in
// degrees. Equal steps in L look like equal steps in brightness, which is what
// makes generated palettes consistent across hues.
type OKLCH struct{ L, C, H float64 }

func srgbToLinear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func linearToSrgb(v float64) float64 {
	if v <= 0.0031308 {
		return 12.92 * v
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

// ToOKLCH converts an sRGB color.
func ToOKLCH(c color.NRGBA) OKLCH {
	r, g, b := srgbToLinear(float64(c.R)/255), srgbToLinear(float64(c.G)/255), srgbToLinear(float64(c.B)/255)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	L := 0.2104542553*l + 0.7936177850*m - 0.0040720468*s
	A := 1.9779984951*l - 2.4285922050*m + 0.4505937099*s
	B := 0.0259040371*l + 0.7827717662*m - 0.8086757660*s
	h := math.Atan2(B, A) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return OKLCH{L, math.Hypot(A, B), h}
}

func (o OKLCH) linear() (r, g, b float64) {
	a, bb := o.C*math.Cos(o.H*math.Pi/180), o.C*math.Sin(o.H*math.Pi/180)
	l := o.L + 0.3963377774*a + 0.2158037573*bb
	m := o.L - 0.1055613458*a - 0.0638541728*bb
	s := o.L - 0.0894841775*a - 1.2914855480*bb
	l, m, s = l*l*l, m*m*m, s*s*s
	return 4.0767416621*l - 3.3077115913*m + 0.2309699292*s,
		-1.2684380046*l + 2.6097574011*m - 0.3413193965*s,
		-0.0041960863*l - 0.7034186147*m + 1.7076147010*s
}

// RGB converts to sRGB, reducing chroma until the color fits the gamut (so
// hue and lightness are kept, saturation gives way).
func (o OKLCH) RGB() color.NRGBA {
	in := func(v float64) bool { return v >= -1e-4 && v <= 1+1e-4 }
	r, g, b := o.linear()
	for i := 0; i < 24 && !(in(r) && in(g) && in(b)); i++ {
		o.C *= 0.9
		r, g, b = o.linear()
	}
	cv := func(v float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(1, linearToSrgb(v))) * 255)) }
	return color.NRGBA{R: cv(r), G: cv(g), B: cv(b), A: 255}
}

// Luminance is WCAG relative luminance.
func Luminance(c color.NRGBA) float64 {
	return 0.2126*srgbToLinear(float64(c.R)/255) + 0.7152*srgbToLinear(float64(c.G)/255) + 0.0722*srgbToLinear(float64(c.B)/255)
}

// Contrast is the WCAG contrast ratio (1..21). Body text wants ≥ 4.5.
func Contrast(a, b color.NRGBA) float64 {
	la, lb := Luminance(a), Luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// Hex formats a color as #rrggbb (or #rrggbbaa when translucent).
func Hex(c color.NRGBA) string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

// ParseHex parses #rgb, #rrggbb or #rrggbbaa.
func ParseHex(s string) (color.NRGBA, error) {
	var c color.NRGBA
	c.A = 255
	if len(s) > 0 && s[0] == '#' {
		s = s[1:]
	}
	var err error
	switch len(s) {
	case 3:
		_, err = fmt.Sscanf(s, "%1x%1x%1x", &c.R, &c.G, &c.B)
		c.R, c.G, c.B = c.R*17, c.G*17, c.B*17
	case 6:
		_, err = fmt.Sscanf(s, "%02x%02x%02x", &c.R, &c.G, &c.B)
	case 8:
		_, err = fmt.Sscanf(s, "%02x%02x%02x%02x", &c.R, &c.G, &c.B, &c.A)
	default:
		err = fmt.Errorf("bad length")
	}
	if err != nil {
		return c, fmt.Errorf("kit: bad hex color %q", s)
	}
	return c, nil
}

// Lerp interpolates between a and b (t in 0..1).
func Lerp(a, b, t float32) float32 { return a + (b-a)*t }

// LerpColor interpolates two colors in sRGB (fine for short UI tints).
func LerpColor(a, b color.NRGBA, t float32) color.NRGBA {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	l := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*t + 0.5) }
	return color.NRGBA{R: l(a.R, b.R), G: l(a.G, b.G), B: l(a.B, b.B), A: l(a.A, b.A)}
}

// WithAlpha returns c with alpha a.
func WithAlpha(c color.NRGBA, a uint8) color.NRGBA { c.A = a; return c }

// Lighten brightens a color by d per channel (hover tints).
func Lighten(c color.NRGBA, d uint8) color.NRGBA {
	add := func(v uint8) uint8 { return uint8(min(255, int(v)+int(d))) }
	return color.NRGBA{R: add(c.R), G: add(c.G), B: add(c.B), A: c.A}
}

// Seed is the input to Generate: a couple of brand colors in, a full
// contrast-checked theme out.
type Seed struct {
	Name    string
	Accent  color.NRGBA // brand / primary color
	Neutral color.NRGBA // tints the grays; its hue matters, chroma is capped (zero value = pure gray)
	Dark    bool
	// Contrast 0 (default) … 1 (high contrast: brighter text, stronger borders).
	Contrast float64
}

// Generate builds a complete theme from a seed (see GeneratePalette).
func Generate(s Seed) *Theme {
	t := New(GeneratePalette(s))
	t.Name, t.Dark = s.Name, s.Dark
	return t
}

// GeneratePalette derives every token from a seed in OKLCH: neutrals on a
// fixed lightness ladder tinted by Neutral's hue, accent and status colors at
// lightnesses that keep text readable on the background in both modes.
// Apply it to an existing theme with Derive to keep its fonts and metrics.
func GeneratePalette(s Seed) Palette {
	n := ToOKLCH(s.Neutral)
	nc := math.Min(n.C, 0.03) * 0.6
	if s.Neutral.A == 0 {
		nc = 0
	}
	a := ToOKLCH(s.Accent)
	k := s.Contrast
	neu := func(l float64) color.NRGBA { return OKLCH{l, nc, n.H}.RGB() }
	hue := func(h, l, c float64) color.NRGBA { return OKLCH{l, c, h}.RGB() }

	var p Palette
	if s.Dark {
		p.Bg, p.Panel, p.Surface, p.Hover = neu(0.215-0.05*k), neu(0.24-0.04*k), neu(0.275-0.02*k), neu(0.305)
		p.Border, p.Divider = neu(0.33+0.12*k), neu(0.26+0.08*k)
		p.Fg, p.Muted, p.Dim = neu(0.91+0.08*k), neu(0.71+0.12*k), neu(0.54+0.14*k)
		al, bgl, brl := 0.74+0.08*k, 0.33, 0.43
		p.Accent, p.AccentBg, p.AccentBorder = hue(a.H, al, a.C), hue(a.H, bgl, a.C*0.35), hue(a.H, brl, a.C*0.45)
		p.OnAccent = neu(0.18)
		p.Good, p.GoodBg = hue(140, al, 0.12), hue(140, 0.28, 0.035)
		p.Warn, p.WarnBg = hue(85, al+0.06, 0.11), hue(85, 0.29, 0.035)
		p.Bad, p.BadBg = hue(22, al-0.06, 0.13), hue(22, 0.28, 0.045)
		p.Overlay, p.Scrim = neu(0.315), color.NRGBA{A: 0x8c}
	} else {
		p.Bg, p.Panel, p.Surface, p.Hover = neu(0.995), neu(0.97-0.01*k), neu(0.985), neu(0.935)
		p.Border, p.Divider = neu(0.87-0.2*k), neu(0.925-0.1*k)
		p.Fg, p.Muted, p.Dim = neu(0.24-0.1*k), neu(0.47-0.12*k), neu(0.6-0.14*k)
		al := 0.53 - 0.08*k
		p.Accent, p.AccentBg, p.AccentBorder = hue(a.H, al, a.C), hue(a.H, 0.95, a.C*0.25), hue(a.H, 0.83, a.C*0.45)
		p.OnAccent = neu(0.99)
		p.Good, p.GoodBg = hue(145, al, 0.13), hue(145, 0.95, 0.04)
		p.Warn, p.WarnBg = hue(70, al+0.05, 0.12), hue(85, 0.95, 0.05)
		p.Bad, p.BadBg = hue(25, al, 0.17), hue(25, 0.95, 0.04)
		p.Overlay, p.Scrim = neu(1), WithAlpha(neu(0.2), 0x52)
	}
	return p.Legible(s.Dark)
}

// Text contrast targets Legible enforces (WCAG AA is 4.5:1 for body text).
const (
	ContrastText  = 7.0 // Fg
	ContrastMuted = 6.5 // Muted: kept a clear step above Dim
	ContrastAA    = 4.5 // Dim, accent and status text
)

// Legible returns p with every text color moved in lightness, away from the
// backgrounds and only as far as needed, until it meets its contrast target
// on every surface it is drawn on: Fg, Muted and Dim on Bg, Panel, Surface
// and Hover; Accent and the status colors there and on their own tinted
// backgrounds; OnAccent on Accent. Colors that already pass are unchanged,
// and hue and chroma are kept. Focus follows Accent.
func (p Palette) Legible(dark bool) Palette {
	fit := func(c *color.NRGBA, target float64, bgs ...color.NRGBA) {
		*c = fitContrast(*c, target, dark, bgs)
	}
	surfaces := []color.NRGBA{p.Bg, p.Panel, p.Surface, p.Hover, p.Overlay}
	fit(&p.Fg, ContrastText, surfaces...)
	// Button fills tint toward Fg when hovered or pressed: text must stay
	// readable at the strongest tint too.
	pressed := func(c color.NRGBA) color.NRGBA { return LerpColor(c, p.Fg, PressTint) }
	surfaces = append(surfaces, pressed(p.Surface), pressed(p.Overlay)) // menu items highlight like buttons
	fit(&p.Fg, ContrastText, surfaces...)
	fit(&p.Muted, ContrastMuted, surfaces...)
	fit(&p.Dim, ContrastAA, surfaces...)
	fit(&p.Accent, ContrastAA, append(surfaces, p.AccentBg, pressed(p.AccentBg))...)
	fit(&p.Good, ContrastAA, append(surfaces, p.GoodBg)...)
	fit(&p.Warn, ContrastAA, append(surfaces, p.WarnBg)...)
	fit(&p.Bad, ContrastAA, append(surfaces, p.BadBg, pressed(p.BadBg))...)
	// OnAccent sits on Accent: the opposite direction from the other texts.
	p.OnAccent = fitContrast(p.OnAccent, ContrastAA, !dark, []color.NRGBA{p.Accent})
	p.Focus = p.Accent
	return p
}

// WithContrast raises a palette's contrast for any theme, generated or
// hand-made: level 0 returns p unchanged, 1 is high contrast (text toward
// AAA, borders and the focus ring at least 3:1 against the background, as
// WCAG asks of non-text UI).
func (p Palette) WithContrast(dark bool, level float64) Palette {
	if level <= 0 {
		return p
	}
	k := min(level, 1)
	fit := func(c *color.NRGBA, target float64, bgs ...color.NRGBA) {
		*c = fitContrast(*c, target, dark, bgs)
	}
	surfaces := []color.NRGBA{p.Bg, p.Panel, p.Surface, p.Hover, p.Overlay}
	fit(&p.Fg, ContrastText+5*k, surfaces...)
	fit(&p.Muted, ContrastMuted+2*k, surfaces...)
	fit(&p.Dim, ContrastAA+2*k, surfaces...)
	fit(&p.Border, 1.6+1.4*k, p.Bg, p.Panel, p.Surface)
	fit(&p.Divider, 1.3+0.7*k, p.Bg, p.Panel)
	fit(&p.AccentBorder, 1.6+1.4*k, p.Bg, p.Surface)
	fit(&p.Accent, ContrastAA+1.5*k, append(surfaces, p.AccentBg)...)
	fit(&p.Focus, 3, p.Bg, p.Panel, p.Surface)
	return p.Legible(dark)
}

// fitContrast steps c's OKLCH lightness up (lighter) or down until it reaches
// target against every bg, or hits white/black.
func fitContrast(c color.NRGBA, target float64, lighter bool, bgs []color.NRGBA) color.NRGBA {
	worst := func(c color.NRGBA) float64 {
		w := 21.0
		for _, b := range bgs {
			w = math.Min(w, Contrast(c, b))
		}
		return w
	}
	if worst(c) >= target {
		return c
	}
	o := ToOKLCH(c)
	for i := 0; i < 200 && worst(c) < target; i++ {
		if lighter {
			o.L = math.Min(1, o.L+0.005)
		} else {
			o.L = math.Max(0, o.L-0.005)
		}
		c = o.RGB()
		if o.L == 0 || o.L == 1 {
			break
		}
	}
	return c
}

// PresetSeeds are the seeds of the generated presets.
var PresetSeeds = []Seed{
	{Name: "Light", Accent: rgb(0x2f6fd0), Neutral: rgb(0x6b7280)},
	{Name: "Midnight", Dark: true, Accent: rgb(0x8b7cf6), Neutral: rgb(0x4b5a8a)},
	{Name: "Forest", Dark: true, Accent: rgb(0x4fbf8a), Neutral: rgb(0x4a6b58)},
	{Name: "Paper", Accent: rgb(0xd9622b), Neutral: rgb(0x8a7a62)},
	{Name: "Rose", Accent: rgb(0xd6457a), Neutral: rgb(0x8a6070)},
	{Name: "High contrast", Dark: true, Accent: rgb(0x5cb6ff), Contrast: 1},
}

// Presets returns the bundled themes in display order (Dark first). They
// share one font shaper, so switching between them keeps text caches warm.
func Presets() []*Theme {
	out := []*Theme{Dark()}
	for _, s := range PresetSeeds {
		out = append(out, out[0].Derive(func(t *Theme) { t.Palette, t.Name, t.Dark = GeneratePalette(s), s.Name, s.Dark }))
	}
	return out
}
