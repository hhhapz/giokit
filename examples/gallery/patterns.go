package main

import (
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
)

type patternsState struct {
	patName, patNote             widget.Editor
	patSave, patReset, patLocked kit.Clickable
	patLink, patCreate           kit.Clickable
	patShowError, patCreated     bool
	patAccordion                 kit.Accordion
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
		if kit.Required("patterns:name", g.patName.Text()).Message == "" {
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
			return th.Toolbar(gtx, "patterns:toolbar", []kit.ToolbarAction{
				{ID: "save", Label: "Save", Icon: kit.IconCheck, Clickable: &g.patSave, Kind: kit.Solid},
				{ID: "reset", Label: "Reset", Clickable: &g.patReset, Kind: kit.Secondary},
				{ID: "unavailable", Label: "Unavailable", Clickable: &g.patLocked, Kind: kit.Secondary, Disabled: true},
			})
		}),
		kit.Rigid(kit.Gap(0, 8)),
		kit.Rigid(func(gtx C) D { return th.Link(gtx, &g.patLink, "patterns:link", "Read the component guide") }),
	)
}

func (g *gallery) demoForm(gtx C) D {
	th := g.th
	nameError := ""
	if g.patShowError {
		nameError = kit.Required("patterns:name", g.patName.Text()).Message
	}
	return th.Form(gtx, "patterns:form", []kit.FormField{
		{ID: "name", Label: "Name", Error: nameError, Control: func(gtx C) D { return th.TextField(gtx, &g.patName, "patterns:name", "Enter a name") }},
		{ID: "note", Label: "Note", Help: "Optional details", Control: func(gtx C) D { return th.TextArea(gtx, &g.patNote, "patterns:note", "Add a note", 2) }},
	})
}

func (g *gallery) demoEmpty(gtx C) D {
	th := g.th
	if g.patCreated {
		return th.Banner(gtx, kit.StatusGood, "Item created", "The empty state action worked.")
	}
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = gtx.Dp(160), gtx.Dp(160)
	return th.EmptyState(gtx, "patterns:empty", "Nothing here yet", "Create an item to get started.", func(gtx C) D {
		return th.Button(gtx, &g.patCreate, "patterns:create", "Create item", kit.Primary)
	})
}

func (g *gallery) demoBreadcrumbs(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return th.Breadcrumbs(gtx, "patterns:crumbs", []kit.Breadcrumb{
				{ID: "home", Label: "Home", Clickable: &g.patCrumbRoot},
				{ID: "guide", Label: "Guide", Clickable: &g.patCrumbGuide},
				{ID: "components", Label: "Components"},
			})
		}),
		kit.Rigid(kit.Gap(0, 12)),
		kit.Rigid(func(gtx C) D {
			return th.Accordion(gtx, &g.patAccordion, "patterns:accordion", []kit.AccordionSection{
				{ID: "first", Title: "First section", Summary: "Details", Body: func(gtx C) D { return th.Paragraph(gtx, "The first section is open.", th.Muted) }},
				{ID: "second", Title: "Second section", Summary: "More", Body: func(gtx C) D { return th.Paragraph(gtx, "The second section is open.", th.Muted) }},
			})
		}),
	)
}
