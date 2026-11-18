package main

import (
	"fmt"
	"slices"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

type item struct {
	ID    string
	Name  string
	Qty   float64
	Done  bool
	Notes string
}

type rowState struct {
	hover kit.HoverArea
	done  kit.Clickable
	step  ui.StepperState
}

type overviewState struct {
	rows      kit.Pool[rowState]
	scroll    kit.List
	items     []*item
	game      lightsOut
	quickSel  ui.SelectState
	surprise  kit.Clickable
	toTheme   kit.Clickable
	explore   kit.Pool[kit.Clickable]
	luckySeed uint32
}

// explorePages are the pages the Overview links to, for first-time visitors.
var explorePages = []string{"all", "menus", "collections", "pickers", "theme", "keyboard"}

// table is the Tasks card's table. Its columns, in order: the item's done
// checkbox labelled with its name, notes, quantity and a status badge.
var table = kit.Table{Inset: 10, RowPad: 9, Cols: []kit.Column{
	{Title: "Item"}, {Title: "Notes", Width: 180}, {Title: "Qty", Width: 96}, {Width: 84, Align: layout.E},
}}

// Columns of table.
const (
	colItem = iota
	colNotes
	colQty
	colStatus
)

func (g *gallery) initOverview() {
	g.overview.game.seed = 7
	g.overview.game.shuffle()
	for i, n := range []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo"} {
		g.overview.items = append(g.overview.items, &item{ID: fmt.Sprint(i + 1), Name: n, Qty: 1, Notes: "note " + n})
	}
}

func (g *gallery) find(id string) *item {
	for _, it := range g.overview.items {
		if it.ID == id {
			return it
		}
	}
	return nil
}

func (g *gallery) openCount() int {
	n := 0
	for _, it := range g.overview.items {
		if !it.Done {
			n++
		}
	}
	return n
}

func (g *gallery) handleOverview(gtx C) {
	g.overview.rows.Each(func(id string, rs *rowState) {
		it := g.find(id)
		switch {
		case rs.done.Clicked(gtx):
			it.Done = !it.Done
		case rs.step.Minus.Clicked(gtx):
			it.Qty = max(0, it.Qty-0.5)
		case rs.step.Plus.Clicked(gtx):
			it.Qty += 0.5
		}
	})
	g.handleGame(gtx)
	if id, ok := g.overview.quickSel.Update(gtx); ok {
		// Not a preset: Custom, the seed from the Theme page (-1).
		g.theme.preset = slices.IndexFunc(g.theme.presets, func(p *kit.Theme) bool { return p.Name == id })
		g.applyTheme()
	}
	if g.overview.surprise.Clicked(gtx) {
		g.surpriseTheme()
	}
	if g.overview.toTheme.Clicked(gtx) {
		g.page = "theme"
	}
	for _, id := range explorePages {
		if g.overview.explore.Get(id).Clicked(gtx) {
			g.page = id
		}
	}
}

// surpriseTheme picks a random preset, corner style, font and control size:
// the quickest way to see that every component follows the theme.
func (g *gallery) surpriseTheme() {
	next := func(n int) int { return lcgNext(&g.overview.luckySeed, n) }
	g.theme.preset = next(len(g.theme.presets))
	knobs := &g.theme.knobs
	for _, k := range []*knob{&knobs.corners, &knobs.fonts, &knobs.controls, &knobs.elevation} {
		k.sel = next(len(k.options))
	}
	g.applyTheme()
	g.status = fmt.Sprintf("Theme: %s, %s corners, %s", g.th.Name, knobs.corners.selected().label, g.th.Fonts)
}

func (g *gallery) overviewPage(gtx C) D {
	th := g.th
	actions := func(gtx C) D {
		return ui.Button{ID: "overview:theme", Text: "Open the theme editor"}.Layout(gtx, th, &g.overview.toTheme)
	}
	rows := []layout.Widget{
		func(gtx C) D {
			return twoCols(gtx, 14,
				func(gtx C) D { return g.section(gtx, "Theme", g.quickTheme) },
				func(gtx C) D { return g.section(gtx, "Lights out", g.demoGame) })
		},
		func(gtx C) D { return g.section(gtx, "Tasks", g.tasks) },
		func(gtx C) D {
			return twoCols(gtx, 14,
				func(gtx C) D { return g.section(gtx, "Playground", g.playground) },
				func(gtx C) D { return g.section(gtx, "Explore", g.exploreLinks) })
		},
	}
	return g.pageFrame(gtx, "Welcome to giokit", actions, func(gtx C) D {
		return th.List(gtx, &g.overview.scroll, "overview", len(rows), func(gtx C, i int) D {
			return layout.Inset{Left: 24, Right: 24}.Layout(gtx, rows[i])
		})
	})
}

