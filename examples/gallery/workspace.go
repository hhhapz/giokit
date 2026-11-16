package main

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
)

// The workspace page is a small project organizer: folders, selectable work
// items, and an editable detail form using the same pieces as an actual app.
type workspaceState struct {
	folders   kit.Tree
	workItems kit.SelectableList
	workTable kit.DataTable
	records   []workRecord
	priority  kit.NumberField
	workName  widget.Editor
	secret    widget.Editor
	save      kit.Clickable
	showError bool
}

type workRecord struct {
	ID, Name, Status, Secret string
	Priority                 float64
}

var workspaceFolders = []kit.TreeNode{
	{ID: "project", Label: "Project", Children: []kit.TreeNode{
		{ID: "design", Label: "Design"}, {ID: "build", Label: "Build"}, {ID: "review", Label: "Review"},
	}},
	{ID: "archive", Label: "Archive"},
}

func (g *gallery) workChoices() []kit.Choice {
	out := make([]kit.Choice, len(g.records))
	for i, r := range g.records {
		out[i] = kit.Choice{ID: r.ID, Label: r.Name}
	}
	return out
}
func (g *gallery) workRows() []kit.DataRow {
	out := make([]kit.DataRow, len(g.records))
	for i, r := range g.records {
		out[i] = kit.DataRow{ID: r.ID, Cells: []string{r.Name, r.Status}}
	}
	return out
}
func (g *gallery) loadWork(id string) {
	for _, r := range g.records {
		if r.ID != id {
			continue
		}
		g.workItems.Selected, g.workTable.Selected = id, id
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
	g.workTable.Table.Cols = []kit.Column{{Title: "Item"}, {Title: "Status", Width: unit.Dp(110)}}
	g.priority.Min, g.priority.Max = 1, 5
	g.workName.SingleLine = true
	g.loadWork("spec")
}

func (g *gallery) handleWorkspace(gtx C) {
	if id, ok := g.folders.Changed(); ok {
		g.status = "Folder: " + id
	}
	if id, ok := g.workItems.Changed(); ok {
		g.loadWork(id)
		g.status = "Item: " + id
	}
	if id, ok := g.workTable.Changed(); ok {
		g.loadWork(id)
		g.status = "Table row: " + id
	}
	if _, ok := g.priority.Changed(); ok {
		g.status = fmt.Sprintf("Priority: %g", g.priority.Value)
	}
	if g.save.Clicked(gtx) {
		g.showError = true
		if kit.Required("work:name", g.workName.Text()).Message == "" && g.priority.Error == "" {
			for i := range g.records {
				if g.records[i].ID == g.workItems.Selected {
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
							kit.Flexed(1, func(gtx C) D { return th.Tree(gtx, &g.folders, "folders", workspaceFolders) }),
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
										return th.SelectableList(gtx, &g.workItems, "work", g.workChoices())
									}),
									kit.Rigid(kit.Gap(0, 8)),
									kit.Rigid(func(gtx C) D { return th.Caption(gtx, "Sortable table") }),
									kit.Rigid(kit.Gap(0, 6)),
									kit.Flexed(1, func(gtx C) D { return th.DataTable(gtx, &g.workTable, "worktable", g.workRows()) }),
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
									kit.Rigid(func(gtx C) D { return th.TextField(gtx, &g.workName, "work:name", "Item name") }),
									kit.Rigid(func(gtx C) D {
										if g.showError {
											return th.FieldError(gtx, kit.Required("work:name", g.workName.Text()))
										}
										return D{}
									}),
									kit.Rigid(kit.Gap(0, 8)),
									kit.Rigid(func(gtx C) D {
										return kit.Row(gtx,
											kit.Flexed(1, func(gtx C) D { return th.NumberField(gtx, &g.priority, "work:priority", "Priority") }),
											kit.Rigid(kit.Gap(10, 0)),
											kit.Flexed(1, func(gtx C) D { return th.PasswordField(gtx, &g.secret, "work:secret", "Private token") }),
											kit.Rigid(kit.Gap(10, 0)),
											kit.Rigid(func(gtx C) D { return th.Button(gtx, &g.save, "work:save", "Save", kit.Solid) }),
										)
									}),
									kit.Rigid(func(gtx C) D {
										return th.FieldError(gtx, kit.Validation{FieldID: "work:priority", Message: g.priority.Error})
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
