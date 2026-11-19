package main

import (
	"fmt"
	"image"
	"strings"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/unit"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

// demo is one titled card: a component or a small group of related ones.
type demo struct {
	title string
	body  func(g *gallery, gtx C) D
}

// page is one screen. A page either lists demos (rendered as cards in a
// scrolling column) or lays itself out (layout), for demos that need the
// whole pane: the overview app, split panes, collections, logs, the theme
// editor.
type page struct {
	id, title string
	demos     []demo
	layout    func(g *gallery, gtx C) D
	actions   func(g *gallery, gtx C) D // optional, beside the page title
}

// category is a sidebar section, as in a component library's docs.
type category struct {
	title string
	pages []page
}

// catalog is the gallery's table of contents: its categories, their pages
// in sidebar order, and the sidebar's rows. It is built once and never
// changed; a gallery reads it through its catalog field.
type catalog struct {
	categories []category
	pages      []page
	rows       []navRow
}

// navRow is one sidebar row: a category header or a page.
type navRow struct {
	header string
	page   int // index into pages, when header == ""
}

func newCatalog(categories []category) *catalog {
	c := &catalog{categories: categories}
	for _, cat := range categories {
		c.rows = append(c.rows, navRow{header: cat.title})
		for _, p := range cat.pages {
			c.rows = append(c.rows, navRow{page: len(c.pages)})
			c.pages = append(c.pages, p)
		}
	}
	return c
}

// rowOfPage is the sidebar row index of pages[i].
func (c *catalog) rowOfPage(i int) int {
	for r, row := range c.rows {
		if row.header == "" && row.page == i {
			return r
		}
	}
	return 0
}

// page returns the page with the given id.
func (c *catalog) page(id string) (page, bool) {
	for _, p := range c.pages {
		if p.id == id {
			return p, true
		}
	}
	return page{}, false
}

// pageIDs lists every page id in sidebar order.
func (c *catalog) pageIDs() []string {
	ids := make([]string, len(c.pages))
	for i, p := range c.pages {
		ids[i] = p.id
	}
	return ids
}

// demoCatalog is every demo, grouped into pages and categories. Page
// layouts reach the catalog through gallery.catalog, not this variable, so
// that the "all" page can list it without an initialization cycle.
var demoCatalog = newCatalog([]category{
	{"Getting started", []page{
		{id: "overview", title: "Overview", layout: (*gallery).overviewPage},
		{id: "all", title: "All components", layout: (*gallery).allPage},
	}},
	{"Inputs", []page{
		{id: "buttons", title: "Buttons", demos: []demo{
			{"Buttons and size", (*gallery).demoButtons},
		}},
		{id: "selection", title: "Selection", demos: []demo{
			{"Switches and checkboxes", (*gallery).demoToggles},
			{"Slider", (*gallery).demoSlider},
			{"Multiple selection", (*gallery).demoMultiSelect},
		}},
		{id: "text", title: "Text fields", demos: []demo{
			{"Text fields", (*gallery).demoTextFields},
			{"Combobox", (*gallery).demoCombobox},
		}},
		{id: "pickers", title: "Pickers", demos: []demo{
			{"Select", (*gallery).demoSelect},
			{"Date", (*gallery).demoDate},
		}},
	}},
	{"Navigation", []page{
		{id: "tabs", title: "Tabs", demos: []demo{
			{"Tabs and segmented control", (*gallery).demoTabs},
		}},
		{id: "menus", title: "Menus", actions: (*gallery).moreMenuButton, demos: []demo{
			{"Menu", (*gallery).demoMenu},
			{"Context menu", (*gallery).demoContextMenu},
		}},
		{id: "disclosure", title: "Disclosure", demos: []demo{
			{"Breadcrumbs and accordion", (*gallery).demoBreadcrumbs},
			{"Disclosure and flow", (*gallery).demoDisclosure},
		}},
	}},
	{"Data display", []page{
		{id: "collections", title: "Lists, trees, tables", layout: (*gallery).workspacePage},
		{id: "logs", title: "Logs", layout: (*gallery).logsPage},
		{id: "video", title: "Video", demos: []demo{
			{"Video stream", (*gallery).demoVideo},
		}},
	}},
	{"Feedback", []page{
		{id: "status", title: "Status", demos: []demo{
			{"Badges, spinner, progress", (*gallery).demoStatus},
			{"Banners", (*gallery).demoBanners},
			{"Empty state", (*gallery).demoEmpty},
		}},
		{id: "overlays", title: "Dialogs and popovers", demos: []demo{
			{"Popover", (*gallery).demoPopover},
			{"Dialog and toasts", (*gallery).demoDialogs},
		}},
	}},
	{"Layout", []page{
		{id: "panes", title: "Split panes", layout: (*gallery).layoutPage},
		{id: "forms", title: "Forms", demos: []demo{
			{"Toolbar and link", (*gallery).demoToolbar},
			{"Responsive form", (*gallery).demoForm},
			{"Disabled (th.Disable)", (*gallery).demoDisabled},
		}},
	}},
	{"System", []page{
		{id: "theme", title: "Theme", layout: (*gallery).themePage},
		{id: "keyboard", title: "Keyboard", demos: []demo{
			{"Shortcuts", (*gallery).demoKeyboard},
		}},
	}},
})

func (g *gallery) sidebar(gtx C) D {
	th := g.th
	cat := g.catalog
	nav := keyNav{
		ID:      "sidebar",
		IDs:     cat.pageIDs(),
		Current: &g.page,
		Reveal:  func(i int) { g.side.ScrollIntoView(cat.rowOfPage(i)) },
	}
	return kit.Fill(gtx, th.Panel, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		return kit.Pad(th.Space(kit.Space2), th.Space(kit.Space2p5), func(gtx C) D {
			return nav.Layout(gtx, &g.navGroup, func(gtx C) D {
				return th.List(gtx, &g.side, "sidebar", len(cat.rows), func(gtx C, r int) D {
					row := cat.rows[r]
					if row.header != "" {
						top := th.Space(kit.Space3p5)
						if r == 0 {
							top = th.Space(kit.Space0p5)
						}
						return layout.Inset{Top: top, Bottom: th.Space(kit.Space1), Left: th.Space(kit.Space2p5)}.Layout(gtx, func(gtx C) D {
							return th.Caption(gtx, row.header)
						})
					}
					p := cat.pages[row.page]
					trail, tc := "", th.Dim
					if p.id == "overview" {
						trail, tc = fmt.Sprint(g.openCount()), th.Warn
					}
					return layout.Inset{Bottom: navItemGap}.Layout(gtx, func(gtx C) D {
						return ui.NavItem{ID: "nav:" + p.id, Label: p.title, Trailing: trail, TrailColor: tc, Active: g.page == p.id}.Layout(gtx, th, g.nav.Get(p.id))
					})
				})
			})
		})(gtx)
	})
}

