package kit

import "image/color"

// Hand-made themes from fixed colors, as opposed to the seeded ones
// (PresetSeeds). Each maps its scheme's own colors onto the palette roles,
// then Legible nudges any text color that misses its contrast target.

// Catppuccin flavor colors, from catppuccin.com/palette: the ones the
// palette roles use.
type catppuccin struct {
	name                                              string
	dark                                              bool
	red, yellow, green, lavender                      uint32
	text, subtext0, overlay1                          uint32
	surface2, surface1, surface0, base, mantle, crust uint32
}

var (
	latte = catppuccin{name: "Catppuccin Latte",
		red: 0xd20f39, yellow: 0xdf8e1d, green: 0x40a02b, lavender: 0x7287fd,
		text: 0x4c4f69, subtext0: 0x6c6f85, overlay1: 0x8c8fa1,
		surface2: 0xacb0be, surface1: 0xbcc0cc, surface0: 0xccd0da, base: 0xeff1f5, mantle: 0xe6e9ef, crust: 0xdce0e8}
	frappe = catppuccin{name: "Catppuccin Frappé", dark: true,
		red: 0xe78284, yellow: 0xe5c890, green: 0xa6d189, lavender: 0xbabbf1,
		text: 0xc6d0f5, subtext0: 0xa5adce, overlay1: 0x838ba7,
		surface2: 0x626880, surface1: 0x51576d, surface0: 0x414559, base: 0x303446, mantle: 0x292c3c, crust: 0x232634}
	macchiato = catppuccin{name: "Catppuccin Macchiato", dark: true,
		red: 0xed8796, yellow: 0xeed49f, green: 0xa6da95, lavender: 0xb7bdf8,
		text: 0xcad3f5, subtext0: 0xa5adcb, overlay1: 0x8087a2,
		surface2: 0x5b6078, surface1: 0x494d64, surface0: 0x363a4f, base: 0x24273a, mantle: 0x1e2030, crust: 0x181926}
	mocha = catppuccin{name: "Catppuccin Mocha", dark: true,
		red: 0xf38ba8, yellow: 0xf9e2af, green: 0xa6e3a1, lavender: 0xb4befe,
		text: 0xcdd6f4, subtext0: 0xa6adc8, overlay1: 0x7f849c,
		surface2: 0x585b70, surface1: 0x45475a, surface0: 0x313244, base: 0x1e1e2e, mantle: 0x181825, crust: 0x11111b}
)

// tint mixes c into bg: the soft background behind accent and status text.
func tint(bg, c color.NRGBA, t float32) color.NRGBA { return LerpColor(bg, c, t) }

func (f catppuccin) palette() Palette {
	base, lav := rgb(f.base), rgb(f.lavender)
	p := Palette{
		Bg: base, Panel: rgb(f.mantle), Surface: rgb(f.surface0), Hover: rgb(f.surface1),
		Border: rgb(f.surface1), Divider: rgb(f.surface0),
		Fg: rgb(f.text), Muted: rgb(f.subtext0), Dim: rgb(f.overlay1),
		Accent: lav, AccentBg: tint(base, lav, 0.22), AccentBorder: tint(base, lav, 0.45), OnAccent: rgb(f.crust),
		Good: rgb(f.green), GoodBg: tint(base, rgb(f.green), 0.16),
		Warn: rgb(f.yellow), WarnBg: tint(base, rgb(f.yellow), 0.16),
		Bad: rgb(f.red), BadBg: tint(base, rgb(f.red), 0.16),
		Focus: lav, Overlay: rgb(f.surface0), Scrim: WithAlpha(rgb(f.crust), 0x8c),
	}
	if !f.dark { // light surfaces sit above the page, lighter than it
		p.Surface, p.Overlay = rgb(0xffffff), rgb(0xffffff)
		p.Hover, p.OnAccent = rgb(f.crust), rgb(0xffffff)
		p.Border = rgb(f.surface2)
	}
	return p.Legible(f.dark)
}

