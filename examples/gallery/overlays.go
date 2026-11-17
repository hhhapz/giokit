package main

import (
	"fmt"
	"time"

	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

// overlaysState: menus, selects, a context menu, a popover, a modal dialog
// and toasts. The dialog and toasts are hosted at the window root (main.go).
type overlaysState struct {
	fileMenu  ui.MenuState
	moreMenu  ui.MenuState
	ctxMenu   ui.MenuState
	sizeSel   ui.SelectState
	zone      ui.SelectState
	filters   kit.Popup
	filterBtn kit.Clickable
	onlyOpen  kit.Clickable
	mine      kit.Clickable
	apply     kit.Clickable
	openOnly  bool
	mineOnly  bool
	sizeI     int
	zoneI     int

	dialog    kit.Dialog
	delBtn    kit.Clickable
	cancelDel kit.Clickable
	confirm   kit.Clickable
	toasts    ui.Toasts
	toastBtns [3]kit.Clickable
	due       ui.DateFieldState
	dueDate   time.Time
	prof      ui.ComboboxState
	tagsOpen  ui.Collapsible
}

var (
	fileItems = []ui.MenuItem{
		{ID: "new", Label: "New file", Icon: kit.IconPlus, Shortcut: "Ctrl+N"},
		{ID: "open", Label: "Open…", Shortcut: "Ctrl+O"},
		{ID: "save", Label: "Save", Shortcut: "Ctrl+S", Disabled: true},
		{Divider: true},
		{ID: "delete", Label: "Delete project…", Icon: kit.IconX, Danger: true},
	}
	moreItems = []ui.MenuItem{
		{ID: "docs", Label: "Documentation", Icon: kit.IconInfo},
		{ID: "about", Label: "About giokit"},
	}
	ctxItems = []ui.MenuItem{
		{ID: "copy", Label: "Copy", Shortcut: "Ctrl+C"},
		{ID: "paste", Label: "Paste", Shortcut: "Ctrl+V"},
		{ID: "rename", Label: "Rename"},
		{Divider: true},
		{ID: "remove", Label: "Remove", Danger: true},
	}
	sizeOptions = []string{"Small", "Medium", "Large", "Extra large"}
	zoneNames   = []string{
		"UTC", "Europe/London", "Europe/Paris", "Europe/Berlin", "Europe/Helsinki", "Africa/Cairo",
		"Asia/Dubai", "Asia/Karachi", "Asia/Kolkata", "Asia/Bangkok", "Asia/Shanghai", "Asia/Tokyo",
		"Australia/Sydney", "Pacific/Auckland", "America/Sao_Paulo", "America/New_York",
		"America/Chicago", "America/Denver", "America/Los_Angeles", "Pacific/Honolulu",
	}
	toastKinds = []kit.Status{kit.StatusInfo, kit.StatusGood, kit.StatusBad}
	profNames  = []string{"Kathryn Hare", "Karen Yeats", "Lila Kari", "Ian Goulden", "Jeffrey Shallit", "Kevin Purbhoo", "Alfred Menezes"}
)

func (g *gallery) initOverlays() {
	g.sizeI, g.zoneI = 1, 0
	g.dueDate = time.Date(2026, 11, 20, 0, 0, 0, 0, time.Local)
}

func (g *gallery) handleOverlays(gtx C) {
	if id, ok := g.fileMenu.Update(gtx); ok {
		if id == "delete" {
			g.dialog.Show()
		} else {
			g.status = "File → " + id
		}
	}
	if id, ok := g.moreMenu.Update(gtx); ok {
		g.status = "More → " + id
	}
	if id, ok := g.ctxMenu.Update(gtx); ok {
		g.status = "Context → " + id
	}
	if i, ok := g.sizeSel.Update(gtx); ok {
		g.sizeI = i
	}
	if i, ok := g.zone.Update(gtx); ok {
		g.zoneI = i
	}
	if d, ok := g.due.Update(gtx); ok {
		g.dueDate = d
	}
	if g.filterBtn.Clicked(gtx) {
		g.filters.Toggle()
	}
	if g.onlyOpen.Clicked(gtx) {
		g.openOnly = !g.openOnly
	}
	if g.mine.Clicked(gtx) {
		g.mineOnly = !g.mineOnly
	}
	if g.apply.Clicked(gtx) {
		g.filters.Hide()
		g.filterBtn.FocusQuiet(gtx)
		g.status = fmt.Sprintf("Filters: open only %v, mine %v", g.openOnly, g.mineOnly)
	}
	if g.delBtn.Clicked(gtx) {
		g.dialog.Show()
	}
	if g.cancelDel.Clicked(gtx) {
		g.dialog.Hide()
	}
	if g.confirm.Clicked(gtx) {
		g.dialog.Hide()
		g.toasts.Push(kit.StatusGood, "Project deleted", "giokit and its 3 environments are gone.")
	}
	if g.dialog.DidDismiss() {
		g.status = "Delete cancelled"
	}
	for i := range g.toastBtns {
		if g.toastBtns[i].Clicked(gtx) {
			titles := []string{"Update available", "Saved", "Build failed"}
			bodies := []string{"Version 0.3 is ready to install.", "", "3 errors in kit/theme.go."}
			g.toasts.Push(toastKinds[i], titles[i], bodies[i])
		}
	}
}

// moreMenuButton is the Menus page's title action: a menu at the window's
// right edge, which must shift to stay inside.
func (g *gallery) moreMenuButton(gtx C) D {
	th := g.th
	return ui.Menu{ID: "more", Items: moreItems, Trigger: func(gtx C) D {
		return ui.IconButton{ID: "more:btn", Icon: kit.IconMenu, Label: "more:btn", Active: true}.Layout(gtx, th, &g.moreMenu.Trigger)
	}}.Layout(gtx, th, &g.moreMenu)
}

func (g *gallery) demoMenu(gtx C) D {
	th := g.th
	return ui.Menu{ID: "file", Items: fileItems, Trigger: func(gtx C) D {
		return ui.Button{ID: "file:btn", Text: "File", Icon: kit.IconChevronDown}.Layout(gtx, th, &g.fileMenu.Trigger)
	}}.Layout(gtx, th, &g.fileMenu)
}

func (g *gallery) demoPopover(gtx C) D {
	th := g.th
	return th.Popover(gtx, &g.filters, "filters", func(gtx C) D {
		return ui.Button{ID: "filters:btn", Text: "Filters"}.Layout(gtx, th, &g.filterBtn)
	}, func(gtx C) D {
		return layout.UniformInset(12).Layout(gtx, func(gtx C) D {
			return kit.Col(gtx,
				kit.Rigid(func(gtx C) D { return th.Caption(gtx, "Show") }),
				kit.Rigid(kit.Gap(0, 10)),
				kit.Rigid(func(gtx C) D {
					return ui.Switch{ID: "filters:open-only", Label: "Open items only", On: g.openOnly}.Layout(gtx, th, &g.onlyOpen)
				}),
				kit.Rigid(kit.Gap(0, 10)),
				kit.Rigid(func(gtx C) D {
					return ui.Checkbox{ID: "filters:mine", Label: "Assigned to me", Checked: g.mineOnly}.Layout(gtx, th, &g.mine)
				}),
				kit.Rigid(kit.Gap(0, 14)),
				kit.Rigid(func(gtx C) D {
					return ui.Button{ID: "filters:apply", Text: "Apply", Style: ui.Solid}.Layout(gtx, th, &g.apply)
				}),
			)
		})
	})
}

func (g *gallery) demoSelect(gtx C) D {
	th := g.th
	return kit.Row(gtx,
		kit.Rigid(kit.Fixed(200, func(gtx C) D {
			return ui.Select{ID: "size", Options: sizeOptions, Selected: g.sizeI}.Layout(gtx, th, &g.sizeSel)
		})),
		kit.Rigid(kit.Gap(10, 0)),
		kit.Rigid(kit.Fixed(240, func(gtx C) D {
			return ui.Select{ID: "zone", Options: zoneNames, Selected: g.zoneI}.Layout(gtx, th, &g.zone)
		})),
	)
}

func (g *gallery) demoDate(gtx C) D {
	return kit.Fixed(220, func(gtx C) D {
		return ui.DateField{ID: "due", Value: g.dueDate, Placeholder: "Pick a date"}.Layout(gtx, g.th, &g.due)
	})(gtx)
}

func (g *gallery) demoCombobox(gtx C) D {
	return kit.Fixed(280, func(gtx C) D {
		return ui.Combobox{ID: "prof", Placeholder: "Professor (type to search)", Options: profNames}.Layout(gtx, g.th, &g.prof)
	})(gtx)
}

func (g *gallery) demoDisclosure(gtx C) D {
	th := g.th
	g.tagsOpen.Update(gtx)
	return ui.Disclosure{ID: "tags", Title: "Known professors", Trailing: fmt.Sprint(len(profNames)), Open: g.tagsOpen.Open, Body: func(gtx C) D {
		var chips []kit.W
		for _, n := range profNames {
			chips = append(chips, func(gtx C) D { return th.Pill(gtx, n, th.Muted, th.Surface) })
		}
		return kit.Flow(gtx, 6, 6, chips...)
	}}.Layout(gtx, th, &g.tagsOpen.Clickable)
}

func (g *gallery) demoContextMenu(gtx C) D {
	th := g.th
	return ui.ContextMenu{ID: "ctx", Items: ctxItems, Content: func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return kit.Surface{Bg: th.Surface, Border: th.Border, Radius: th.Radius}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(24).Layout(gtx, func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					return th.Text(gtx, "Right-click anywhere in this box", th.Type.Body, th.Muted, kit.Regular)
				})
			})
		})
	}}.Layout(gtx, th, &g.ctxMenu)
}

