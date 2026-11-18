package main

import (
	"bytes"
	"image"
	"image/color"
	"io"
	"slices"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/fonts"
	"github.com/hhhapz/giokit/kit/ui"
)

type themeState struct {
	presets  []*kit.Theme
	preset   int // index into presets, or -1 for the custom theme
	presetBt kit.Pool[kit.Clickable]
	density  ui.TabsState
	dens     int
	motion   kit.Clickable
	motionOn bool
	custom   kit.Clickable
	hue      slider
	darkSeed kit.Clickable
	seedDark bool
	copyBt   kit.Clickable
	list     kit.List
	knobs    themeKnobs

	prevSave, prevSolid kit.Clickable
	prevName            widget.Editor
	prevSel             ui.SelectState
	prevSelI            int
	prevCheck           kit.Clickable
	prevChecked         bool
}

// knob is one theme setting shown as a labelled segmented control.
type knob struct {
	id, label string
	options   []knobOption
	sel       int // index into options
	tabs      ui.TabsState
}

// knobOption is one choice of a knob: its tab label and the theme setting
// it applies.
type knobOption struct {
	label string
	apply func(t *kit.Theme)
}

func (k *knob) selected() knobOption { return k.options[k.sel] }

func (k *knob) labels() []string {
	out := make([]string, len(k.options))
	for i, o := range k.options {
		out[i] = o.label
	}
	return out
}

// themeKnobs are the Theme page's knobs. The Overview shows a few of them
// too.
type themeKnobs struct {
	corners, stroke, ring         knob
	fonts, text, heading          knob
	elevation, contrast, controls knob
}

// all lists every knob, in display order. Each sets its own theme fields;
// applyTheme sets the palette before applying them, so the contrast knob
// adjusts the chosen palette.
func (k *themeKnobs) all() []*knob {
	return []*knob{&k.corners, &k.stroke, &k.ring, &k.fonts, &k.text, &k.heading, &k.elevation, &k.contrast, &k.controls}
}

func newThemeKnobs() themeKnobs {
	radius := func(label string, r unit.Dp) knobOption {
		return knobOption{label: label, apply: func(t *kit.Theme) { t.Radius = r }}
	}
	stroke := func(label string, w unit.Dp) knobOption {
		return knobOption{label: label, apply: func(t *kit.Theme) { t.Stroke = w }}
	}
	ring := func(label string, w unit.Dp) knobOption {
		return knobOption{label: label, apply: func(t *kit.Theme) { t.Ring = w }}
	}
	textScale := func(label string, f float32) knobOption {
		return knobOption{label: label, apply: func(t *kit.Theme) { t.Type = kit.DefaultType().Scaled(f) }}
	}
	heading := func(label string, w font.Weight) knobOption {
		return knobOption{label: label, apply: func(t *kit.Theme) { t.HeadingWeight = w }}
	}
	elevation := func(label string, e float32) knobOption {
		return knobOption{label: label, apply: func(t *kit.Theme) { t.Elevation = e }}
	}
	controls := func(label string, c kit.ControlSize) knobOption {
		return knobOption{label: label, apply: func(t *kit.Theme) { t.Controls = c }}
	}
	var fontOptions []knobOption
	for _, set := range fonts.All() {
		fontOptions = append(fontOptions, knobOption{label: set.Name, apply: func(t *kit.Theme) {
			if t.Fonts != set.Name {
				t.UseFonts(set) // a new shaper only when the font set changes
			}
		}})
	}
	return themeKnobs{
		corners: knob{id: "corners", label: "Corners", sel: 2, options: []knobOption{
			radius("Sharp", 0), radius("Subtle", 3), radius("Default", 6), radius("Round", 10), radius("Pill", 999),
		}},
		stroke: knob{id: "stroke", label: "Borders", sel: 0, options: []knobOption{
			stroke("Hairline", 1), stroke("Medium", 1.5), stroke("Bold", 2),
		}},
		ring: knob{id: "ring", label: "Focus ring", sel: 1, options: []knobOption{
			ring("Thin", 1.5), ring("Default", 2), ring("Thick", 3),
		}},
		fonts: knob{id: "fonts", label: "Font", sel: 0, options: fontOptions},
		text: knob{id: "text", label: "Text size", sel: 1, options: []knobOption{
			textScale("90%", 0.9), textScale("100%", 1), textScale("115%", 1.15), textScale("130%", 1.3),
		}},
		heading: knob{id: "heading", label: "Headings", sel: 2, options: []knobOption{
			heading("Medium", font.Medium), heading("Semibold", font.SemiBold), heading("Bold", font.Bold),
		}},
		elevation: knob{id: "elevation", label: "Shadows", sel: 1, options: []knobOption{
			elevation("Flat", 0), elevation("Soft", 1), elevation("Strong", 2),
		}},
		contrast: knob{id: "contrast", label: "Contrast", sel: 0, options: []knobOption{
			{label: "Standard", apply: func(*kit.Theme) {}},
			{label: "High", apply: func(t *kit.Theme) { t.Palette = t.Palette.WithContrast(t.Dark, 1) }},
		}},
		controls: knob{id: "controls", label: "Controls", sel: 1, options: []knobOption{
			controls("Small", kit.ControlSmall), controls("Medium", kit.ControlMedium), controls("Large", kit.ControlLarge),
		}},
	}
}

