package kit

import (
	"encoding/hex"
	"fmt"
	"image/color"
	"math"
	"strings"
)

// OKLCH is a perceptual color: L lightness 0..1, C chroma (~0..0.37), H hue in
// degrees. Equal steps in L look like equal steps in brightness, which is what
// makes generated palettes consistent across hues.
//
// It is the polar form of Björn Ottosson's Oklab color space ("A perceptual
// color space for image processing", 2020,
// https://bottosson.github.io/posts/oklab/); the conversion matrices below
// are the ones published there.
type OKLCH struct {
	L float64 // perceived lightness, 0 (black) to 1 (white)
	C float64 // chroma, 0 for grey; sRGB colours stay below about 0.37
	H float64 // hue angle in degrees, 0 to 360
}

// srgbToLinear and linearToSrgb are the sRGB transfer function and its
// inverse (IEC 61966-2-1), on components from 0 to 1.
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

// ToOKLCH converts an sRGB color, ignoring alpha.
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
	return OKLCH{L: L, C: math.Hypot(A, B), H: h}
}

// linear converts o to linear sRGB, unclamped: out-of-gamut colors have
// components outside 0..1.
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

// Gamut mapping in RGB: chroma shrinks by gamutChromaStep per attempt, for
// at most gamutMaxSteps attempts (0.9^24 leaves under 8% of the chroma),
// until each linear component is within gamutTolerance of 0..1. What is
// left over is clipped.
const (
	gamutChromaStep = 0.9
	gamutMaxSteps   = 24
	gamutTolerance  = 1e-4
)

// RGB converts to opaque sRGB, reducing chroma until the color fits the
// gamut (so hue and lightness are kept, saturation gives way).
func (o OKLCH) RGB() color.NRGBA {
	in := func(v float64) bool { return v >= -gamutTolerance && v <= 1+gamutTolerance }
	r, g, b := o.linear()
	for i := 0; i < gamutMaxSteps && !(in(r) && in(g) && in(b)); i++ {
		o.C *= gamutChromaStep
		r, g, b = o.linear()
	}
	cv := func(v float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(1, linearToSrgb(v))) * 255)) }
	return color.NRGBA{R: cv(r), G: cv(g), B: cv(b), A: 255}
}

// Luminance is WCAG 2.x relative luminance, from 0 (black) to 1 (white),
// ignoring alpha.
func Luminance(c color.NRGBA) float64 {
	return 0.2126*srgbToLinear(float64(c.R)/255) + 0.7152*srgbToLinear(float64(c.G)/255) + 0.0722*srgbToLinear(float64(c.B)/255)
}

// Contrast is the WCAG 2.x contrast ratio of two opaque colors, from 1 to
// 21, in either order. See ContrastAA and ContrastText for the targets.
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

// ParseHex parses a color written as #rgb, #rrggbb or #rrggbbaa, in
// either case; the leading # is optional. It returns the zero color and an
// error naming the input when s is not one of those forms.
func ParseHex(s string) (color.NRGBA, error) {
	c, err := parseHex(s)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("kit: %w", err)
	}
	return c, nil
}

func parseHex(s string) (color.NRGBA, error) {
	digits := strings.TrimPrefix(s, "#")
	if len(digits) == 3 { // #rgb is #rrggbb with each digit doubled
		digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
	}
	if len(digits) != 6 && len(digits) != 8 {
		return color.NRGBA{}, fmt.Errorf("hex color %q: want #rgb, #rrggbb or #rrggbbaa", s)
	}
	b, err := hex.DecodeString(digits)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("hex color %q: %w", s, err)
	}
	c := color.NRGBA{R: b[0], G: b[1], B: b[2], A: 255}
	if len(b) == 4 {
		c.A = b[3]
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
func WithAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

// Lighten brightens a color by d per channel (hover tints).
func Lighten(c color.NRGBA, d uint8) color.NRGBA {
	add := func(v uint8) uint8 { return uint8(min(255, int(v)+int(d))) }
	return color.NRGBA{R: add(c.R), G: add(c.G), B: add(c.B), A: c.A}
}
