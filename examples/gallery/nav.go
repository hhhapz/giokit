package main

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"

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

var categories = []category{
	{"Getting started", []page{
		{id: "overview", title: "Overview", layout: (*gallery).overview},
		{id: "all", title: "All components"}, // layout set in init: it lists pages
	}},
	{"Inputs", []page{
		{id: "buttons", title: "Buttons", demos: []demo{
			{"Buttons and size", (*gallery).demoButtons},
		}},
		{id: "selection", title: "Selection", demos: []demo{
			{"Switches and checkboxes", (*gallery).demoToggles},
			{"Slider", (*gallery).demoSlider},
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
		{id: "collections", title: "Lists, trees, tables", layout: (*gallery).workspace},
		{id: "logs", title: "Logs", layout: (*gallery).logs},
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
}

// pages is every page in sidebar order.
var pages []page

func init() {
	categories[0].pages[1].layout = (*gallery).allPage
	for _, c := range categories {
		pages = append(pages, c.pages...)
	}
}

// navRow is one sidebar row: a category header or a page.
type navRow struct {
	header string
	page   int // index into pages, when header == ""
}

var navRows = func() []navRow {
	var out []navRow
	i := 0
	for _, c := range categories {
		out = append(out, navRow{header: c.title})
		for range c.pages {
			out = append(out, navRow{page: i})
			i++
		}
	}
	return out
}()

// rowOfPage is the sidebar list index of pages[i].
func rowOfPage(i int) int {
	for r, row := range navRows {
		if row.header == "" && row.page == i {
			return r
		}
	}
	return 0
}

func (g *gallery) sidebar(gtx C) D {
	th := g.th
	ids := make([]string, len(pages))
	for i, p := range pages {
		ids[i] = p.id
	}
	return kit.Fill(gtx, th.Panel, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		return kit.Pad(8, 10, func(gtx C) D {
			return navKeys(gtx, &g.navGroup, func(i int) { g.side.ScrollIntoView(rowOfPage(i)) }, "sidebar", ids, g.page,
				func(id string) { g.page = id }, func(gtx C) D {
					return th.List(gtx, &g.side, "sidebar", len(navRows), func(gtx C, r int) D {
						row := navRows[r]
						if row.header != "" {
							top := unitDp(14)
							if r == 0 {
								top = 2
							}
							return layout.Inset{Top: top, Bottom: 4, Left: 10}.Layout(gtx, func(gtx C) D {
								return th.Caption(gtx, row.header)
							})
						}
						p := pages[row.page]
						trail, tc := "", th.Dim
						if p.id == "overview" {
							trail, tc = fmt.Sprint(g.openCount()), th.Warn
						}
						return layout.Inset{Bottom: 1}.Layout(gtx, func(gtx C) D {
							return ui.NavItem{ID: "nav:" + p.id, Label: p.title, Trailing: trail, TrailColor: tc, Active: g.page == p.id}.Layout(gtx, th, g.nav.Get(p.id))
						})
					})
				})
		})(gtx)
	})
}

// currentPage lays out the selected page.
func (g *gallery) currentPage(gtx C) D {
	for _, p := range pages {
		if p.id == g.page {
			if p.layout != nil {
				return p.layout(g, gtx)
			}
			return g.demoPage(gtx, p)
		}
	}
	return D{}
}

// demoPage lays out a page's demos as cards in a scrolling column.
func (g *gallery) demoPage(gtx C, p page) D {
	var actions kit.W
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
	for _, p := range pages {
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
