package main

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

// The workspace page is a small project organizer: folders, selectable work
// items, and an editable detail form using the same pieces as an actual app.
type workspaceState struct {
	folders   ui.TreeState
	workItems ui.SelectableListState
	workTable ui.DataTableState
	folderSel string // the selected folder
	workSel   string // the selected record: the list and the table show it
	records   []workRecord
	priority  ui.NumberFieldState
	workName  widget.Editor
	secret    widget.Editor
	save      kit.Clickable
	showError bool
}

type workRecord struct {
	ID, Name, Status, Secret string
	Priority                 float64
}

var workspaceFolders = []ui.TreeNode{
	{ID: "project", Label: "Project", Children: []ui.TreeNode{
		{ID: "design", Label: "Design"}, {ID: "build", Label: "Build"}, {ID: "review", Label: "Review"},
	}},
	{ID: "archive", Label: "Archive"},
}

var workColumns = []kit.Column{{Title: "Item"}, {Title: "Status", Width: unit.Dp(110)}}

func (g *gallery) workChoices() []ui.Choice {
	out := make([]ui.Choice, len(g.records))
	for i, r := range g.records {
		out[i] = ui.Choice{ID: r.ID, Label: r.Name}
	}
	return out
}
func (g *gallery) workRows() []ui.DataRow {
	out := make([]ui.DataRow, len(g.records))
	for i, r := range g.records {
		out[i] = ui.DataRow{ID: r.ID, Cells: []string{r.Name, r.Status}}
	}
	return out
}
func (g *gallery) loadWork(id string) {
	for _, r := range g.records {
		if r.ID != id {
			continue
		}
		g.workSel = id
		g.workName.SetText(r.Name)
		g.priority.SetValue(r.Priority)
		g.secret.SetText(r.Secret)
		g.showError = false
		return
	}
}

func (g *gallery) initWorkspace() {
	g.records = []workRecord{
		{ID: "spec", Name: "Write specification", Status: "Active", Priority: 2},
		{ID: "icons", Name: "Review icons", Status: "Review", Priority: 3},
		{ID: "tests", Name: "Add interaction tests", Status: "Planned", Priority: 1},
		{ID: "release", Name: "Prepare release", Status: "Planned", Priority: 4},
	}
	g.folders.SetExpanded("project", true)
	g.workName.SingleLine = true
	g.loadWork("spec")
}

func (g *gallery) handleWorkspace(gtx C) {
	if id, ok := g.folders.Update(gtx); ok {
		g.folderSel = id
		g.status = "Folder: " + id
	}
	if id, ok := g.workItems.Update(gtx); ok {
		g.loadWork(id)
		g.status = "Item: " + id
	}
	if id, ok := g.workTable.Update(gtx); ok {
		g.loadWork(id)
		g.status = "Table row: " + id
	}
	if _, ok := g.priority.Update(gtx); ok {
		g.status = fmt.Sprintf("Priority: %g", g.priority.Value)
	}
	if g.save.Clicked(gtx) {
		g.showError = true
		if g.th.Required("work:name", g.workName.Text()).Message == "" && g.priority.Error == "" {
			for i := range g.records {
				if g.records[i].ID == g.workSel {
					g.records[i].Name = g.workName.Text()
					g.records[i].Priority = g.priority.Value
					g.records[i].Secret = g.secret.Text()
					break
				}
			}
			g.status = "Saved: " + g.workName.Text()
		}
	}
}

func (g *gallery) workspace(gtx C) D {
	th := g.th
	return g.pageFrame(gtx, "Workspace", nil, func(gtx C) D {
		return layout.Inset{Left: 24, Right: 24, Bottom: 20}.Layout(gtx, func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(kit.Fixed(240, func(gtx C) D {
					return th.Card(gtx, func(gtx C) D {
						gtx.Constraints.Min = gtx.Constraints.Max
						return kit.Col(gtx,
							kit.Rigid(func(gtx C) D { return th.Caption(gtx, "Folders") }),
							kit.Rigid(kit.Gap(0, 8)),
							kit.Flexed(1, func(gtx C) D {
								return ui.Tree{ID: "folders", Roots: workspaceFolders, Selected: g.folderSel}.Layout(gtx, th, &g.folders)
							}),
						)
					})
				})),
				kit.Rigid(kit.Gap(14, 0)),
				kit.Flexed(1, func(gtx C) D {
					return kit.Col(gtx,
						kit.Flexed(1, func(gtx C) D {
							return th.Card(gtx, func(gtx C) D {
								gtx.Constraints.Min = gtx.Constraints.Max
								return kit.Col(gtx,
									kit.Rigid(func(gtx C) D { return th.Caption(gtx, "Work items") }),
									kit.Rigid(kit.Gap(0, 8)),
									kit.Rigid(func(gtx C) D {
										gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = gtx.Dp(145), gtx.Dp(145)
										return ui.SelectableList{ID: "work", Items: g.workChoices(), Selected: g.workSel}.Layout(gtx, th, &g.workItems)
									}),
									kit.Rigid(kit.Gap(0, 8)),
									kit.Rigid(func(gtx C) D { return th.Caption(gtx, "Sortable table") }),
									kit.Rigid(kit.Gap(0, 6)),
									kit.Flexed(1, func(gtx C) D {
										return ui.DataTable{ID: "worktable", Columns: workColumns, Rows: g.workRows(), Selected: g.workSel}.Layout(gtx, th, &g.workTable)
									}),
								)
							})
						}),
						kit.Rigid(kit.Gap(0, 12)),
						kit.Rigid(func(gtx C) D {
							return th.Card(gtx, func(gtx C) D {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return kit.Col(gtx,
									kit.Rigid(func(gtx C) D { return th.Caption(gtx, "Edit item") }),
									kit.Rigid(kit.Gap(0, 10)),
									kit.Rigid(func(gtx C) D {
										return ui.TextField{ID: "work:name", Placeholder: "Item name"}.Layout(gtx, th, &g.workName)
									}),
									kit.Rigid(func(gtx C) D {
										if g.showError {
											return ui.FieldError{Message: g.th.Required("work:name", g.workName.Text()).Message}.Layout(gtx, th)
										}
										return D{}
									}),
									kit.Rigid(kit.Gap(0, 8)),
									kit.Rigid(func(gtx C) D {
										return kit.Row(gtx,
											kit.Flexed(1, func(gtx C) D {
												return ui.NumberField{ID: "work:priority", Placeholder: "Priority", Min: 1, Max: 5}.Layout(gtx, th, &g.priority)
											}),
											kit.Rigid(kit.Gap(10, 0)),
											kit.Flexed(1, func(gtx C) D {
												return ui.TextField{ID: "work:secret", Placeholder: "Private token", Password: true}.Layout(gtx, th, &g.secret)
											}),
											kit.Rigid(kit.Gap(10, 0)),
											kit.Rigid(func(gtx C) D {
												return ui.Button{ID: "work:save", Text: "Save", Style: ui.Solid}.Layout(gtx, th, &g.save)
											}),
										)
									}),
									kit.Rigid(func(gtx C) D {
										return ui.FieldError{Message: g.priority.Error}.Layout(gtx, th)
									}),
								)
							})
						}),
					)
				}),
			)
		})
	})
}
