package main

import (
	"fmt"
	"slices"
	"time"

	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

// overlaysState holds the overlay demos' state: menus, selects, a context
// menu, a popover, a modal dialog and toasts. The dialog and toasts are
// hosted at the window root (main.go).
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
	toastBtns [len(demoToasts)]kit.Clickable
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
	profNames = []string{"Kathryn Hare", "Karen Yeats", "Lila Kari", "Ian Goulden", "Jeffrey Shallit", "Kevin Purbhoo", "Alfred Menezes"}
)

// demoToast is a toast the Dialogs demo pushes from a button.
type demoToast struct {
	id     string // the button's id
	button string // the button's label
	status kit.Status
	title  string
	body   string // "" for a title-only toast
}

// demoToasts are the Dialogs demo's toast buttons, in display order.
var demoToasts = [...]demoToast{
	{id: "toast:info", button: "Info toast", status: kit.StatusInfo, title: "Update available", body: "Version 0.3 is ready to install."},
	{id: "toast:good", button: "Success toast", status: kit.StatusGood, title: "Saved"},
	{id: "toast:bad", button: "Error toast", status: kit.StatusBad, title: "Build failed", body: "3 errors in kit/theme.go."},
}

func (g *gallery) initOverlays() {
	g.overlays.sizeI, g.overlays.zoneI = 1, 0
	g.overlays.dueDate = time.Date(2026, 11, 20, 0, 0, 0, 0, time.Local)
}

func (g *gallery) handleOverlays(gtx C) {
	if id, ok := g.overlays.fileMenu.Update(gtx); ok {
		if id == "delete" {
			g.overlays.dialog.Show()
		} else {
			g.status = "File → " + id
		}
	}
	if id, ok := g.overlays.moreMenu.Update(gtx); ok {
		g.status = "More → " + id
	}
	if id, ok := g.overlays.ctxMenu.Update(gtx); ok {
		g.status = "Context → " + id
	}
	if id, ok := g.overlays.sizeSel.Update(gtx); ok {
		g.overlays.sizeI = slices.Index(sizeOptions, id)
	}
	if id, ok := g.overlays.zone.Update(gtx); ok {
		g.overlays.zoneI = slices.Index(zoneNames, id)
	}
	if d, ok := g.overlays.due.Update(gtx); ok {
		g.overlays.dueDate = d
	}
	g.overlays.tagsOpen.Update(gtx)
	if g.overlays.filterBtn.Clicked(gtx) {
		g.overlays.filters.Toggle()
	}
	if g.overlays.onlyOpen.Clicked(gtx) {
		g.overlays.openOnly = !g.overlays.openOnly
	}
	if g.overlays.mine.Clicked(gtx) {
		g.overlays.mineOnly = !g.overlays.mineOnly
	}
	if g.overlays.apply.Clicked(gtx) {
		g.overlays.filters.Hide()
		g.overlays.filterBtn.FocusQuiet(gtx)
		g.status = fmt.Sprintf("Filters: open only %v, mine %v", g.overlays.openOnly, g.overlays.mineOnly)
	}
	if g.overlays.delBtn.Clicked(gtx) {
		g.overlays.dialog.Show()
	}
	if g.overlays.cancelDel.Clicked(gtx) {
		g.overlays.dialog.Hide()
	}
	if g.overlays.confirm.Clicked(gtx) {
		g.overlays.dialog.Hide()
		g.overlays.toasts.Push(kit.StatusGood, "Project deleted", "giokit and its 3 environments are gone.")
	}
	if g.overlays.dialog.DidDismiss() {
		g.status = "Delete cancelled"
	}
	for i := range g.overlays.toastBtns {
		if g.overlays.toastBtns[i].Clicked(gtx) {
			t := demoToasts[i]
			g.overlays.toasts.Push(t.status, t.title, t.body)
		}
	}
}

// moreMenuButton is the Menus page's title action: a menu at the window's
// right edge, which must shift to stay inside.
func (g *gallery) moreMenuButton(gtx C) D {
	th := g.th
	return ui.Menu{ID: "more", Items: moreItems, Trigger: func(gtx C) D {
		return ui.IconButton{ID: "more:btn", Icon: kit.IconMenu, Label: "More actions", Active: true}.Layout(gtx, th, &g.overlays.moreMenu.Trigger)
	}}.Layout(gtx, th, &g.overlays.moreMenu)
}

func (g *gallery) demoMenu(gtx C) D {
	th := g.th
	return ui.Menu{ID: "file", Items: fileItems, Trigger: func(gtx C) D {
		return ui.Button{ID: "file:btn", Text: "File", Icon: kit.IconChevronDown}.Layout(gtx, th, &g.overlays.fileMenu.Trigger)
	}}.Layout(gtx, th, &g.overlays.fileMenu)
}

