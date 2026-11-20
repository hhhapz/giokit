package kit

import (
	"image/color"
	"time"

	"github.com/hhhapz/giokit/gio/font"
	"github.com/hhhapz/giokit/gio/font/gofont"
	"github.com/hhhapz/giokit/gio/text"
	"github.com/hhhapz/giokit/gio/unit"
)

// TypeScale is the theme's text sizes. Components read sizes from here, so a
// theme can be denser or roomier without touching components.
type TypeScale struct {
	Caption unit.Sp // uppercase section labels, column headers
	Small   unit.Sp // secondary text: hints, captions under fields
	Body    unit.Sp // controls and most text
	Text    unit.Sp // reading text and empty-state titles
	Stat    unit.Sp // large figures (Stat cards)
	Title   unit.Sp // dialog and page titles
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

// DefaultType returns giokit's type scale, the Size* constants. Each call
// returns a fresh value.
func DefaultType() TypeScale {
	return TypeScale{
		Caption: SizeCaption,
		Small:   SizeSmall,
		Body:    SizeBody,
		Text:    SizeText,
		Stat:    SizeStat,
		Title:   SizeTitle,
	}
}

// Theme is everything a component reads to draw: colors, text sizes and
// faces, spacing, shape, motion and the strings it shows or speaks.
//
// Build a theme with New, Dark, Light, Generate or ThemeFromTokens; the zero
// Theme has no shaper and cannot draw text. Fields may be written directly,
// between frames, and take effect from the next frame. A theme belongs to
// the UI goroutine. Copies made by Derive (or by assignment) share its
// Shaper, which is not safe for concurrent use, so a copy is not
// permission to draw on another goroutine.
type Theme struct {
	Palette           // the colour tokens
	Name    string    // the theme's display name ("Dark", a preset's name)
	Dark    bool      // the palette is dark (for apps choosing images/icons)
	Type    TypeScale // text sizes; scale them all with Type.Scaled

	// Radius is the corner radius of controls (buttons, fields, selects).
	// Everything else derives from it: RadiusSm (checkboxes, menu rows, tabs),
	// RadiusLg (cards, menus, dialogs) and RadiusFull (badges). 0 is square
	// everywhere; a huge value makes pill-shaped controls.
	Radius unit.Dp
	// Stroke is the border width of controls, fields and cards (1 is a
	// hairline). Ring is the keyboard focus ring width (2; thicker is
	// easier to see). Styled applies both to a Surface that leaves its own
	// widths 0.
	Stroke, Ring unit.Dp
	// Density multiplies spacing (Space): 0.8 compact, 1 regular, 1.2
	// comfortable. 0 means 1, so a theme built without it is regular.
	Density float32
	// Controls is the height class of buttons, fields, selects, tabs and menu
	// rows; they line up with each other at every size.
	Controls ControlSize
	// Elevation scales the shadow under menus, popovers and dialogs:
	// 0 flat (border only), 1 soft, 2 strong.
	Elevation float32
	// HeadingWeight is the weight of page headings.
	HeadingWeight font.Weight
	// Motion scales every animation duration (Dur). Unlike Density, 0 is
	// meaningful: it disables animation (reduced motion), and continuous
	// animations (spinners) then draw a static frame.
	Motion float32
	// DisabledAlpha is the opacity of controls inside Theme.Disable.
	DisabledAlpha float32

	Metrics         // fixed sizes: icons, controls, overlay widths
	Strings Strings // text kit shows or speaks (English by default)

	UI, Mono font.Typeface // bundled typefaces for UI text and code/logs
	Fonts    string        // name of the font set (UseFonts); "Go" by default

	// Shaper shapes all text, from bundled fonts only. UseFonts replaces it.
	Shaper *text.Shaper
}

// New's defaults for the fields a palette does not set.
const (
	defaultRadius        unit.Dp = 6
	defaultStroke        unit.Dp = 1
	defaultRing          unit.Dp = 2
	defaultDensity               = 1
	defaultMotion                = 1
	defaultElevation             = 1
	defaultDisabledAlpha         = 0.45
	defaultFontSet               = "Go"
)

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
	const dark = true
	return newNamed("Dark", dark, p.Legible(dark))
}

// Light is the default light theme, generated from the same accent.
func Light() *Theme {
	return Generate(PresetSeeds()[0])
}

// New builds a theme from a palette, with giokit's default type scale,
// metrics, shape, motion and English strings, and the bundled Go fonts.
// It uses p as given; Palette.Legible fixes text contrast first if needed.
//
// New creates a text shaper, which loads fonts and caches shaped text: it
// is too costly to call per frame. Build themes once and restyle with
// Derive, which shares the shaper.
func New(p Palette) *Theme {
	// Bundled fonts only: system fallback makes rendering machine-dependent and
	// hides missing glyphs on the dev box that show as tofu elsewhere.
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	return &Theme{
		Palette:       p,
		Type:          DefaultType(),
		Radius:        defaultRadius,
		Stroke:        defaultStroke,
		Ring:          defaultRing,
		Density:       defaultDensity,
		Motion:        defaultMotion,
		Elevation:     defaultElevation,
		HeadingWeight: Bold,
		DisabledAlpha: defaultDisabledAlpha,
		Metrics:       DefaultMetrics(),
		Strings:       English(),
		UI:            "Go",
		Mono:          "Go Mono",
		Fonts:         defaultFontSet,
		Shaper:        shaper,
	}
}