// twoCols lays a and b side by side, or stacked when the page is narrow.
func twoCols(gtx C, gap unit.Dp, a, b layout.Widget) D {
	if gtx.Constraints.Max.X < gtx.Dp(760) {
		return kit.Col(gtx, kit.Rigid(a), kit.Rigid(b))
	}
	return layout.Flex{Alignment: layout.Start}.Layout(gtx,
		kit.Flexed(1, a),
		kit.Rigid(kit.Gap(gap, 0)),
		kit.Flexed(1, b),
	)
}

// quickTheme is the Overview's theme bar: preset, a random pick, and the
// knobs that change the look the most (shared with the Theme page).
func (g *gallery) quickTheme(gtx C) D {
	th := g.th
	names := make([]string, 0, len(g.theme.presets)+1)
	for _, p := range g.theme.presets {
		names = append(names, p.Name)
	}
	names = append(names, "Custom")
	sel := g.theme.preset
	if sel < 0 {
		sel = len(g.theme.presets)
	}
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(kit.Fixed(76, func(gtx C) D { return th.Text(gtx, "Theme", th.Type.Body, th.Muted, kit.Regular) })),
				kit.Rigid(kit.Fixed(150, func(gtx C) D {
					return ui.Select{ID: "quick:preset", Options: choices(names), Selected: names[sel]}.Layout(gtx, th, &g.overview.quickSel)
				})),
				kit.Rigid(kit.Gap(10, 0)),
				kit.Rigid(func(gtx C) D {
					return ui.Button{ID: "quick:surprise", Text: "Surprise me", Style: ui.Ghost}.Layout(gtx, th, &g.overview.surprise)
				}),
			)
		}),
		kit.Rigid(kit.Gap(0, 10)),
		kit.Rigid(func(gtx C) D {
			return g.knobRowsW(gtx, 76, &g.theme.knobs.corners, &g.theme.knobs.fonts, &g.theme.knobs.controls)
		}),
	)
}

// tasks is the original overview: stats over an editable table.
func (g *gallery) tasks(gtx C) D {
	th := g.th
	var qty float64
	done := 0
	for _, it := range g.overview.items {
		qty += it.Qty
		if it.Done {
			done++
		}
	}
	rows := []layout.FlexChild{
		kit.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				kit.Flexed(1, func(gtx C) D {
					return ui.Stat{Title: "Items", Value: fmt.Sprint(len(g.overview.items))}.Layout(gtx, th)
				}),
				kit.Rigid(kit.Gap(10, 0)),
				kit.Flexed(1, func(gtx C) D {
					return ui.Stat{Title: "Quantity", Value: fmt.Sprintf("%g", qty), ValueColor: th.Fg}.Layout(gtx, th)
				}),
				kit.Rigid(kit.Gap(10, 0)),
				kit.Flexed(1, func(gtx C) D {
					return ui.Stat{Title: "Done", Value: fmt.Sprintf("%d/%d", done, len(g.overview.items)), ValueColor: th.Good}.Layout(gtx, th)
				}),
			)
		}),
		kit.Rigid(kit.Gap(0, 12)),
		kit.Rigid(func(gtx C) D { return table.Header(gtx, th) }),
	}
	for i, it := range g.overview.items {
		rows = append(rows, kit.Rigid(func(gtx C) D { return g.row(gtx, it, i == len(g.overview.items)-1) }))
	}
	return kit.Col(gtx, rows...)
}

