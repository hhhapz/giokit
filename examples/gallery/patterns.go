package main

import (
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

type patternsState struct {
	name, note         widget.Editor
	toolbar            ui.ToolbarState
	link, create       kit.Clickable
	showError, created bool
	accordion          ui.AccordionState
	open               string
	crumbs             ui.BreadcrumbsState
}

func (g *gallery) initPatterns() { g.patterns.name.SingleLine = true }

func (g *gallery) handlePatterns(gtx C) {
	for {
		if _, ok := g.patterns.name.Update(gtx); !ok {
			break
		}
	}
	switch id, _ := g.patterns.toolbar.Update(gtx); id {
	case "save":
		g.patterns.showError = true
		if g.th.Required("patterns:name", g.patterns.name.Text()).Message == "" {
			g.status = "Saved pattern: " + g.patterns.name.Text()
		}
	case "reset":
		g.patterns.name.SetText("")
		g.patterns.note.SetText("")
		g.patterns.showError = false
		g.status = "Form reset"
	}
	if g.patterns.link.Clicked(gtx) {
		g.status = "Documentation link activated"
	}
	if g.patterns.create.Clicked(gtx) {
		g.patterns.created = true
		g.status = "New item created"
	}
	switch id, _ := g.patterns.crumbs.Update(gtx); id {
	case "home":
		g.status = "Breadcrumb: Home"
	case "guide":
		g.status = "Breadcrumb: Guide"
	}
	if id, ok := g.patterns.accordion.Update(gtx); ok {
		g.patterns.open = id
	}
}

func (g *gallery) demoToolbar(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return ui.Toolbar{ID: "patterns:toolbar", Actions: []ui.ToolbarAction{
				{ID: "save", Text: "Save", Icon: kit.IconCheck, Style: ui.Solid},
				{ID: "reset", Text: "Reset"},
				{ID: "unavailable", Text: "Unavailable", Disabled: true},
			}}.Layout(gtx, th, &g.patterns.toolbar)
		}),
		kit.Rigid(kit.Gap(0, 8)),
		kit.Rigid(func(gtx C) D {
			return ui.Link{ID: "patterns:link", Text: "Read the component guide"}.Layout(gtx, th, &g.patterns.link)
		}),
	)
}

func (g *gallery) demoForm(gtx C) D {
	th := g.th
	nameError := ""
	if g.patterns.showError {
		nameError = g.th.Required("patterns:name", g.patterns.name.Text()).Message
	}
	return ui.Form{ID: "patterns:form", Fields: []ui.FormField{
		{ID: "name", Label: "Name", Error: nameError, Control: func(gtx C) D {
			return ui.TextField{ID: "patterns:name", Placeholder: "Enter a name"}.Layout(gtx, th, &g.patterns.name)
		}},
		{ID: "note", Label: "Note", Help: "Optional details", Control: func(gtx C) D {
			return ui.TextArea{ID: "patterns:note", Placeholder: "Add a note", Lines: 2}.Layout(gtx, th, &g.patterns.note)
		}},
	}}.Layout(gtx, th)
}

func (g *gallery) demoEmpty(gtx C) D {
	th := g.th
	if g.patterns.created {
		return ui.Banner{Status: kit.StatusGood, Title: "Item created", Body: "The empty state action worked."}.Layout(gtx, th)
	}
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = gtx.Dp(160), gtx.Dp(160)
	return ui.EmptyState{ID: "patterns:empty", Title: "Nothing here yet", Body: "Create an item to get started.", Action: func(gtx C) D {
		return ui.Button{ID: "patterns:create", Text: "Create item", Style: ui.Primary}.Layout(gtx, th, &g.patterns.create)
	}}.Layout(gtx, th)
}

func (g *gallery) demoBreadcrumbs(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return ui.Breadcrumbs{ID: "patterns:crumbs", Path: []ui.Breadcrumb{
				{ID: "home", Label: "Home"},
				{ID: "guide", Label: "Guide"},
				{ID: "components", Label: "Components"},
			}}.Layout(gtx, th, &g.patterns.crumbs)
		}),
		kit.Rigid(kit.Gap(0, 12)),
		kit.Rigid(func(gtx C) D {
			return ui.Accordion{ID: "patterns:accordion", Sections: []ui.AccordionSection{
				{ID: "first", Title: "First section", Summary: "Details", Body: func(gtx C) D { return th.Paragraph(gtx, "The first section is open.", th.Muted) }},
				{ID: "second", Title: "Second section", Summary: "More", Body: func(gtx C) D { return th.Paragraph(gtx, "The second section is open.", th.Muted) }},
			}, Open: g.patterns.open}.Layout(gtx, th, &g.patterns.accordion)
		}),
	)
}