// previewOptions are the theme preview's select options.
var previewOptions = []string{"Private", "Team", "Public"}

// densityOption is one choice of the Theme page's density tabs.
type densityOption struct {
	label   string  // the tab's label and id
	density float32 // Theme.Density
}

// densities are the density tabs, in display order.
var densities = []densityOption{
	{label: "Compact", density: 0.75},
	{label: "Regular", density: 1},
	{label: "Comfortable", density: 1.25},
}

// densityChoices are the density tabs' items.
func densityChoices() []ui.Choice {
	items := make([]ui.Choice, len(densities))
	for i, d := range densities {
		items[i] = ui.Choice{ID: d.label, Label: d.label}
	}
	return items
}

func (g *gallery) initTheme() {
	g.theme.presets = kit.Presets()
	g.th = g.theme.presets[0]
	g.theme.dens, g.theme.motionOn, g.theme.seedDark = 1, true, true
	g.theme.hue.Value = 0.6
	g.theme.list.Axis = layout.Vertical
	g.theme.knobs = newThemeKnobs()
}

// applyTheme rebuilds g.th from the chosen palette plus density and motion.
// Derive keeps the font shaper, so switching is cheap.
func (g *gallery) applyTheme() {
	base := g.th
	var p kit.Palette
	name, dark := "Custom", g.theme.seedDark
	if g.theme.preset >= 0 {
		pr := g.theme.presets[g.theme.preset]
		p, name, dark = pr.Palette, pr.Name, pr.Dark
	} else {
		p = kit.GeneratePalette(g.customSeed())
	}
	g.th = base.Derive(func(t *kit.Theme) {
		t.Palette, t.Name, t.Dark = p, name, dark
		t.Density = densities[g.theme.dens].density
		t.Motion = 0
		if g.theme.motionOn {
			t.Motion = 1
		}
		for _, k := range g.theme.knobs.all() {
			k.selected().apply(t)
		}
	})
}

func (g *gallery) customSeed() kit.Seed {
	h := float64(g.theme.hue.Value) * 360
	return kit.Seed{Name: "Custom", Dark: g.theme.seedDark,
		Accent:  kit.OKLCH{L: 0.65, C: 0.16, H: h}.RGB(),
		Neutral: kit.OKLCH{L: 0.5, C: 0.03, H: h}.RGB()}
}

func (g *gallery) nextTheme() {
	g.theme.preset = (g.theme.preset + 1) % len(g.theme.presets)
	g.applyTheme()
}