// playground is a handful of controls wired to visible results.
func (g *gallery) playground(gtx C) D {
	th := g.th
	row := kit.Flex{Gap: kit.Space3, Align: layout.Middle}
	return kit.Flex{Axis: layout.Vertical, Gap: kit.Space3p5}.Layout(gtx, th,
		kit.Rigid(func(gtx C) D {
			return kit.Flex{Gap: kit.Space6, Align: layout.Middle}.Layout(gtx, th,
				kit.Rigid(func(gtx C) D {
					return ui.Switch{ID: "switch:wifi", Label: "Wi-Fi", On: g.controls.wifiOn}.Layout(gtx, th, &g.controls.wifi)
				}),
				kit.Rigid(func(gtx C) D {
					return ui.Checkbox{ID: "check:agree", Label: "I agree", Checked: g.controls.agreed}.Layout(gtx, th, &g.controls.agree)
				}),
			)
		}),
		kit.Rigid(func(gtx C) D {
			return row.Layout(gtx, th,
				kit.Rigid(func(gtx C) D { return th.Text(gtx, "Volume", th.Type.Body, th.Muted, kit.Regular) }),
				kit.Flexed(1, func(gtx C) D { return g.controls.volume.layout(gtx, th, "slider:volume", "Volume") }),
				kit.Rigid(kit.Fixed(40, func(gtx C) D {
					return th.Text(gtx, fmt.Sprintf("%d%%", int(g.controls.volume.Value*100+0.5)), th.Type.Body, th.Fg, kit.Medium)
				})),
			)
		}),
		kit.Rigid(func(gtx C) D { return th.Progress(gtx, g.controls.volume.Value) }),
		kit.Rigid(func(gtx C) D {
			return kit.Flex{Gap: kit.Space2p5, Align: layout.Middle}.Layout(gtx, th,
				kit.Rigid(func(gtx C) D {
					return ui.Button{ID: "toast:good", Text: "Show a toast"}.Layout(gtx, th, &g.overlays.toastBtns[1])
				}),
				kit.Rigid(func(gtx C) D {
					return ui.Button{ID: "dialog:open", Text: "Open a dialog", Style: ui.Danger}.Layout(gtx, th, &g.overlays.delBtn)
				}),
			)
		}),
	)
}

// exploreLinks sends a first-time visitor to the most telling pages.
func (g *gallery) exploreLinks(gtx C) D {
	th := g.th
	blurbs := map[string]string{
		"all":         "every component on one page",
		"menus":       "menus and context menus, by mouse or keyboard",
		"collections": "a list, a tree and a sortable table",
		"pickers":     "selects and a date picker",
		"theme":       "every theme knob, and the design-token export",
		"keyboard":    "all the shortcuts",
	}
	var rows []layout.FlexChild
	for _, id := range explorePages {
		title := id
		if p, ok := g.catalog.page(id); ok {
			title = p.title
		}
		rows = append(rows, kit.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D {
				return kit.Row(gtx,
					kit.Rigid(func(gtx C) D {
						return ui.Link{ID: "explore:" + id, Text: title}.Layout(gtx, th, g.overview.explore.Get(id))
					}),
					kit.Rigid(kit.Gap(8, 0)),
					kit.Flexed(1, func(gtx C) D { return th.Text(gtx, blurbs[id], th.Type.Body, th.Dim, kit.Regular) }),
				)
			})
		}))
	}
	return kit.Col(gtx, rows...)
}

func (g *gallery) row(gtx C, it *item, last bool) D {
	th := g.th
	rs := g.overview.rows.Get(it.ID)
	return table.Row(gtx, th, &rs.hover, "row:"+it.ID, last, func(gtx C, col int, hovered bool) D {
		switch col {
		case colItem:
			return ui.Checkbox{ID: "done:" + it.ID, Label: it.Name, Checked: it.Done}.Layout(gtx, th, &rs.done)
		case colNotes:
			return th.Text(gtx, it.Notes, th.Type.Body, th.Dim, kit.Regular)
		case colQty:
			return ui.Stepper{ID: "qty:" + it.ID, Label: it.Name, Value: fmt.Sprintf("%g", it.Qty), Active: hovered}.Layout(gtx, th, &rs.step)
		default:
			if it.Done {
				return ui.Badge{Status: kit.StatusGood, Text: "done"}.Layout(gtx, th)
			}
			return ui.Badge{Status: kit.StatusInfo, Text: "open"}.Layout(gtx, th)
		}
	})
}
