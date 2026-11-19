package main

import (
	"fmt"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/unit"
	"github.com/hhhapz/giokit/gio/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

// The workspace page is a small project organizer: folders, selectable work
// items, and an editable detail form using the same pieces as an actual app.
type workspaceState struct {
	folders   ui.TreeState
	items     ui.SelectableListState
	table     ui.DataTableState
	folderSel string // the selected folder
	selected  string // the selected record: the list and the table show it
	records   []workRecord
	priority  ui.NumberFieldState
	name      widget.Editor
	secret    widget.Editor
	save      kit.Clickable
	showError bool
}

// workRecord is one work item: the list, the table and the detail form all
// show it.
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

// The work table's columns, in order: indexes into workColumns and into
// each row's Cells.
const (
	workColName = iota
	workColStatus
)

var workColumns = []ui.DataColumn{
	workColName:   {ID: "item", Title: "Item"},
	workColStatus: {ID: "status", Title: "Status", Width: unit.Dp(110)},
}

// workCell draws the Status column as a badge (a custom cell renderer; the
// cell text still sorts the column and names the row).
func (g *gallery) workCell(th *kit.Theme) func(gtx C, r ui.DataRow, col int) D {
	return func(gtx C, r ui.DataRow, col int) D {
		if col != workColStatus {
			return th.Text(gtx, r.Cells[col], th.Type.Body, th.Fg, kit.Regular)
		}
		st := map[string]kit.Status{"Active": kit.StatusGood, "Review": kit.StatusWarn}[r.Cells[workColStatus]]
		return ui.Badge{Status: st, Text: r.Cells[workColStatus]}.Layout(gtx, th)
	}
}

// workChoices are the records as the work list's items.
func (g *gallery) workChoices() []ui.Choice {
	out := make([]ui.Choice, len(g.workspace.records))
	for i, r := range g.workspace.records {
		out[i] = ui.Choice{ID: r.ID, Label: r.Name}
	}
	return out
}

// workRows are the records as the work table's rows.
func (g *gallery) workRows() []ui.DataRow {
	out := make([]ui.DataRow, len(g.workspace.records))
	for i, r := range g.workspace.records {
		cells := make([]string, len(workColumns))
		cells[workColName], cells[workColStatus] = r.Name, r.Status
		out[i] = ui.DataRow{ID: r.ID, Cells: cells}
	}
	return out
}

// loadWork selects record id and fills the detail form from it. An unknown
// id changes nothing.
func (g *gallery) loadWork(id string) {
	for _, r := range g.workspace.records {
		if r.ID != id {
			continue
		}
		g.workspace.selected = id
		g.workspace.name.SetText(r.Name)
		g.workspace.priority.SetValue(r.Priority)
		g.workspace.secret.SetText(r.Secret)
		g.workspace.showError = false
		return
	}
}

func (g *gallery) initWorkspace() {
	g.workspace.records = []workRecord{
		{ID: "spec", Name: "Write specification", Status: "Active", Priority: 2},
		{ID: "icons", Name: "Review icons", Status: "Review", Priority: 3},
		{ID: "tests", Name: "Add interaction tests", Status: "Planned", Priority: 1},
		{ID: "release", Name: "Prepare release", Status: "Planned", Priority: 4},
	}
	g.workspace.folders.SetExpanded("project", true)
	g.workspace.name.SingleLine = true
	g.loadWork("spec")
}

func (g *gallery) handleWorkspace(gtx C) {
	if e, ok := g.workspace.folders.Update(gtx); ok {
		g.workspace.folderSel = e.Selected
		g.status = "Folder: " + e.Selected
	}
	if e, ok := g.workspace.items.Update(gtx); ok {
		g.loadWork(e.Selected)
		g.status = "Item: " + e.Selected
	}
	if e, ok := g.workspace.table.Update(gtx); ok {
		g.loadWork(e.Selected)
		g.status = "Table row: " + e.Selected
	}
	if _, ok := g.workspace.priority.Update(gtx); ok {
		g.status = fmt.Sprintf("Priority: %g", g.workspace.priority.Value())
	}
	if g.workspace.save.Clicked(gtx) {
		g.workspace.showError = true
		if g.th.Required("work:name", g.workspace.name.Text()).Message == "" && g.workspace.priority.Valid() {
			for i := range g.workspace.records {
				if g.workspace.records[i].ID == g.workspace.selected {
					g.workspace.records[i].Name = g.workspace.name.Text()
					g.workspace.records[i].Priority = g.workspace.priority.Value()
					g.workspace.records[i].Secret = g.workspace.secret.Text()
					break
				}
			}
			g.status = "Saved: " + g.workspace.name.Text()
		}
	}
}

func (g *gallery) workspacePage(gtx C) D {
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
								return ui.Tree{ID: "folders", Roots: workspaceFolders, Selected: g.workspace.folderSel}.Layout(gtx, th, &g.workspace.folders)
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
										return ui.SelectableList{ID: "work", Items: g.workChoices(), Selected: g.workspace.selected}.Layout(gtx, th, &g.workspace.items)
									}),
									kit.Rigid(kit.Gap(0, 8)),
									kit.Rigid(func(gtx C) D { return th.Caption(gtx, "Sortable table") }),
									kit.Rigid(kit.Gap(0, 6)),
									kit.Flexed(1, func(gtx C) D {
										return ui.DataTable{ID: "worktable", Columns: workColumns, Rows: g.workRows(), Selected: g.workspace.selected, Cell: g.workCell(th)}.Layout(gtx, th, &g.workspace.table)
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
										return ui.TextField{ID: "work:name", Placeholder: "Item name"}.Layout(gtx, th, &g.workspace.name)
									}),
									kit.Rigid(func(gtx C) D {
										if g.workspace.showError {
											return ui.FieldError{Message: g.th.Required("work:name", g.workspace.name.Text()).Message}.Layout(gtx, th)
										}
										return D{}
									}),
									kit.Rigid(kit.Gap(0, 8)),
									kit.Rigid(func(gtx C) D {
										return kit.Row(gtx,
											kit.Flexed(1, func(gtx C) D {
												return ui.NumberField{ID: "work:priority", Placeholder: "Priority", Min: 1, Max: 5}.Layout(gtx, th, &g.workspace.priority)
											}),
											kit.Rigid(kit.Gap(10, 0)),
											kit.Flexed(1, func(gtx C) D {
												return ui.TextField{ID: "work:secret", Placeholder: "Private token", Password: true}.Layout(gtx, th, &g.workspace.secret)
											}),
											kit.Rigid(kit.Gap(10, 0)),
											kit.Rigid(func(gtx C) D {
												return ui.Button{ID: "work:save", Text: "Save", Style: ui.Solid}.Layout(gtx, th, &g.workspace.save)
											}),
										)
									}),
									kit.Rigid(func(gtx C) D {
										return ui.FieldError{Message: g.workspace.priority.Error(th)}.Layout(gtx, th)
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
