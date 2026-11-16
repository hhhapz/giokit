package main

import (
	"image"
	"image/color"
	"io"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/fonts"
)

type themeState struct {
	presets  []*kit.Theme
	preset   int // index into presets, or -1 for the custom theme
	presetBt kit.Pool[kit.Clickable]
	density  kit.Tabs
	dens     int
	motion   kit.Clickable
	motionOn bool
	custom   kit.Clickable
	hue      kit.Slider
	lastHue  float32
	darkSeed kit.Clickable
	seedDark bool
	copyBt   kit.Clickable
	themeLst kit.List
	knobs    []*knob

	prevSave, prevSolid kit.Clickable
	prevName            widget.Editor
	prevSel             kit.Select
	prevSelI            int
	prevCheck           kit.Clickable
	prevChecked         bool
}

// knob is one theme setting shown as a labelled segmented control.
type knob struct {
	id, label string
	names     []string
	tabs      kit.Tabs
	sel       int
}

func (k *knob) is(name string) bool { return k.names[k.sel] == name }

// knob values, in the order of each knob's names.
var (
	cornerVals    = []unit.Dp{0, 3, 6, 10, 999}
	strokeVals    = []unit.Dp{1, 1.5, 2}
	ringVals      = []unit.Dp{1.5, 2, 3}
	textVals      = []float32{0.9, 1, 1.15, 1.3}
	headingVals   = []font.Weight{font.Medium, font.SemiBold, font.Bold}
	elevationVals = []float32{0, 1, 2}
	controlVals   = []kit.ControlSize{kit.ControlSmall, kit.ControlMedium, kit.ControlLarge}
)

func (g *gallery) knob(id string) *knob {
	for _, k := range g.knobs {
		if k.id == id {
			return k
		}
	}
	panic("no knob " + id)
}

var densityNames = []string{"Compact", "Regular", "Comfortable"}
var densityVals = []float32{0.75, 1, 1.25}

func (g *gallery) initTheme() {
	g.presets = kit.Presets()
	g.th = g.presets[0]
	g.dens, g.motionOn, g.seedDark = 1, true, true
	g.density.Style = kit.TabsSegmented
	g.hue.Value, g.lastHue = 0.6, 0.6
	g.themeLst.Axis = layout.Vertical
	var fontNames []string
	for _, f := range fonts.All() {
		fontNames = append(fontNames, f.Name)
	}
	add := func(id, label string, sel int, names ...string) {
		k := &knob{id: id, label: label, names: names, sel: sel}
		k.tabs.Style = kit.TabsSegmented
		g.knobs = append(g.knobs, k)
	}
	add("corners", "Corners", 2, "Sharp", "Subtle", "Default", "Round", "Pill")
	add("stroke", "Borders", 0, "Hairline", "Medium", "Bold")
	add("ring", "Focus ring", 1, "Thin", "Default", "Thick")
	add("fonts", "Font", 0, fontNames...)
	add("text", "Text size", 1, "90%", "100%", "115%", "130%")
	add("heading", "Headings", 2, "Medium", "Semibold", "Bold")
	add("elevation", "Shadows", 1, "Flat", "Soft", "Strong")
	add("contrast", "Contrast", 0, "Standard", "High")
	add("controls", "Controls", 1, "Small", "Medium", "Large")
}

