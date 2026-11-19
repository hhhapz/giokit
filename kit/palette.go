package kit

import (
	"image/color"
	"math"
	"slices"
)

// Palette holds the semantic color tokens every component draws from. Each
// names a role, not a hue, so one set of components reads correctly in any
// palette; the comments say where each role appears. Legible fixes a
// palette's text colors to meet their contrast targets.
type Palette struct {
	Bg      color.NRGBA // window background
	Panel   color.NRGBA // bars, sidebars, cards
	Surface color.NRGBA // buttons, inputs, selected nav item
	Hover   color.NRGBA // hovered rows, nav items and headers; differs from Bg and Panel, so it shows on cards
	Border  color.NRGBA // panel separators, button and input outlines
	Divider color.NRGBA // row separators, card outlines

	Fg    color.NRGBA // primary text
	Muted color.NRGBA // secondary text
	Dim   color.NRGBA // tertiary text, headings, disabled

	Accent, AccentBg, AccentBorder color.NRGBA // primary actions, selection, "open" state
	OnAccent                       color.NRGBA // text/icons on a solid Accent fill
	Good, GoodBg                   color.NRGBA // done / success
	Warn, WarnBg                   color.NRGBA // needs attention
	Bad, BadBg                     color.NRGBA // errors, destructive actions

	Focus   color.NRGBA // keyboard focus ring
	Overlay color.NRGBA // tooltips, popovers, menus, dialogs (drawn above content)
	Scrim   color.NRGBA // translucent wash behind a modal dialog
}

// rgb is an opaque color written as 0xrrggbb.
func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

// Seed is the input to Generate: a couple of brand colors in, a full
// contrast-checked theme out.
type Seed struct {
	Name    string      // the generated theme's Name
	Accent  color.NRGBA // brand / primary color
	Neutral color.NRGBA // tints the grays; its hue matters, chroma is capped (zero value = pure gray)
	Dark    bool        // generate a dark palette
	// Contrast 0 (default) … 1 (high contrast: brighter text, stronger borders).
	Contrast float64
}

// Generate builds a complete theme from a seed (see GeneratePalette).
func Generate(s Seed) *Theme {
	return newNamed(s.Name, s.Dark, GeneratePalette(s))
}

// GeneratePalette derives every token from a seed in OKLCH: neutrals on a
// fixed lightness ladder tinted by Neutral's hue, accent and status colors at
// lightnesses that keep text readable on the background in both modes.
// Apply it to an existing theme with Derive to keep its fonts and metrics.
func GeneratePalette(s Seed) Palette {
	n := ToOKLCH(s.Neutral)
	ramp := neutralRamp{chroma: math.Min(n.C, neutralMaxChroma) * neutralChromaScale, hue: n.H}
	if s.Neutral.A == 0 {
		ramp.chroma = 0
	}
	var p Palette
	if s.Dark {
		p = darkRecipe(ramp, ToOKLCH(s.Accent), s.Contrast)
	} else {
		p = lightRecipe(ramp, ToOKLCH(s.Accent), s.Contrast)
	}
	return p.Legible(s.Dark)
}

// Neutral tint: a seed's Neutral chroma is capped, then scaled down, so the
// grays only lean toward its hue.
const (
	neutralMaxChroma   = 0.03
	neutralChromaScale = 0.6
)

// neutralRamp makes the grays of a generated palette: one hue and chroma,
// any lightness.
type neutralRamp struct {
	chroma, hue float64
}

// at returns the gray at OKLCH lightness l.
func (r neutralRamp) at(l float64) color.NRGBA { return oklch(l, r.chroma, r.hue) }

// oklch converts OKLCH components to an in-gamut sRGB color.
func oklch(l, c, h float64) color.NRGBA { return OKLCH{L: l, C: c, H: h}.RGB() }

// darkRecipe is a dark palette before Legible: every value is an OKLCH
// lightness (and, for colors, chroma), and k is the seed's Contrast, which
// spreads backgrounds and text further apart.
func darkRecipe(n neutralRamp, accent OKLCH, k float64) Palette {
	const goodHue, warnHue, badHue = 140, 85, 22
	textL := 0.74 + 0.08*k // accent and status text
	return Palette{
		Bg:           n.at(0.215 - 0.05*k),
		Panel:        n.at(0.24 - 0.04*k),
		Surface:      n.at(0.275 - 0.02*k),
		Hover:        n.at(0.305),
		Border:       n.at(0.33 + 0.12*k),
		Divider:      n.at(0.26 + 0.08*k),
		Fg:           n.at(0.91 + 0.08*k),
		Muted:        n.at(0.71 + 0.12*k),
		Dim:          n.at(0.54 + 0.14*k),
		Accent:       oklch(textL, accent.C, accent.H),
		AccentBg:     oklch(0.33, accent.C*0.35, accent.H),
		AccentBorder: oklch(0.43, accent.C*0.45, accent.H),
		OnAccent:     n.at(0.18),
		Good:         oklch(textL, 0.12, goodHue),
		GoodBg:       oklch(0.28, 0.035, goodHue),
		Warn:         oklch(textL+0.06, 0.11, warnHue),
		WarnBg:       oklch(0.29, 0.035, warnHue),
		Bad:          oklch(textL-0.06, 0.13, badHue),
		BadBg:        oklch(0.28, 0.045, badHue),
		Overlay:      n.at(0.315),
		Scrim:        WithAlpha(none, 0x8c),
	}
}