func (g *gallery) demoDialogs(gtx C) D {
	th := g.th
	return kit.Flex{Gap: kit.Space2p5, Align: layout.Middle}.Layout(gtx, th,
		kit.Rigid(func(gtx C) D {
			return ui.Button{ID: "dialog:open", Text: "Delete project…", Style: ui.Danger}.Layout(gtx, th, &g.delBtn)
		}),
		kit.Rigid(func(gtx C) D { return ui.Button{ID: "toast:info", Text: "Info toast"}.Layout(gtx, th, &g.toastBtns[0]) }),
		kit.Rigid(func(gtx C) D {
			return ui.Button{ID: "toast:good", Text: "Success toast"}.Layout(gtx, th, &g.toastBtns[1])
		}),
		kit.Rigid(func(gtx C) D { return ui.Button{ID: "toast:bad", Text: "Error toast"}.Layout(gtx, th, &g.toastBtns[2]) }),
	)
}

// deleteDialog is the content of the confirm dialog hosted at the root.
func (g *gallery) deleteDialog(gtx C) D {
	th := g.th
	return ui.DialogContent{Title: "Delete project?", Body: func(gtx C) D {
		return th.Paragraph(gtx, "This removes giokit and its 3 environments. It can't be undone.", th.Muted)
	}, Actions: []kit.W{func(gtx C) D { return ui.Button{ID: "dialog:cancel", Text: "Cancel"}.Layout(gtx, th, &g.cancelDel) }, func(gtx C) D {
		return ui.Button{ID: "dialog:delete", Text: "Delete", Style: ui.Danger}.Layout(gtx, th, &g.confirm)
	}}}.Layout(gtx, th)
}
