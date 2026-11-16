// Package kit is a small component layer over Gio for dense, Zed-style desktop tools.
//
// Rules the components rely on (see SKILL.md):
//   - Handle input before drawing, and read hover/press state only inside a
//     component's own Layout callback, never before it.
//   - Any state change during a frame must schedule a redraw. kit.Clickable does
//     this for clicks; use kit.Notify(gtx) for anything else.
//   - App state is owned by the UI goroutine. Background work hands results back
//     with App.Post.
package kit

import (
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Palette holds the semantic color tokens every component draws from.
// Components never use raw colors: add a token instead.
type Palette struct {
	Bg      color.NRGBA // window background
	Panel   color.NRGBA // bars, sidebars, cards, row hover
	Surface color.NRGBA // buttons, inputs, selected nav item
	Hover   color.NRGBA // hovered nav item / control
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

// TypeScale is the theme's text sizes. Components read sizes from here, so a
// theme can be denser or roomier without touching components.
type TypeScale struct {
	Caption, Small, Body, Text, Stat, Title unit.Sp
}

// Default type scale (also exposed as the Size* constants).
const (
	SizeCaption unit.Sp = 11 // uppercase section labels, column headers
	SizeSmall   unit.Sp = 12
	SizeBody    unit.Sp = 13
	SizeText    unit.Sp = 14
	SizeStat    unit.Sp = 20
	SizeTitle   unit.Sp = 22
)

var DefaultType = TypeScale{SizeCaption, SizeSmall, SizeBody, SizeText, SizeStat, SizeTitle}

type Theme struct {
	Palette
	Name string
	Dark bool // the palette is dark (for apps choosing images/icons)
	Type TypeScale

	// Radius is the corner radius of controls (buttons, fields, selects).
	// Everything else derives from it: RadiusSm (checkboxes, menu rows, tabs),
	// RadiusLg (cards, menus, dialogs) and RadiusFull (badges). 0 is square
	// everywhere; a huge value makes pill-shaped controls.
	Radius  unit.Dp
	Stroke  unit.Dp // border width of controls, fields and cards (1 hairline)
	Ring    unit.Dp // keyboard focus ring width (2; thicker is easier to see)
	Density float32 // spacing multiplier: 0.8 compact, 1 regular, 1.2 comfortable
	// Controls is the height class of buttons, fields, selects, tabs and menu
	// rows; they line up with each other at every size.
	Controls ControlSize
	// Elevation scales the shadow under menus, popovers and dialogs:
	// 0 flat (border only), 1 soft, 2 strong.
	Elevation float32
	// HeadingWeight is the weight of page headings.
	HeadingWeight font.Weight
	// Motion scales every animation duration. 0 disables animation (reduced
	// motion); continuous animations (spinners) then draw a static frame.
	Motion float32
	// DisabledAlpha is the opacity of controls inside Theme.Disable.
	DisabledAlpha float32

	UI, Mono font.Typeface // bundled typefaces for UI text and code/logs
	Fonts    string        // name of the font set (UseFonts); "Go" by default

	Shaper *text.Shaper
	mat    *material.Theme
}

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

// Dark is the default theme: a muted One Dark–like palette.
func Dark() *Theme {
	p := Palette{
		Bg: rgb(0x1e2124), Panel: rgb(0x24282c), Surface: rgb(0x2b3035), Hover: rgb(0x30363c),
		Border: rgb(0x363c42), Divider: rgb(0x2a2f34),
		Fg: rgb(0xdce0e5), Muted: rgb(0x959ca4), Dim: rgb(0x646b73),
		Accent: rgb(0x74ade8), AccentBg: rgb(0x263b50), AccentBorder: rgb(0x34506c), OnAccent: rgb(0x14181c),
		Good: rgb(0xa1c181), GoodBg: rgb(0x2a3527), Warn: rgb(0xdec184), WarnBg: rgb(0x3a3424),
		Bad: rgb(0xd07277), BadBg: rgb(0x3e2729),
		Focus: rgb(0x74ade8), Overlay: rgb(0x31373d), Scrim: color.NRGBA{A: 0x8c},
	}
	t := New(p.Legible(true))
	t.Name, t.Dark = "Dark", true
	return t
}

// Light is the default light theme, generated from the same accent.
func Light() *Theme {
	return Generate(PresetSeeds[0])
}

// New builds a theme from a palette with default metrics.
func New(p Palette) *Theme {
	// Bundled fonts only: system fallback makes rendering machine-dependent and
	// hides missing glyphs on the dev box that show as tofu elsewhere.
	sh := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	t := &Theme{Palette: p, Type: DefaultType, Radius: 6, Stroke: 1, Ring: 2, Density: 1, Motion: 1,
		Elevation: 1, HeadingWeight: font.Bold, DisabledAlpha: 0.45,
		UI: "Go", Mono: "Go Mono", Fonts: "Go", Shaper: sh}
	t.syncMaterial()
	return t
}

func (t *Theme) syncMaterial() {
	m := material.NewTheme()
	if t.mat != nil {
		cp := *t.mat
		m = &cp
	}
	m.Shaper = t.Shaper
	m.Palette = material.Palette{Bg: t.Bg, Fg: t.Fg, ContrastBg: t.Accent, ContrastFg: t.OnAccent}
	m.Face = t.UI
	t.mat = m
}

// Derive returns a copy of t changed by f, sharing the font shaper. Use it to
// restyle a subtree (a danger zone, a sidebar with its own tint, a compact
// table) without touching components:
//
//	danger := th.Derive(func(t *kit.Theme) { t.Accent, t.AccentBg = t.Bad, t.BadBg })
func (t *Theme) Derive(f func(t *Theme)) *Theme {
	c := *t
	f(&c)
	c.syncMaterial()
	return &c
}

// ControlSize is the height class of controls (Theme.Controls).
type ControlSize int

const (
	ControlMedium ControlSize = iota // the default
	ControlSmall
	ControlLarge
)

// control scales a control's inner padding by density and Controls, so
// buttons, fields and selects keep the same height as each other.
func (t *Theme) control(v unit.Dp) unit.Dp {
	f := float32(1)
	switch t.Controls {
	case ControlSmall:
		f = 0.55
	case ControlLarge:
		f = 1.5
	}
	return t.Space(v) * unit.Dp(f)
}

// RadiusSm is the corner of small parts: checkboxes, menu rows, tabs.
func (t *Theme) RadiusSm() unit.Dp { return min(t.Radius*2/3, 8) }

// RadiusLg is the corner of containers: cards, menus, popovers, dialogs.
// It is capped, so pill-shaped controls don't make capsule-shaped cards.
func (t *Theme) RadiusLg() unit.Dp { return min(t.Radius*1.4, 16) }

// RadiusFull is the corner of badges and pills: fully round, unless the
// theme is square (Radius 0).
func (t *Theme) RadiusFull() unit.Dp {
	if t.Radius == 0 {
		return 0
	}
	return 999
}

// Styled fills a Surface's border and ring widths from the theme. Components
// build their surfaces through it, so Stroke and Ring apply everywhere.
func (t *Theme) Styled(s Surface) Surface {
	if s.Stroke == 0 {
		s.Stroke = t.Stroke
	}
	if s.Ring == 0 {
		s.Ring = t.Ring
	}
	return s
}

// Fonts is a set of bundled typefaces: a UI face and a code face, with the
// font files to shape them. See package kit/fonts for the curated sets.
type Fonts struct {
	Name     string
	UI, Mono font.Typeface
	Faces    []font.FontFace
}

// UseFonts switches t to the font set f. It builds a new shaper (call it
// when the choice changes, not every frame); Go's fonts stay in the
// collection as a fallback for glyphs f lacks.
func (t *Theme) UseFonts(f Fonts) {
	faces := append(append([]font.FontFace(nil), f.Faces...), gofont.Collection()...)
	t.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(faces))
	t.UI, t.Mono, t.Fonts = f.UI, f.Mono, f.Name
	t.syncMaterial()
}

// Scaled returns the type scale with every size multiplied by f.
func (s TypeScale) Scaled(f float32) TypeScale {
	m := unit.Sp(f)
	return TypeScale{s.Caption * m, s.Small * m, s.Body * m, s.Text * m, s.Stat * m, s.Title * m}
}

// Material exposes the underlying material theme for widgets kit doesn't wrap.
func (t *Theme) Material() *material.Theme { return t.mat }

// Space scales a spacing value by the theme's density.
func (t *Theme) Space(v unit.Dp) unit.Dp {
	if t.Density == 0 {
		return v
	}
	return unit.Dp(float32(v) * t.Density)
}

// Dur scales an animation duration by the theme's Motion (0 → no animation).
func (t *Theme) Dur(d time.Duration) time.Duration {
	return time.Duration(float64(d) * float64(t.Motion))
}

// Standard durations (before Motion scaling).
const (
	DurFast   = 90 * time.Millisecond  // hover and press tints
	DurNormal = 160 * time.Millisecond // toggles, indicators, page transitions
	DurSlow   = 260 * time.Millisecond // larger movement
)

var (
	Regular  = font.Normal
	Medium   = font.Medium
	SemiBold = font.SemiBold
	Bold     = font.Bold
)
