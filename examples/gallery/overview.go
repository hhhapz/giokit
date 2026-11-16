package main

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/hhhapz/giokit/kit"
)

type item struct {
	ID    string
	Name  string
	Qty   float64
	Done  bool
	Notes string
}

type rowState struct {
	hover          kit.Hover
	done, dec, inc kit.Clickable
}

type overviewState struct {
	rows      kit.Pool[rowState]
	ovScroll  kit.List
	items     []*item
	game      lightsOut
	quickSel  kit.Select
	surprise  kit.Clickable
	toTheme   kit.Clickable
	explore   kit.Pool[kit.Clickable]
	luckySeed uint32
}

// explorePages are the pages the Overview links to, for first-time visitors.
var explorePages = []string{"all", "menus", "collections", "pickers", "theme", "keyboard"}

var table = kit.Table{Inset: 10, RowPad: 9, Cols: []kit.Column{
	{Width: 34}, {Title: "Item"}, {Title: "Notes", Width: 180}, {Title: "Qty", Width: 96}, {Width: 84, Align: layout.E},
}}

func (g *gallery) initOverview() {
	g.game.seed = 7
	g.game.shuffle()
	for i, n := range []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo"} {
		g.items = append(g.items, &item{ID: fmt.Sprint(i + 1), Name: n, Qty: 1, Notes: "note " + n})
	}
}

func (g *gallery) find(id string) *item {
	for _, it := range g.items {
		if it.ID == id {
			return it
		}
	}
	return nil
}

func (g *gallery) openCount() int {
	n := 0
	for _, it := range g.items {
		if !it.Done {
			n++
		}
	}
	return n
}

func (g *gallery) handleOverview(gtx C) {
	g.rows.Each(func(id string, rs *rowState) {
		it := g.find(id)
		switch {
		case rs.done.Clicked(gtx):
			it.Done = !it.Done
		case rs.dec.Clicked(gtx):
			it.Qty = max(0, it.Qty-0.5)
		case rs.inc.Clicked(gtx):
			it.Qty += 0.5
		}
	})
	g.handleGame(gtx)
	if i, ok := g.quickSel.Changed(gtx); ok {
		g.preset = i
		if i == len(g.presets) {
			g.preset = -1 // Custom: the seed from the Theme page
		}
		g.applyTheme()
	}
	if g.surprise.Clicked(gtx) {
		g.surpriseTheme()
	}
	if g.toTheme.Clicked(gtx) {
		g.page = "theme"
	}
	for _, id := range explorePages {
		if g.explore.Get(id).Clicked(gtx) {
			g.page = id
		}
	}
}

// surpriseTheme picks a random preset, corner style, font and control size:
// the quickest way to see that every component follows the theme.
func (g *gallery) surpriseTheme() {
	next := func(n int) int {
		g.luckySeed = g.luckySeed*1664525 + 1013904223
		return int(g.luckySeed>>16) % n
	}
	g.preset = next(len(g.presets))
	for _, id := range []string{"corners", "fonts", "controls", "elevation"} {
		k := g.knob(id)
		k.sel = next(len(k.names))
	}
	g.applyTheme()
	g.status = fmt.Sprintf("Theme: %s, %s corners, %s", g.th.Name, g.knob("corners").names[g.knob("corners").sel], g.th.Fonts)
}

func (g *gallery) overview(gtx C) D {
	th := g.th
	actions := func(gtx C) D {
		return th.Button(gtx, &g.toTheme, "overview:theme", "Open the theme editor", kit.Secondary)
	}
	rows := []kit.W{
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
		return th.List(gtx, &g.ovScroll, "overview", len(rows), func(gtx C, i int) D {
			return layout.Inset{Left: 24, Right: 24}.Layout(gtx, rows[i])
		})
	})
}

// twoCols lays a and b side by side, or stacked when the page is narrow.
func twoCols(gtx C, gap unit.Dp, a, b kit.W) D {
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
	names := make([]string, 0, len(g.presets)+1)
	for _, p := range g.presets {
		names = append(names, p.Name)
	}
	names = append(names, "Custom")
	sel := g.preset
	if sel < 0 {
		sel = len(g.presets)
	}
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(kit.Fixed(76, func(gtx C) D { return th.Text(gtx, "Theme", th.Type.Body, th.Muted, kit.Regular) })),
				kit.Rigid(kit.Fixed(150, func(gtx C) D { return th.Select(gtx, &g.quickSel, "quick:preset", names, sel) })),
				kit.Rigid(kit.Gap(10, 0)),
				kit.Rigid(func(gtx C) D { return th.Button(gtx, &g.surprise, "quick:surprise", "Surprise me", kit.Ghost) }),
			)
		}),
		kit.Rigid(kit.Gap(0, 10)),
		kit.Rigid(func(gtx C) D { return g.knobRowsW(gtx, 76, "corners", "fonts", "controls") }),
	)
}