// applyTheme rebuilds g.th from the chosen palette plus density and motion.
// Derive keeps the font shaper, so switching is cheap.
func (g *gallery) applyTheme() {
	base := g.th
	var p kit.Palette
	name, dark := "Custom", g.seedDark
	if g.preset >= 0 {
		pr := g.presets[g.preset]
		p, name, dark = pr.Palette, pr.Name, pr.Dark
	} else {
		p = kit.GeneratePalette(g.customSeed())
	}
	g.th = base.Derive(func(t *kit.Theme) {
		t.Palette, t.Name, t.Dark = p, name, dark
		t.Density = densityVals[g.dens]
		t.Motion = 0
		if g.motionOn {
			t.Motion = 1
		}
		if g.knobs == nil {
			return
		}
		if g.knob("contrast").is("High") {
			t.Palette = t.Palette.WithContrast(dark, 1)
		}
		t.Radius = cornerVals[g.knob("corners").sel]
		t.Stroke = strokeVals[g.knob("stroke").sel]
		t.Ring = ringVals[g.knob("ring").sel]
		t.Type = kit.DefaultType.Scaled(textVals[g.knob("text").sel])
		t.HeadingWeight = headingVals[g.knob("heading").sel]
		t.Elevation = elevationVals[g.knob("elevation").sel]
		t.Controls = controlVals[g.knob("controls").sel]
	})
	if g.knobs == nil {
		return
	}
	if set := fonts.All()[g.knob("fonts").sel]; g.th.Fonts != set.Name {
		g.th.UseFonts(set) // a new shaper only when the font set changes
	}
}

func (g *gallery) customSeed() kit.Seed {
	h := float64(g.hue.Value) * 360
	return kit.Seed{Name: "Custom", Dark: g.seedDark,
		Accent:  kit.OKLCH{L: 0.65, C: 0.16, H: h}.RGB(),
		Neutral: kit.OKLCH{L: 0.5, C: 0.03, H: h}.RGB()}
}

func (g *gallery) nextTheme() {
	g.preset = (g.preset + 1) % len(g.presets)
	g.applyTheme()
}

func (g *gallery) handleTheme(gtx C) {
	changed := false
	for i := range g.presets {
		if g.presetBt.Get(g.presets[i].Name).Clicked(gtx) {
			g.preset, changed = i, true
		}
	}
	if i, ok := g.density.Clicked(gtx); ok {
		g.dens, changed = i, true
	}
	if g.motion.Clicked(gtx) {
		g.motionOn, changed = !g.motionOn, true
	}
	if g.custom.Clicked(gtx) {
		g.preset, changed = -1, true
	}
	if g.darkSeed.Clicked(gtx) {
		g.seedDark, changed = !g.seedDark, true
		if g.preset != -1 {
			g.preset = -1
		}
	}
	// The slider handles its input during layout; compare with last frame.
	if g.hue.Value != g.lastHue {
		g.lastHue, g.preset, changed = g.hue.Value, -1, true
	}
	for _, k := range g.knobs {
		if i, ok := k.tabs.Clicked(gtx); ok && i != k.sel {
			k.sel, changed = i, true
		}
	}
	g.prevSave.Clicked(gtx)
	g.prevSolid.Clicked(gtx)
	if i, ok := g.prevSel.Changed(gtx); ok {
		g.prevSelI = i
	}
	if g.prevCheck.Clicked(gtx) {
		g.prevChecked = !g.prevChecked
	}
	if g.copyBt.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(string(g.th.Tokens())))})
		g.status = "Theme tokens (DTCG JSON) copied to the clipboard"
	}
	if changed {
		g.applyTheme()
	}
}

func swatch(gtx C, th *kit.Theme, name string, c color.NRGBA) D {
	return layout.Inset{Bottom: 8, Right: 8}.Layout(gtx, func(gtx C) D {
		return kit.Row(gtx,
			kit.Rigid(func(gtx C) D {
				sz := image.Pt(gtx.Dp(28), gtx.Dp(28))
				r := gtx.Dp(6)
				paint.FillShape(gtx.Ops, c, clip.UniformRRect(image.Rectangle{Max: sz}, r).Op(gtx.Ops))
				kit.Border(gtx, image.Rectangle{Max: sz}, r, 1, th.Border)
				return D{Size: sz}
			}),
			kit.Rigid(kit.Gap(8, 0)),
			kit.Rigid(kit.Fixed(96, func(gtx C) D {
				return kit.Col(gtx,
					kit.Rigid(func(gtx C) D { return th.Text(gtx, name, th.Type.Small, th.Fg, kit.Medium) }),
					kit.Rigid(func(gtx C) D { return th.Code(gtx, kit.Hex(c), th.Type.Caption, th.Dim) }),
				)
			})),
		)
	})
}