// navItemGap separates sidebar rows: a hairline, below the spacing scale.
const navItemGap unit.Dp = 1

// currentPage lays out the selected page.
func (g *gallery) currentPage(gtx C) D {
	p, ok := g.catalog.page(g.page)
	switch {
	case !ok:
		return D{}
	case p.layout != nil:
		return p.layout(g, gtx)
	default:
		return g.demoPage(gtx, p)
	}
}

// demoPage lays out a page's demos as cards in a scrolling column.
func (g *gallery) demoPage(gtx C, p page) D {
	var actions layout.Widget
	if p.actions != nil {
		actions = func(gtx C) D { return p.actions(g, gtx) }
	}
	return g.pageFrame(gtx, p.title, actions, func(gtx C) D {
		return g.th.List(gtx, g.pageLists.Get(p.id), p.id, len(p.demos), func(gtx C, i int) D {
			d := p.demos[i]
			return layout.Inset{Left: 24, Right: 24}.Layout(gtx, func(gtx C) D {
				return g.section(gtx, d.title, func(gtx C) D { return demoBody(g, gtx, d) })
			})
		})
	})
}

// allPage is the kitchen sink: every demo, in sidebar order, on one
// scrolling page, each card titled with its page.
func (g *gallery) allPage(gtx C) D {
	type entry struct {
		title string
		d     demo
	}
	var all []entry
	for _, p := range g.catalog.pages {
		for _, d := range p.demos {
			title := d.title
			if !strings.EqualFold(title, p.title) {
				title = p.title + ": " + d.title
			}
			all = append(all, entry{title, d})
		}
	}
	return g.pageFrame(gtx, "All components", nil, func(gtx C) D {
		return g.th.List(gtx, g.pageLists.Get("all"), "all", len(all), func(gtx C, i int) D {
			e := all[i]
			return layout.Inset{Left: 24, Right: 24}.Layout(gtx, func(gtx C) D {
				return g.section(gtx, e.title, func(gtx C) D { return demoBody(g, gtx, e.d) })
			})
		})
	})
}

// demoBody lays a demo out at its natural size: a demo that should fill the
// card's width (a text field row, a banner) asks for it itself.
func demoBody(g *gallery, gtx C, d demo) D {
	gtx.Constraints.Min = image.Point{}
	return d.body(g, gtx)
}