// newNamed is New for a named palette of known mode.
func newNamed(name string, dark bool, p Palette) *Theme {
	t := New(p)
	t.Name, t.Dark = name, dark
	return t
}

// Derive returns a copy of t changed by f; t is unchanged. Use it to
// restyle a subtree (a danger zone, a sidebar with its own tint, a compact
// table) without touching components:
//
//	danger := th.Derive(func(t *kit.Theme) { t.Accent, t.AccentBg = t.Bad, t.BadBg })
//
// Value fields (colors, sizes, Strings, Metrics) are the copy's own. The Shaper is shared, which keeps text caches warm;
// calling UseFonts on the copy gives it a new shaper without touching t.
func (t *Theme) Derive(f func(t *Theme)) *Theme {
	c := *t
	f(&c)
	return &c
}

// ControlSize is the height class of controls: Theme.Controls for the whole
// theme, a component's Size for one instance.
type ControlSize int

// Control sizes. ControlDefault is the zero value, so a component without a
// Size follows its theme, and a theme without Controls is medium. Any value
// other than these is treated as medium.
const (
	ControlDefault ControlSize = iota // on a component: the theme's size; on a theme: medium
	ControlSmall
	ControlMedium
	ControlLarge
)

// Padding multipliers of the small and large control sizes; medium is 1.
const (
	controlSmallScale = 0.55
	controlLargeScale = 1.5
)

// ControlPad scales a control's inner padding v by density and size
// (ControlDefault: the theme's Controls), so buttons, fields and selects of
// one size share a height.
func (t *Theme) ControlPad(size ControlSize, v unit.Dp) unit.Dp {
	if size == ControlDefault {
		size = t.Controls
	}
	scale := unit.Dp(1)
	switch size {
	case ControlSmall:
		scale = controlSmallScale
	case ControlLarge:
		scale = controlLargeScale
	}
	return t.Space(v) * scale
}

// control is ControlPad at the theme's size.
func (t *Theme) control(v unit.Dp) unit.Dp { return t.ControlPad(ControlDefault, v) }

// Caps on the derived radii. They are design limits, not geometry: they
// keep a pill-shaped theme (huge Radius) from rounding small parts into
// dots and containers into capsules.
const (
	radiusSmMax unit.Dp = 8
	radiusLgMax unit.Dp = 16
)

// radiusRound is larger than any half side a control has. Surface and
// Border clamp a radius to half the shorter side, so it draws fully round.
const radiusRound unit.Dp = 999

// RadiusSm is the corner of small parts: checkboxes, menu rows, tabs. It is
// two thirds of Radius, at most 8dp.
func (t *Theme) RadiusSm() unit.Dp { return min(t.Radius*2/3, radiusSmMax) }

// RadiusLg is the corner of containers: cards, menus, popovers, dialogs. It
// is 1.4 times Radius, at most 16dp, so pill-shaped controls don't make
// capsule-shaped cards.
func (t *Theme) RadiusLg() unit.Dp { return min(t.Radius*1.4, radiusLgMax) }

// RadiusFull is the corner of badges and pills: fully round, unless the
// theme is square (Radius 0).
func (t *Theme) RadiusFull() unit.Dp {
	if t.Radius == 0 {
		return 0
	}
	return radiusRound
}

// Styled fills a Surface's border and ring widths from the theme's Stroke
// and Ring where the Surface leaves them 0. Components build their surfaces
// through it, so Stroke and Ring apply everywhere.
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
	Name     string          // recorded in Theme.Fonts and theme tokens
	UI, Mono font.Typeface   // faces for UI text and for code and logs
	Faces    []font.FontFace // the font files; UseFonts does not modify them
}

// UseFonts switches t to the font set f, in place. It builds a new shaper
// (call it when the choice changes, not every frame); Go's fonts stay in
// the collection as a fallback for glyphs f lacks. Themes derived from t
// earlier keep the old shaper.
func (t *Theme) UseFonts(f Fonts) {
	faces := append(append([]font.FontFace(nil), f.Faces...), gofont.Collection()...)
	t.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(faces))
	t.UI, t.Mono, t.Fonts = f.UI, f.Mono, f.Name
}

// Scaled returns the type scale with every size multiplied by f.
func (s TypeScale) Scaled(f float32) TypeScale {
	m := unit.Sp(f)
	return TypeScale{
		Caption: s.Caption * m,
		Small:   s.Small * m,
		Body:    s.Body * m,
		Text:    s.Text * m,
		Stat:    s.Stat * m,
		Title:   s.Title * m,
	}
}

// Space scales a spacing value by the theme's Density (0 counts as 1).
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

// Font weights for Theme.Text and Theme.HeadingWeight, so callers need not
// import gioui.org/font.
const (
	Regular  = font.Normal
	Medium   = font.Medium
	SemiBold = font.SemiBold
	Bold     = font.Bold
)