// CatppuccinLatte is Catppuccin's light flavor, with its lavender accent.
func CatppuccinLatte() *Theme { return newNamed(latte.name, latte.dark, latte.palette()) }

// CatppuccinFrappe is Catppuccin's lightest dark flavor, with its lavender
// accent.
func CatppuccinFrappe() *Theme { return newNamed(frappe.name, frappe.dark, frappe.palette()) }

// CatppuccinMacchiato is Catppuccin's middle dark flavor, with its
// lavender accent.
func CatppuccinMacchiato() *Theme {
	return newNamed(macchiato.name, macchiato.dark, macchiato.palette())
}

// CatppuccinMocha is Catppuccin's darkest flavor, with its lavender accent.
func CatppuccinMocha() *Theme { return newNamed(mocha.name, mocha.dark, mocha.palette()) }

// blackPalette is Dark on true black, for OLED screens: surfaces are
// near-black steps so cards and inputs still separate from the page.
func blackPalette() Palette {
	p := Dark().Palette
	p.Bg, p.Panel, p.Surface, p.Hover = rgb(0x000000), rgb(0x0a0a0a), rgb(0x161616), rgb(0x1f1f1f)
	p.Border, p.Divider, p.Overlay = rgb(0x2c2c2c), rgb(0x1a1a1a), rgb(0x1c1c1c)
	p.Fg, p.Muted, p.Dim = rgb(0xe6e6e6), rgb(0x9a9a9a), rgb(0x6a6a6a)
	p.AccentBg, p.GoodBg, p.WarnBg, p.BadBg = tint(p.Bg, p.Accent, 0.2), tint(p.Bg, p.Good, 0.14), tint(p.Bg, p.Warn, 0.14), tint(p.Bg, p.Bad, 0.14)
	p.AccentBorder, p.OnAccent = tint(p.Bg, p.Accent, 0.4), rgb(0x000000)
	return p.Legible(true)
}

// Black is the default dark theme on true black.
func Black() *Theme { return newNamed("Black", true, blackPalette()) }

// pinkPalette is a soft light theme: blush backgrounds, a rose accent and
// plum text.
func pinkPalette() Palette {
	bg, accent := rgb(0xfff4f8), rgb(0xd94f8f)
	good, warn, bad := rgb(0x2f8a55), rgb(0xb7731a), rgb(0xc8344e)
	p := Palette{
		Bg: bg, Panel: rgb(0xffe8f1), Surface: rgb(0xfffafc), Hover: rgb(0xffdbe9),
		Border: rgb(0xf0bcd1), Divider: rgb(0xf8d5e3),
		Fg: rgb(0x4a2236), Muted: rgb(0x85506a), Dim: rgb(0xa8798f),
		Accent: accent, AccentBg: tint(bg, accent, 0.14), AccentBorder: tint(bg, accent, 0.4), OnAccent: rgb(0xffffff),
		Good: good, GoodBg: tint(bg, good, 0.12), Warn: warn, WarnBg: tint(bg, warn, 0.14), Bad: bad, BadBg: tint(bg, bad, 0.12),
		Focus: accent, Overlay: rgb(0xfffafc), Scrim: WithAlpha(rgb(0x4a2236), 0x66),
	}
	return p.Legible(false)
}

// Pink is a soft pink light theme.
func Pink() *Theme { return newNamed("Pink", false, pinkPalette()) }

// namedPalettes are the hand-made themes Presets lists after the seeded
// ones.
func namedPalettes() []struct {
	name string
	dark bool
	p    Palette
} {
	return []struct {
		name string
		dark bool
		p    Palette
	}{
		{"Black", true, blackPalette()},
		{latte.name, false, latte.palette()},
		{frappe.name, true, frappe.palette()},
		{macchiato.name, true, macchiato.palette()},
		{mocha.name, true, mocha.palette()},
		{"Pink", false, pinkPalette()},
	}
}