func (g *gallery) themePage(gtx C) D {
	th := g.th
	sections := []func(gtx C) D{
		func(gtx C) D { return g.section(gtx, "Preview", g.themePreview) },
		func(gtx C) D {
			return g.section(gtx, "Presets", func(gtx C) D {
				var ws []kit.W
				for i, p := range g.presets {
					i, p := i, p
					ws = append(ws, func(gtx C) D {
						return th.Radio(gtx, g.presetBt.Get(p.Name), "preset:"+p.Name, p.Name, g.preset == i)
					})
				}
				ws = append(ws, func(gtx C) D { return th.Radio(gtx, &g.custom, "preset:custom", "Custom", g.preset == -1) })
				return wrapRows(gtx, 4, ws)
			})
		},
		func(gtx C) D {
			return g.section(gtx, "Generate from a seed", func(gtx C) D {
				return kit.Col(gtx,
					kit.Rigid(func(gtx C) D {
						return th.Paragraph(gtx, "Pick a hue: kit.GeneratePalette builds every token in OKLCH, keeping text contrast in light and dark.", th.Muted)
					}),
					kit.Rigid(kit.Gap(0, 10)),
					kit.Rigid(func(gtx C) D {
						return kit.Row(gtx,
							kit.Rigid(func(gtx C) D { return th.Text(gtx, "Hue", th.Type.Body, th.Muted, kit.Regular) }),
							kit.Rigid(kit.Gap(12, 0)),
							kit.Flexed(1, func(gtx C) D { return th.Slider(gtx, &g.hue, "slider:hue") }),
							kit.Rigid(kit.Gap(16, 0)),
							kit.Rigid(func(gtx C) D { return th.Switch(gtx, &g.darkSeed, "switch:dark", "Dark", g.seedDark) }),
						)
					}),
				)
			})
		},
		func(gtx C) D {
			return g.section(gtx, "Shape", func(gtx C) D { return g.knobRows(gtx, "corners", "stroke", "ring") })
		},
		func(gtx C) D {
			return g.section(gtx, "Type", func(gtx C) D { return g.knobRows(gtx, "fonts", "text", "heading") })
		},
		func(gtx C) D {
			return g.section(gtx, "Depth, contrast, size", func(gtx C) D { return g.knobRows(gtx, "elevation", "contrast", "controls") })
		},
		func(gtx C) D {
			return g.section(gtx, "Spacing and motion", func(gtx C) D {
				return kit.Row(gtx,
					kit.Rigid(kit.Fixed(110, func(gtx C) D { return th.Text(gtx, "Density", th.Type.Body, th.Muted, kit.Regular) })),
					kit.Rigid(func(gtx C) D { return th.Tabs(gtx, &g.density, "density", densityNames, g.dens) }),
					kit.Rigid(kit.Gap(24, 0)),
					kit.Rigid(func(gtx C) D { return th.Switch(gtx, &g.motion, "switch:motion", "Animations", g.motionOn) }),
				)
			})
		},
		func(gtx C) D {
			return g.section(gtx, "Tokens", func(gtx C) D {
				p := th.Palette
				type tok struct {
					n string
					c color.NRGBA
				}
				toks := []tok{{"bg", p.Bg}, {"panel", p.Panel}, {"surface", p.Surface}, {"hover", p.Hover}, {"border", p.Border},
					{"divider", p.Divider}, {"fg", p.Fg}, {"muted", p.Muted}, {"dim", p.Dim}, {"accent", p.Accent},
					{"accent-bg", p.AccentBg}, {"on-accent", p.OnAccent}, {"good", p.Good}, {"warn", p.Warn}, {"bad", p.Bad}, {"overlay", p.Overlay}}
				var ws []kit.W
				for _, t := range toks {
					t := t
					ws = append(ws, func(gtx C) D { return swatch(gtx, th, t.n, t.c) })
				}
				return kit.Col(gtx,
					kit.Rigid(func(gtx C) D { return wrapRows(gtx, 4, ws) }),
					kit.Rigid(kit.Gap(0, 6)),
					kit.Rigid(func(gtx C) D {
						return kit.Row(gtx,
							kit.Rigid(func(gtx C) D { return th.Button(gtx, &g.copyBt, "tokens:copy", "Copy as DTCG JSON", kit.Primary) }),
							kit.Rigid(kit.Gap(12, 0)),
							kit.Flexed(1, func(gtx C) D {
								return th.Paragraph(gtx, "Design-token JSON works with Figma (Tokens Studio), Penpot and Style Dictionary. Load one with -theme file.json.", th.Dim)
							}),
						)
					}),
				)
			})
		},
	}
	return g.pageFrame(gtx, "Theme", nil, func(gtx C) D {
		return th.List(gtx, &g.themeLst, "theme", len(sections), func(gtx C, i int) D {
			return layout.Inset{Left: 24, Right: 24}.Layout(gtx, func(gtx C) D { return sections[i](gtx) })
		})
	})
}