// lightRecipe is a light palette before Legible; see darkRecipe.
func lightRecipe(n neutralRamp, accent OKLCH, k float64) Palette {
	// Warn text sits at a more orange hue than its background: dark amber
	// at the background's hue reads as olive on white.
	const goodHue, warnHue, warnBgHue, badHue = 145, 70, 85, 25
	textL := 0.53 - 0.08*k // accent and status text
	return Palette{
		Bg:           n.at(0.995),
		Panel:        n.at(0.97 - 0.01*k),
		Surface:      n.at(0.985),
		Hover:        n.at(0.935),
		Border:       n.at(0.87 - 0.2*k),
		Divider:      n.at(0.925 - 0.1*k),
		Fg:           n.at(0.24 - 0.1*k),
		Muted:        n.at(0.47 - 0.12*k),
		Dim:          n.at(0.6 - 0.14*k),
		Accent:       oklch(textL, accent.C, accent.H),
		AccentBg:     oklch(0.95, accent.C*0.25, accent.H),
		AccentBorder: oklch(0.83, accent.C*0.45, accent.H),
		OnAccent:     n.at(0.99),
		Good:         oklch(textL, 0.13, goodHue),
		GoodBg:       oklch(0.95, 0.04, goodHue),
		Warn:         oklch(textL+0.05, 0.12, warnHue),
		WarnBg:       oklch(0.95, 0.05, warnBgHue),
		Bad:          oklch(textL, 0.17, badHue),
		BadBg:        oklch(0.95, 0.04, badHue),
		Overlay:      n.at(1),
		Scrim:        WithAlpha(n.at(0.2), 0x52),
	}
}

// WCAG 2.x contrast targets. Legible enforces the text targets; WithContrast
// raises them and adds the non-text one.
const (
	// ContrastText is Fg's target: WCAG AAA for normal text (7:1).
	ContrastText = 7.0
	// ContrastMuted is Muted's target, a clear step above Dim.
	ContrastMuted = 6.5
	// ContrastAA is the target of Dim, accent and status text: WCAG AA for
	// normal text (4.5:1), which also exceeds AA for large text (3:1).
	ContrastAA = 4.5
	// contrastNonText is WCAG's 3:1 for UI components and focus indicators.
	contrastNonText = 3.0
)

// Legible returns p with every text color moved in lightness, away from the
// backgrounds and only as far as needed, until it meets its contrast target
// on every surface it is drawn on: Fg, Muted and Dim on Bg, Panel, Surface,
// Hover and Overlay, and on the pressed tints of Surface and Overlay; Accent and the status colors there and on their own tinted
// backgrounds; OnAccent on Accent. Colors that already pass are unchanged,
// and hue and chroma are kept. Focus follows Accent.
//
// dark says which way text moves: lighter for a dark palette, darker for a
// light one (OnAccent the opposite way). It is the caller's statement of
// the palette's mode, normally Theme.Dark, and is not derived from Bg, so a
// deliberately unusual palette keeps its intent.
func (p Palette) Legible(dark bool) Palette {
	fit := func(c *color.NRGBA, target float64, bgs ...color.NRGBA) {
		*c = fitContrast(*c, target, dark, bgs)
	}
	plain := []color.NRGBA{p.Bg, p.Panel, p.Surface, p.Hover, p.Overlay}
	fit(&p.Fg, ContrastText, plain...)
	// Button fills tint toward Fg when hovered or pressed: text must stay
	// readable at the strongest tint too. pressed reads the Fg fitted above,
	// so Fg is fitted again against the pressed fills.
	pressed := func(c color.NRGBA) color.NRGBA { return LerpColor(c, p.Fg, PressTint) }
	// Menu items highlight like buttons.
	text := slices.Concat(plain, []color.NRGBA{pressed(p.Surface), pressed(p.Overlay)})
	fit(&p.Fg, ContrastText, text...)
	fit(&p.Muted, ContrastMuted, text...)
	fit(&p.Dim, ContrastAA, text...)
	fit(&p.Accent, ContrastAA, slices.Concat(text, []color.NRGBA{p.AccentBg, pressed(p.AccentBg)})...)
	fit(&p.Good, ContrastAA, slices.Concat(text, []color.NRGBA{p.GoodBg})...)
	fit(&p.Warn, ContrastAA, slices.Concat(text, []color.NRGBA{p.WarnBg})...)
	fit(&p.Bad, ContrastAA, slices.Concat(text, []color.NRGBA{p.BadBg, pressed(p.BadBg)})...)
	// OnAccent sits on Accent: the opposite direction from the other texts.
	p.OnAccent = fitContrast(p.OnAccent, ContrastAA, !dark, []color.NRGBA{p.Accent})
	p.Focus = p.Accent
	return p
}