// tasks is the original overview: stats over an editable table.
func (g *gallery) tasks(gtx C) D {
	th := g.th
	var qty float64
	done := 0
	for _, it := range g.items {
		qty += it.Qty
		if it.Done {
			done++
		}
	}
	rows := []layout.FlexChild{
		kit.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				kit.Flexed(1, th.Stat("Items", fmt.Sprint(len(g.items)), th.Fg)),
				kit.Rigid(kit.Gap(10, 0)),
				kit.Flexed(1, th.Stat("Quantity", fmt.Sprintf("%g", qty), th.Fg)),
				kit.Rigid(kit.Gap(10, 0)),
				kit.Flexed(1, th.Stat("Done", fmt.Sprintf("%d/%d", done, len(g.items)), th.Good)),
			)
		}),
		kit.Rigid(kit.Gap(0, 12)),
		kit.Rigid(func(gtx C) D { return table.Header(gtx, th) }),
	}
	for i, it := range g.items {
		rows = append(rows, kit.Rigid(func(gtx C) D { return g.row(gtx, it, i == len(g.items)-1) }))
	}
	return kit.Col(gtx, rows...)
}

// playground is a handful of controls wired to visible results.
func (g *gallery) playground(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return hrow(gtx, 24,
				func(gtx C) D { return th.Switch(gtx, &g.wifi, "switch:wifi", "Wi-Fi", g.wifiOn) },
				func(gtx C) D { return th.CheckboxLabel(gtx, &g.agree, "check:agree", "I agree", g.agreed) },
			)
		}),
		kit.Rigid(kit.Gap(0, 14)),
		kit.Rigid(func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(func(gtx C) D { return th.Text(gtx, "Volume", th.Type.Body, th.Muted, kit.Regular) }),
				kit.Rigid(kit.Gap(12, 0)),
				kit.Flexed(1, func(gtx C) D { return th.Slider(gtx, &g.volume, "slider:volume") }),
				kit.Rigid(kit.Gap(12, 0)),
				kit.Rigid(kit.Fixed(40, func(gtx C) D {
					return th.Text(gtx, fmt.Sprintf("%d%%", int(g.volume.Value*100+0.5)), th.Type.Body, th.Fg, kit.Medium)
				})),
			)
		}),
		kit.Rigid(kit.Gap(0, 10)),
		kit.Rigid(func(gtx C) D { return th.Progress(gtx, g.volume.Value) }),
		kit.Rigid(kit.Gap(0, 14)),
		kit.Rigid(func(gtx C) D {
			return hrow(gtx, 10,
				func(gtx C) D { return th.Button(gtx, &g.toastBtns[1], "toast:good", "Show a toast", kit.Secondary) },
				func(gtx C) D { return th.Button(gtx, &g.delBtn, "dialog:open", "Open a dialog", kit.Danger) },
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
		for _, p := range pages {
			if p.id == id {
				title = p.title
			}
		}
		rows = append(rows, kit.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D {
				return kit.Row(gtx,
					kit.Rigid(func(gtx C) D { return th.Link(gtx, g.explore.Get(id), "explore:"+id, title) }),
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
	rs := g.rows.Get(it.ID)
	return table.Row(gtx, th, &rs.hover, "row:"+it.ID, last, func(gtx C, col int, hovered bool) D {
		switch col {
		case 0:
			return th.Checkbox(gtx, &rs.done, "done:"+it.ID, it.Done)
		case 1:
			fg := th.Fg
			if it.Done {
				fg = th.Muted
			}
			return th.Text(gtx, it.Name, th.Type.Text, fg, kit.Medium)
		case 2:
			return th.Text(gtx, it.Notes, th.Type.Body, th.Dim, kit.Regular)
		case 3:
			return th.Stepper(gtx, &rs.dec, &rs.inc, "qty:"+it.ID, fmt.Sprintf("%g", it.Qty), th.Fg, hovered)
		default:
			if it.Done {
				return th.Badge(gtx, kit.StatusGood, "done")
			}
			return th.Badge(gtx, kit.StatusInfo, "open")
		}
	})
}