func (g *gallery) handleTheme(gtx C) {
	changed := false
	for i := range g.theme.presets {
		if g.theme.presetBt.Get(g.theme.presets[i].Name).Clicked(gtx) {
			g.theme.preset, changed = i, true
		}
	}
	if id, ok := g.theme.density.Update(gtx); ok {
		g.theme.dens = slices.IndexFunc(densities, func(d densityOption) bool { return d.label == id })
		changed = true
	}
	if g.theme.motion.Clicked(gtx) {
		g.theme.motionOn, changed = !g.theme.motionOn, true
	}
	if g.theme.custom.Clicked(gtx) {
		g.theme.preset, changed = -1, true
	}
	if g.theme.darkSeed.Clicked(gtx) {
		g.theme.seedDark, changed = !g.theme.seedDark, true
		if g.theme.preset != -1 {
			g.theme.preset = -1
		}
	}
	if g.theme.hue.update(gtx) {
		g.theme.preset, changed = -1, true
	}
	for _, k := range g.theme.knobs.all() {
		if id, ok := k.tabs.Update(gtx); ok && id != k.selected().label {
			k.sel, changed = slices.IndexFunc(k.options, func(o knobOption) bool { return o.label == id }), true
		}
	}
	g.theme.prevSave.Clicked(gtx)
	g.theme.prevSolid.Clicked(gtx)
	if id, ok := g.theme.prevSel.Update(gtx); ok {
		g.theme.prevSelI = slices.Index(previewOptions, id)
	}
	if g.theme.prevCheck.Clicked(gtx) {
		g.theme.prevChecked = !g.theme.prevChecked
	}
	if g.theme.copyBt.Clicked(gtx) {
		if b, err := g.th.Tokens(); err != nil {
			g.status = err.Error()
		} else {
			gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(bytes.NewReader(b))})
			g.status = "Theme tokens (DTCG JSON) copied to the clipboard"
		}
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
				var ws []layout.Widget
				for i, p := range g.theme.presets {
					ws = append(ws, func(gtx C) D {
						return ui.Radio{ID: "preset:" + p.Name, Label: p.Name, Selected: g.theme.preset == i}.Layout(gtx, th, g.theme.presetBt.Get(p.Name))
					})
				}
				ws = append(ws, func(gtx C) D {
					return ui.Radio{ID: "preset:custom", Label: "Custom", Selected: g.theme.preset == -1}.Layout(gtx, th, &g.theme.custom)
				})
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
							kit.Flexed(1, func(gtx C) D { return g.theme.hue.layout(gtx, th, "slider:hue", "Hue") }),
							kit.Rigid(kit.Gap(16, 0)),
							kit.Rigid(func(gtx C) D {
								return ui.Switch{ID: "switch:dark", Label: "Dark", On: g.theme.seedDark}.Layout(gtx, th, &g.theme.darkSeed)
							}),
						)
					}),
				)
			})
		},
		func(gtx C) D {
			return g.section(gtx, "Shape", func(gtx C) D {
				return g.knobRows(gtx, &g.theme.knobs.corners, &g.theme.knobs.stroke, &g.theme.knobs.ring)
			})
		},
		func(gtx C) D {
			return g.section(gtx, "Type", func(gtx C) D {
				return g.knobRows(gtx, &g.theme.knobs.fonts, &g.theme.knobs.text, &g.theme.knobs.heading)
			})
		},
		func(gtx C) D {
			return g.section(gtx, "Depth, contrast, size", func(gtx C) D {
				return g.knobRows(gtx, &g.theme.knobs.elevation, &g.theme.knobs.contrast, &g.theme.knobs.controls)
			})
		},
		func(gtx C) D {
			return g.section(gtx, "Spacing and motion", func(gtx C) D {
				return kit.Row(gtx,
					kit.Rigid(kit.Fixed(110, func(gtx C) D { return th.Text(gtx, "Density", th.Type.Body, th.Muted, kit.Regular) })),
					kit.Rigid(func(gtx C) D {
						return ui.Tabs{ID: "density", Items: densityChoices(), Selected: densities[g.theme.dens].label, Style: ui.TabsSegmented}.Layout(gtx, th, &g.theme.density)
					}),
					kit.Rigid(kit.Gap(24, 0)),
					kit.Rigid(func(gtx C) D {
						return ui.Switch{ID: "switch:motion", Label: "Animations", On: g.theme.motionOn}.Layout(gtx, th, &g.theme.motion)
					}),
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
				var ws []layout.Widget
				for _, t := range toks {
					ws = append(ws, func(gtx C) D { return swatch(gtx, th, t.n, t.c) })
				}
				return kit.Col(gtx,
					kit.Rigid(func(gtx C) D { return wrapRows(gtx, 4, ws) }),
					kit.Rigid(kit.Gap(0, 6)),
					kit.Rigid(func(gtx C) D {
						return kit.Row(gtx,
							kit.Rigid(func(gtx C) D {
								return ui.Button{ID: "tokens:copy", Text: "Copy as DTCG JSON", Style: ui.Primary}.Layout(gtx, th, &g.theme.copyBt)
							}),
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
		return th.List(gtx, &g.theme.list, "theme", len(sections), func(gtx C, i int) D {
			return layout.Inset{Left: 24, Right: 24}.Layout(gtx, func(gtx C) D { return sections[i](gtx) })
		})
	})
}

// wrapRows lays ws out in rows of n equal columns.
func wrapRows(gtx C, n int, ws []layout.Widget) D {
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

// knobRows lays out knobs, one labelled segmented control a row.
func (g *gallery) knobRows(gtx C, knobs ...*knob) D { return g.knobRowsW(gtx, 110, knobs...) }

// knobRowsW is knobRows with a label column labelW dp wide.
func (g *gallery) knobRowsW(gtx C, labelW unit.Dp, knobs ...*knob) D {
	th := g.th
	var rows []layout.FlexChild
	for i, k := range knobs {
		if i > 0 {
			rows = append(rows, kit.Rigid(kit.Gap(0, 10)))
		}
		rows = append(rows, kit.Rigid(func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(kit.Fixed(labelW, func(gtx C) D { return th.Text(gtx, k.label, th.Type.Body, th.Muted, kit.Regular) })),
				kit.Rigid(func(gtx C) D {
					return ui.Tabs{ID: "knob:" + k.id, Items: choices(k.labels()), Selected: k.selected().label, Style: ui.TabsSegmented}.Layout(gtx, th, &k.tabs)
				}),
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
			return kit.Flex{Gap: kit.Space2p5, Align: layout.Middle}.Layout(gtx, th,
				kit.Rigid(func(gtx C) D { return ui.Button{ID: "preview:save", Text: "Save"}.Layout(gtx, th, &g.theme.prevSave) }),
				kit.Rigid(func(gtx C) D {
					return ui.Button{ID: "preview:solid", Text: "Publish", Style: ui.Solid}.Layout(gtx, th, &g.theme.prevSolid)
				}),
				kit.Rigid(kit.Fixed(180, func(gtx C) D {
					return ui.TextField{ID: "preview:name", Placeholder: "Project name"}.Layout(gtx, th, &g.theme.prevName)
				})),
				kit.Rigid(kit.Fixed(150, func(gtx C) D {
					return ui.Select{ID: "preview:sel", Options: choices(previewOptions), Selected: previewOptions[g.theme.prevSelI]}.Layout(gtx, th, &g.theme.prevSel)
				})),
				kit.Rigid(func(gtx C) D {
					return ui.Checkbox{ID: "preview:check", Label: "Notify me", Checked: g.theme.prevChecked}.Layout(gtx, th, &g.theme.prevCheck)
				}),
				kit.Rigid(func(gtx C) D { return ui.Badge{Status: kit.StatusGood, Text: "live"}.Layout(gtx, th) }),
			)
		}),
		kit.Rigid(kit.Gap(0, 10)),
		kit.Rigid(func(gtx C) D {
			return th.Code(gtx, "func main() { fmt.Println(\"code uses the mono face\") }", th.Type.Small, th.Muted)
		}),
	)
}