func (g *gallery) demoPopover(gtx C) D {
	th := g.th
	return th.Popover(gtx, &g.overlays.filters, "filters", func(gtx C) D {
		return ui.Button{ID: "filters:btn", Text: "Filters"}.Layout(gtx, th, &g.overlays.filterBtn)
	}, func(gtx C) D {
		return layout.UniformInset(12).Layout(gtx, func(gtx C) D {
			return kit.Col(gtx,
				kit.Rigid(func(gtx C) D { return th.Caption(gtx, "Show") }),
				kit.Rigid(kit.Gap(0, 10)),
				kit.Rigid(func(gtx C) D {
					return ui.Switch{ID: "filters:open-only", Label: "Open items only", On: g.overlays.openOnly}.Layout(gtx, th, &g.overlays.onlyOpen)
				}),
				kit.Rigid(kit.Gap(0, 10)),
				kit.Rigid(func(gtx C) D {
					return ui.Checkbox{ID: "filters:mine", Label: "Assigned to me", Checked: g.overlays.mineOnly}.Layout(gtx, th, &g.overlays.mine)
				}),
				kit.Rigid(kit.Gap(0, 14)),
				kit.Rigid(func(gtx C) D {
					return ui.Button{ID: "filters:apply", Text: "Apply", Style: ui.Solid}.Layout(gtx, th, &g.overlays.apply)
				}),
			)
		})
	})
}

func (g *gallery) demoSelect(gtx C) D {
	th := g.th
	return kit.Row(gtx,
		kit.Rigid(kit.Fixed(200, func(gtx C) D {
			return ui.Select{ID: "size", Options: choices(sizeOptions), Selected: sizeOptions[g.overlays.sizeI]}.Layout(gtx, th, &g.overlays.sizeSel)
		})),
		kit.Rigid(kit.Gap(10, 0)),
		kit.Rigid(kit.Fixed(240, func(gtx C) D {
			return ui.Select{ID: "zone", Options: choices(zoneNames), Selected: zoneNames[g.overlays.zoneI]}.Layout(gtx, th, &g.overlays.zone)
		})),
	)
}

func (g *gallery) demoDate(gtx C) D {
	return kit.Fixed(220, func(gtx C) D {
		return ui.DateField{ID: "due", Value: g.overlays.dueDate, Placeholder: "Pick a date"}.Layout(gtx, g.th, &g.overlays.due)
	})(gtx)
}

func (g *gallery) demoCombobox(gtx C) D {
	return kit.Fixed(280, func(gtx C) D {
		return ui.Combobox{ID: "prof", Placeholder: "Professor (type to search)", Options: choices(profNames)}.Layout(gtx, g.th, &g.overlays.prof)
	})(gtx)
}

func (g *gallery) demoDisclosure(gtx C) D {
	th := g.th
	return ui.Disclosure{ID: "tags", Title: "Known professors", Trailing: fmt.Sprint(len(profNames)), Open: g.overlays.tagsOpen.Open, Body: func(gtx C) D {
		var chips []layout.Widget
		for _, n := range profNames {
			chips = append(chips, func(gtx C) D { return th.Pill(gtx, n, th.Muted, th.Surface) })
		}
		return kit.Flow(gtx, 6, 6, chips...)
	}}.Layout(gtx, th, &g.overlays.tagsOpen.Clickable)
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
	}}.Layout(gtx, th, &g.overlays.ctxMenu)
}

func (g *gallery) demoDialogs(gtx C) D {
	th := g.th
	btns := []layout.FlexChild{kit.Rigid(func(gtx C) D {
		return ui.Button{ID: "dialog:open", Text: "Delete project…", Style: ui.Danger}.Layout(gtx, th, &g.overlays.delBtn)
	})}
	for i, t := range demoToasts {
		btns = append(btns, kit.Rigid(func(gtx C) D {
			return ui.Button{ID: t.id, Text: t.button}.Layout(gtx, th, &g.overlays.toastBtns[i])
		}))
	}
	return kit.Flex{Gap: kit.Space2p5, Align: layout.Middle}.Layout(gtx, th, btns...)
}

// deleteDialog is the content of the confirm dialog hosted at the root.
func (g *gallery) deleteDialog(gtx C) D {
	th := g.th
	return ui.DialogContent{Title: "Delete project?", Body: func(gtx C) D {
		return th.Paragraph(gtx, "This removes giokit and its 3 environments. It can't be undone.", th.Muted)
	}, Actions: []layout.Widget{func(gtx C) D {
		return ui.Button{ID: "dialog:cancel", Text: "Cancel"}.Layout(gtx, th, &g.overlays.cancelDel)
	}, func(gtx C) D {
		return ui.Button{ID: "dialog:delete", Text: "Delete", Style: ui.Danger}.Layout(gtx, th, &g.overlays.confirm)
	}}}.Layout(gtx, th)
}