// WithContrast's targets: at full level the text targets rise by the boosts
// and the border targets climb from their floors to their high values.
const (
	// highFgBoost takes Fg to 12:1, well past AAA, for low vision.
	highFgBoost = 5.0
	// highDimBoost takes Dim to Muted's usual 6.5:1; highMutedBoost keeps
	// Muted a step above it, at 8.5:1.
	highMutedBoost = 2.0
	highDimBoost   = 2.0
	// highAccentBoost takes accent text to 6:1, short of Fg so the accent
	// keeps enough chroma to read as a colour.
	highAccentBoost = 1.5
	// borderContrast is the floor of Border and AccentBorder at any level:
	// visible but quiet. At full level they reach contrastNonText.
	borderContrast = 1.6
	// dividerContrast is the floor of Divider, quieter than a border.
	// dividerHighContrast is its full-level target: a divider separates
	// regions rather than marking a control, so it need not reach 3:1.
	dividerContrast     = 1.3
	dividerHighContrast = 2.0
)

// WithContrast raises a palette's contrast for any theme, generated or
// hand-made: level 0 returns p unchanged, 1 is high contrast (text toward
// AAA, borders and the focus ring at least 3:1 against the background, as
// WCAG asks of non-text UI). Levels above 1 count as 1. dark is as for
// Legible.
func (p Palette) WithContrast(dark bool, level float64) Palette {
	if level <= 0 {
		return p
	}
	k := min(level, 1)
	fit := func(c *color.NRGBA, target float64, bgs ...color.NRGBA) {
		*c = fitContrast(*c, target, dark, bgs)
	}
	plain := []color.NRGBA{p.Bg, p.Panel, p.Surface, p.Hover, p.Overlay}
	fit(&p.Fg, ContrastText+highFgBoost*k, plain...)
	fit(&p.Muted, ContrastMuted+highMutedBoost*k, plain...)
	fit(&p.Dim, ContrastAA+highDimBoost*k, plain...)
	fit(&p.Border, borderContrast+(contrastNonText-borderContrast)*k, p.Bg, p.Panel, p.Surface)
	fit(&p.Divider, dividerContrast+(dividerHighContrast-dividerContrast)*k, p.Bg, p.Panel)
	fit(&p.AccentBorder, borderContrast+(contrastNonText-borderContrast)*k, p.Bg, p.Surface)
	fit(&p.Accent, ContrastAA+highAccentBoost*k, slices.Concat(plain, []color.NRGBA{p.AccentBg})...)
	fit(&p.Focus, contrastNonText, p.Bg, p.Panel, p.Surface)
	return p.Legible(dark)
}

// Contrast fitting walks OKLCH lightness in fitStep increments, at most
// fitMaxSteps of them (enough to cross the whole 0..1 range).
const (
	fitStep     = 0.005
	fitMaxSteps = 200
)

// fitContrast steps c's OKLCH lightness up (lighter) or down until it reaches
// target against every bg, or hits white/black.
func fitContrast(c color.NRGBA, target float64, lighter bool, bgs []color.NRGBA) color.NRGBA {
	worst := func(c color.NRGBA) float64 {
		w := 21.0 // the largest possible contrast ratio
		for _, b := range bgs {
			w = math.Min(w, Contrast(c, b))
		}
		return w
	}
	if worst(c) >= target {
		return c
	}
	o := ToOKLCH(c)
	for i := 0; i < fitMaxSteps && worst(c) < target; i++ {
		if lighter {
			o.L = math.Min(1, o.L+fitStep)
		} else {
			o.L = math.Max(0, o.L-fitStep)
		}
		c = o.RGB()
		if o.L == 0 || o.L == 1 {
			break
		}
	}
	return c
}

// PresetSeeds returns the seeds of the generated presets, in display order.
// Each call returns a fresh slice.
func PresetSeeds() []Seed {
	return []Seed{
		{Name: "Light", Accent: rgb(0x2f6fd0), Neutral: rgb(0x6b7280)},
		{Name: "Midnight", Dark: true, Accent: rgb(0x8b7cf6), Neutral: rgb(0x4b5a8a)},
		{Name: "Forest", Dark: true, Accent: rgb(0x4fbf8a), Neutral: rgb(0x4a6b58)},
		{Name: "Paper", Accent: rgb(0xd9622b), Neutral: rgb(0x8a7a62)},
		{Name: "Rose", Accent: rgb(0xd6457a), Neutral: rgb(0x8a6070)},
		{Name: "High contrast", Dark: true, Accent: rgb(0x5cb6ff), Contrast: 1},
	}
}

// Presets returns the bundled themes in display order (Dark first, then
// the seeded ones, then Black, the Catppuccin flavors and Pink). They
// share one font shaper, so switching between them keeps text caches warm.
func Presets() []*Theme {
	out := []*Theme{Dark()}
	for _, s := range PresetSeeds() {
		out = append(out, out[0].Derive(func(t *Theme) { t.Palette, t.Name, t.Dark = GeneratePalette(s), s.Name, s.Dark }))
	}
	for _, n := range namedPalettes() {
		out = append(out, out[0].Derive(func(t *Theme) { t.Palette, t.Name, t.Dark = n.p, n.name, n.dark }))
	}
	return out
}
