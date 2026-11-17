package main

import (
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

type patternsState struct {
	patName, patNote             widget.Editor
	patSave, patReset, patLocked kit.Clickable
	patLink, patCreate           kit.Clickable
	patShowError, patCreated     bool
	patAccordion                 ui.AccordionState
	patOpen                      string
	patCrumbRoot, patCrumbGuide  kit.Clickable
}

func (g *gallery) initPatterns() { g.patName.SingleLine = true }

func (g *gallery) handlePatterns(gtx C) {
	for {
		if _, ok := g.patName.Update(gtx); !ok {
			break
		}
	}
	if g.patSave.Clicked(gtx) {
		g.patShowError = true
		if g.th.Required("patterns:name", g.patName.Text()).Message == "" {
			g.status = "Saved pattern: " + g.patName.Text()
		}
	}
	if g.patReset.Clicked(gtx) {
		g.patName.SetText("")
		g.patNote.SetText("")
		g.patShowError = false
		g.status = "Form reset"
	}
	if g.patLink.Clicked(gtx) {
		g.status = "Documentation link activated"
	}
	if g.patCreate.Clicked(gtx) {
		g.patCreated = true
		g.status = "New item created"
	}
	if g.patCrumbRoot.Clicked(gtx) {
		g.status = "Breadcrumb: Home"
	}
	if g.patCrumbGuide.Clicked(gtx) {
		g.status = "Breadcrumb: Guide"
	}
}

func (g *gallery) demoToolbar(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return ui.Toolbar{ID: "patterns:toolbar", Actions: []ui.ToolbarAction{
				{ID: "save", Text: "Save", Icon: kit.IconCheck, Clickable: &g.patSave, Style: ui.Solid},
				{ID: "reset", Text: "Reset", Clickable: &g.patReset},
				{ID: "unavailable", Text: "Unavailable", Clickable: &g.patLocked, Disabled: true},
			}}.Layout(gtx, th)
		}),
		kit.Rigid(kit.Gap(0, 8)),
		kit.Rigid(func(gtx C) D {
			return ui.Link{ID: "patterns:link", Text: "Read the component guide"}.Layout(gtx, th, &g.patLink)
		}),
	)
}

func (g *gallery) demoForm(gtx C) D {
	th := g.th
	nameError := ""
	if g.patShowError {
		nameError = g.th.Required("patterns:name", g.patName.Text()).Message
	}
	return ui.Form{ID: "patterns:form", Fields: []ui.FormField{
		{ID: "name", Label: "Name", Error: nameError, Control: func(gtx C) D {
			return ui.TextField{ID: "patterns:name", Placeholder: "Enter a name"}.Layout(gtx, th, &g.patName)
		}},
		{ID: "note", Label: "Note", Help: "Optional details", Control: func(gtx C) D {
			return ui.TextArea{ID: "patterns:note", Placeholder: "Add a note", Lines: 2}.Layout(gtx, th, &g.patNote)
		}},
	}}.Layout(gtx, th)
}

func (g *gallery) demoEmpty(gtx C) D {
	th := g.th
	if g.patCreated {
		return ui.Banner{Status: kit.StatusGood, Title: "Item created", Body: "The empty state action worked."}.Layout(gtx, th)
	}
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = gtx.Dp(160), gtx.Dp(160)
	return ui.EmptyState{ID: "patterns:empty", Title: "Nothing here yet", Body: "Create an item to get started.", Action: func(gtx C) D {
		return ui.Button{ID: "patterns:create", Text: "Create item", Style: ui.Primary}.Layout(gtx, th, &g.patCreate)
	}}.Layout(gtx, th)
}

func (g *gallery) demoBreadcrumbs(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return ui.Breadcrumbs{ID: "patterns:crumbs", Path: []ui.Breadcrumb{
				{ID: "home", Label: "Home", Clickable: &g.patCrumbRoot},
				{ID: "guide", Label: "Guide", Clickable: &g.patCrumbGuide},
				{ID: "components", Label: "Components"},
			}}.Layout(gtx, th)
		}),
		kit.Rigid(kit.Gap(0, 12)),
		kit.Rigid(func(gtx C) D {
			if id, ok := g.patAccordion.Update(gtx); ok {
				g.patOpen = id
			}
			return ui.Accordion{ID: "patterns:accordion", Sections: []ui.AccordionSection{
				{ID: "first", Title: "First section", Summary: "Details", Body: func(gtx C) D { return th.Paragraph(gtx, "The first section is open.", th.Muted) }},
				{ID: "second", Title: "Second section", Summary: "More", Body: func(gtx C) D { return th.Paragraph(gtx, "The second section is open.", th.Muted) }},
			}, Open: g.patOpen}.Layout(gtx, th, &g.patAccordion)
		}),
	)
}