// wrapRows lays ws out in rows of n equal columns.
func wrapRows(gtx C, n int, ws []kit.W) D {
	var rows []layout.FlexChild
	for i := 0; i < len(ws); i += n {
		row := ws[i:min(i+n, len(ws))]
		rows = append(rows, kit.Rigid(func(gtx C) D {
			var cells []layout.FlexChild
			for _, w := range row {
				cells = append(cells, kit.Flexed(1, w))
			}
			for j := len(row); j < n; j++ {
				cells = append(cells, kit.Flexed(1, kit.Grow))
			}
			return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D { return layout.Flex{}.Layout(gtx, cells...) })
		}))
	}
	return kit.Col(gtx, rows...)
}

// knobRows lays out the named knobs, one labelled segmented control a row.
func (g *gallery) knobRows(gtx C, ids ...string) D { return g.knobRowsW(gtx, 110, ids...) }

// knobRowsW is knobRows with a label column labelW dp wide.
func (g *gallery) knobRowsW(gtx C, labelW unit.Dp, ids ...string) D {
	th := g.th
	var rows []layout.FlexChild
	for i, id := range ids {
		k := g.knob(id)
		if i > 0 {
			rows = append(rows, kit.Rigid(kit.Gap(0, 10)))
		}
		rows = append(rows, kit.Rigid(func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(kit.Fixed(labelW, func(gtx C) D { return th.Text(gtx, k.label, th.Type.Body, th.Muted, kit.Regular) })),
				kit.Rigid(func(gtx C) D { return th.Tabs(gtx, &k.tabs, "knob:"+k.id, k.names, k.sel) }),
			)
		}))
	}
	return kit.Col(gtx, rows...)
}

// themePreview is a strip of common controls that every knob affects.
func (g *gallery) themePreview(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D { return th.Heading(gtx, "The quick brown fox") }),
		kit.Rigid(kit.Gap(0, 10)),
		kit.Rigid(func(gtx C) D {
			return hrow(gtx, 10,
				func(gtx C) D { return th.Button(gtx, &g.prevSave, "preview:save", "Save", kit.Secondary) },
				func(gtx C) D { return th.Button(gtx, &g.prevSolid, "preview:solid", "Publish", kit.Solid) },
				kit.Fixed(180, func(gtx C) D { return th.TextField(gtx, &g.prevName, "preview:name", "Project name") }),
				kit.Fixed(150, func(gtx C) D {
					return th.Select(gtx, &g.prevSel, "preview:sel", []string{"Private", "Team", "Public"}, g.prevSelI)
				}),
				func(gtx C) D { return th.CheckboxLabel(gtx, &g.prevCheck, "preview:check", "Notify me", g.prevChecked) },
				func(gtx C) D { return th.Badge(gtx, kit.StatusGood, "live") },
			)
		}),
		kit.Rigid(kit.Gap(0, 10)),
		kit.Rigid(func(gtx C) D {
			return th.Code(gtx, "func main() { fmt.Println(\"code uses the mono face\") }", th.Type.Small, th.Muted)
		}),
	)
}
