package main

import (
	"fmt"
	"time"

	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
)

// overlaysState: menus, selects, a context menu, a popover, a modal dialog
// and toasts. The dialog and toasts are hosted at the window root (main.go).
type overlaysState struct {
	fileMenu  kit.Menu
	moreMenu  kit.Menu
	ctxMenu   kit.Menu
	sizeSel   kit.Select
	zone      kit.Select
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
	toasts    kit.Toasts
	toastBtns [3]kit.Clickable
	due       kit.DatePicker
	dueDate   time.Time
	prof      kit.Combobox
	tagsOpen  kit.Collapsible
}

var (
	fileItems = []kit.MenuItem{
		{ID: "new", Label: "New file", Icon: kit.IconPlus, Shortcut: "Ctrl+N"},
		{ID: "open", Label: "Open…", Shortcut: "Ctrl+O"},
		{ID: "save", Label: "Save", Shortcut: "Ctrl+S", Disabled: true},
		{Divider: true},
		{ID: "delete", Label: "Delete project…", Icon: kit.IconX, Danger: true},
	}
	moreItems = []kit.MenuItem{
		{ID: "docs", Label: "Documentation", Icon: kit.IconInfo},
		{ID: "about", Label: "About giokit"},
	}
	ctxItems = []kit.MenuItem{
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
	if id, ok := g.fileMenu.Clicked(gtx); ok {
		if id == "delete" {
			g.dialog.Show()
		} else {
			g.status = "File → " + id
		}
	}
	if id, ok := g.moreMenu.Clicked(gtx); ok {
		g.status = "More → " + id
	}
	if id, ok := g.ctxMenu.Clicked(gtx); ok {
		g.status = "Context → " + id
	}
	if i, ok := g.sizeSel.Changed(gtx); ok {
		g.sizeI = i
	}
	if i, ok := g.zone.Changed(gtx); ok {
		g.zoneI = i
	}
	if d, ok := g.due.Changed(gtx); ok {
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
	return th.Menu(gtx, &g.moreMenu, "more", func(gtx C) D {
		return th.IconButton(gtx, &g.moreMenu.Trigger, "more:btn", kit.IconMenu, true)
	}, moreItems)
}

func (g *gallery) demoMenu(gtx C) D {
	th := g.th
	return th.Menu(gtx, &g.fileMenu, "file", func(gtx C) D {
		return th.IconLabelButton(gtx, &g.fileMenu.Trigger, "file:btn", kit.IconChevronDown, "File", kit.Secondary)
	}, fileItems)
}

func (g *gallery) demoPopover(gtx C) D {
	th := g.th
	return th.Popover(gtx, &g.filters, "filters", func(gtx C) D {
		return th.Button(gtx, &g.filterBtn, "filters:btn", "Filters", kit.Secondary)
	}, func(gtx C) D {
		return layout.UniformInset(12).Layout(gtx, func(gtx C) D {
			return kit.Col(gtx,
				kit.Rigid(func(gtx C) D { return th.Caption(gtx, "Show") }),
				kit.Rigid(kit.Gap(0, 10)),
				kit.Rigid(func(gtx C) D { return th.Switch(gtx, &g.onlyOpen, "filters:open-only", "Open items only", g.openOnly) }),
				kit.Rigid(kit.Gap(0, 10)),
				kit.Rigid(func(gtx C) D { return th.CheckboxLabel(gtx, &g.mine, "filters:mine", "Assigned to me", g.mineOnly) }),
				kit.Rigid(kit.Gap(0, 14)),
				kit.Rigid(func(gtx C) D { return th.Button(gtx, &g.apply, "filters:apply", "Apply", kit.Solid) }),
			)
		})
	})
}

func (g *gallery) demoSelect(gtx C) D {
	th := g.th
	return kit.Row(gtx,
		kit.Rigid(kit.Fixed(200, func(gtx C) D { return th.Select(gtx, &g.sizeSel, "size", sizeOptions, g.sizeI) })),
		kit.Rigid(kit.Gap(10, 0)),
		kit.Rigid(kit.Fixed(240, func(gtx C) D { return th.Select(gtx, &g.zone, "zone", zoneNames, g.zoneI) })),
	)
}

func (g *gallery) demoDate(gtx C) D {
	return kit.Fixed(220, func(gtx C) D { return g.th.DateField(gtx, &g.due, "due", g.dueDate, "Pick a date") })(gtx)
}

func (g *gallery) demoCombobox(gtx C) D {
	return kit.Fixed(280, func(gtx C) D { return g.th.Combobox(gtx, &g.prof, "prof", "Professor (type to search)", profNames) })(gtx)
}

func (g *gallery) demoDisclosure(gtx C) D {
	th := g.th
	return th.Disclosure(gtx, &g.tagsOpen, "tags", "Known professors", fmt.Sprint(len(profNames)), func(gtx C) D {
		var chips []kit.W
		for _, n := range profNames {
			chips = append(chips, func(gtx C) D { return th.Pill(gtx, n, th.Muted, th.Surface) })
		}
		return kit.Flow(gtx, 6, 6, chips...)
	})
}

func (g *gallery) demoContextMenu(gtx C) D {
	th := g.th
	return th.ContextArea(gtx, &g.ctxMenu, "ctx", func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return kit.Surface{Bg: th.Surface, Border: th.Border, Radius: th.Radius}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(24).Layout(gtx, func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					return th.Text(gtx, "Right-click anywhere in this box", th.Type.Body, th.Muted, kit.Regular)
				})
			})
		})
	}, ctxItems)
}

func (g *gallery) demoDialogs(gtx C) D {
	th := g.th
	return hrow(gtx, 10,
		func(gtx C) D { return th.Button(gtx, &g.delBtn, "dialog:open", "Delete project…", kit.Danger) },
		func(gtx C) D { return th.Button(gtx, &g.toastBtns[0], "toast:info", "Info toast", kit.Secondary) },
		func(gtx C) D { return th.Button(gtx, &g.toastBtns[1], "toast:good", "Success toast", kit.Secondary) },
		func(gtx C) D { return th.Button(gtx, &g.toastBtns[2], "toast:bad", "Error toast", kit.Secondary) },
	)
}

// deleteDialog is the content of the confirm dialog hosted at the root.
func (g *gallery) deleteDialog(gtx C) D {
	th := g.th
	return th.DialogContent(gtx, "Delete project?", func(gtx C) D {
		return th.Paragraph(gtx, "This removes giokit and its 3 environments. It can't be undone.", th.Muted)
	},
		func(gtx C) D { return th.Button(gtx, &g.cancelDel, "dialog:cancel", "Cancel", kit.Secondary) },
		func(gtx C) D { return th.Button(gtx, &g.confirm, "dialog:delete", "Delete", kit.Danger) },
	)
}
