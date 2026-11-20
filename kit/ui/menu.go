package ui

import (
	"image"

	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/io/pointer"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/widget"

	"github.com/hhhapz/giokit/kit"
)

// MenuItem is one row of a Menu or ContextMenu. The zero value of each
// field is the common case.
type MenuItem struct {
	ID       string   // reported by Update; item ids are <menu id>/<ID>
	Label    string   // the row's visible and spoken text
	Icon     kit.Icon // optional, leading
	Shortcut string   // optional hint on the right ("Ctrl+S"); bind it in your Keymap
	Danger   bool     // destructive: drawn in Bad
	Disabled bool     // drawn dimmed; arrows skip it and it cannot be picked
	Divider  bool     // a separator line; other fields ignored
}

// MenuState is a Menu's or ContextMenu's interaction state. The zero
// value is closed; it holds event tags, so do not copy it after first use.
type MenuState struct {
	Trigger kit.Clickable // draw the Menu's trigger with it

	popup kit.Popup
	items kit.Pool[kit.Clickable]
	last  []MenuItem
	ctx   int // event tag: only its address matters, naming the context area
	group kit.FocusGroup
}

// Opened reports whether the menu is open.
func (m *MenuState) Opened() bool { return m.popup.Opened() }

// Show opens the menu: below its trigger, or for a ContextMenu where it
// last opened.
func (m *MenuState) Show() { m.popup.Show() }

// Hide closes the menu. Focus is not moved: a component closing it on the
// user's behalf returns focus itself.
func (m *MenuState) Hide() { m.popup.Hide() }

// Toggle opens a closed menu and closes an open one.
func (m *MenuState) Toggle() { m.popup.Toggle() }

// Update returns the id of an item chosen since the last call, and toggles
// the menu when its trigger is clicked. Call it in the handle pass. A pick
// closes the menu and returns focus to the trigger.
func (m *MenuState) Update(gtx layout.Context) (string, bool) {
	if m.Trigger.Clicked(gtx) {
		m.Toggle()
	}
	return m.picked(gtx)
}

// picked returns the id of an item clicked since the last call, closing
// the menu and returning focus to the trigger.
func (m *MenuState) picked(gtx layout.Context) (string, bool) {
	for _, it := range m.last {
		if it.Divider || it.Disabled {
			continue
		}
		c := m.items.Get(it.ID)
		m.group.Join(c, it.ID == m.group.Active)
		if c.Clicked(gtx) {
			m.Hide()
			m.Trigger.FocusQuiet(gtx)
			return it.ID, true
		}
	}
	return "", false
}

// Menu is a dropdown menu: Trigger (drawn with the state's Trigger
// Clickable) and, while open, the items in a floating panel below it.
//
//	if id, ok := v.file.Update(gtx); ok { ... }   // handle pass
//	ui.Menu{ID: "file", Items: items, Trigger: func(gtx layout.Context) layout.Dimensions {
//		return ui.Button{ID: "file:open", Text: "File", Style: ui.Ghost}.Layout(gtx, th, &v.file.Trigger)
//	}}.Layout(gtx, th, &v.file)
//
// Keyboard: Enter/Space on the trigger opens it; the open menu is one focus
// group: Down/Up enter and move through the items (Home/End jump); Enter
// picks; Escape closes; Tab closes and returns focus to the trigger.
type Menu struct {
	ID      string        // the panel's id; items are ID/<item ID>
	Items   []MenuItem    // the rows, top to bottom
	Trigger layout.Widget // draws the opener with the state's Trigger
}

// Layout draws Trigger and, while m is open, the item panel in a popover
// below it. It handles the panel's arrow keys, Enter and Tab; m.Update
// reports the pick and toggles the menu on trigger clicks.
func (mn Menu) Layout(gtx layout.Context, th *kit.Theme, m *MenuState) layout.Dimensions {
	m.last = mn.Items
	m.popup.Role = kit.RoleMenu
	m.opening()
	if d, byKey := m.popup.DidDismiss(); d {
		m.returnFocus(gtx, byKey)
	}
	trigger := func(gtx layout.Context) layout.Dimensions {
		// The trigger keeps its natural size: a caller's minimum (a window's)
		// would stretch it and leave the menu no room.
		gtx.Constraints.Min = image.Point{}
		return mn.Trigger(gtx)
	}
	return th.Popover(gtx, &m.popup, mn.ID, trigger, func(gtx layout.Context) layout.Dimensions { return m.panel(gtx, th, mn.ID, mn.Items) })
}

// ContextMenu lays out Content; a right-click inside it opens the menu at
// the pointer, and Shift+F10 (kit.ActContextMenu) while focus is inside it
// opens the menu at the content's top-left corner. Read picks with
// MenuState.Update, as for a Menu. The area's id is ID/area.
//
// Content has no focus of its own to return to: after Escape, focus is
// not restored to the control inside Content that had it.
type ContextMenu struct {
	ID      string        // the panel's id; the area is ID/area, items ID/<item ID>
	Items   []MenuItem    // the rows, top to bottom
	Content layout.Widget // the region that opens the menu
}

// Layout draws Content, opens m on a right-click or Shift+F10 inside it,
// and draws the open item panel at the pointer. m.Update reports the pick.
func (cm ContextMenu) Layout(gtx layout.Context, th *kit.Theme, m *MenuState) layout.Dimensions {
	m.last = cm.Items
	m.popup.Role = kit.RoleMenu
	m.opening()
	m.popup.DidDismiss() // no trigger to return focus to
	// Right presses anywhere inside the content bubble up to this area
	// (rule 1: read them before drawing).
	for {
		e, ok := gtx.Event(pointer.Filter{Target: &m.ctx, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pe, ok := e.(pointer.Event); ok && pe.Buttons.Contain(pointer.ButtonSecondary) {
			m.popup.ShowAt(pe.Position.Round())
			kit.Notify(gtx)
		}
	}
	return th.Popover(gtx, &m.popup, cm.ID, func(gtx layout.Context) layout.Dimensions {
		kit.OnAction(gtx, kit.ActContextMenu, func() { m.popup.ShowAt(image.Point{}) })
		mr := op.Record(gtx.Ops)
		dims := cm.Content(gtx)
		call := mr.Stop()
		defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
		event.Op(gtx.Ops, &m.ctx)
		kit.Tag(gtx, kit.Part(cm.ID, "area"))
		call.Add(gtx.Ops)
		return dims
	}, func(gtx layout.Context) layout.Dimensions { return m.panel(gtx, th, cm.ID, cm.Items) })
}

// opening points the popup's focus at the item group and, on a fresh open,
// clears the active item until a key moves. Call it before Popover, which
// consumes Opening.
func (m *MenuState) opening() {
	m.popup.FocusOnOpen(m.group.FocusHandle())
	if m.popup.Opening() {
		m.group.Active = ""
	}
}

// returnFocus gives focus back to the trigger when the menu closes: with a
// visible ring if a key closed it, quietly if the pointer did.
func (m *MenuState) returnFocus(gtx layout.Context, byKey bool) {
	if byKey {
		m.Trigger.Focus(gtx)
	} else {
		m.Trigger.FocusQuiet(gtx)
	}
}

// panel lays out the items and handles their keys.
func (m *MenuState) panel(gtx layout.Context, th *kit.Theme, id string, items []MenuItem) layout.Dimensions {
	var nav []string
	for _, it := range items {
		if !it.Divider && !it.Disabled {
			nav = append(nav, it.ID)
			m.group.Join(m.items.Get(it.ID), it.ID == m.group.Active)
		}
	}
	return m.keys(gtx, id, nav, nil, func(gtx layout.Context) layout.Dimensions { return m.rows(gtx, th, id, items) })
}

// keys makes the open panel one focus group over the items in nav (ids into
// m.items): arrows move the active item (entering from the first or last),
// Enter picks it, Tab closes the menu onto its trigger. l, if set, scrolls
// the active item into view.
func (m *MenuState) keys(gtx layout.Context, id string, nav []string, l *kit.List, w layout.Widget) layout.Dimensions {
	// cur is the active item's index. The handlers run after layout and
	// update it, so several keys in one frame each move on from the last.
	cur := -1
	for i, n := range nav {
		if n == m.group.Active {
			cur = i
		}
	}
	return kit.KeyContext(gtx, "menu", func(gtx layout.Context) layout.Dimensions {
		moveTo := func(to int) {
			if len(nav) == 0 {
				return
			}
			cur = max(0, min(len(nav)-1, to))
			m.group.Active = nav[cur]
			if l != nil {
				l.ScrollIntoView(cur)
			}
		}
		// With no active item, Down enters at the first and Up at the last.
		kit.OnAction(gtx, kit.ActMoveDown, func() {
			if cur < 0 {
				moveTo(0)
			} else {
				moveTo(cur + 1)
			}
		})
		kit.OnAction(gtx, kit.ActMoveUp, func() {
			if cur < 0 {
				moveTo(len(nav) - 1)
			} else {
				moveTo(cur - 1)
			}
		})
		kit.OnAction(gtx, kit.ActFirst, func() { moveTo(0) })
		kit.OnAction(gtx, kit.ActLast, func() { moveTo(len(nav) - 1) })
		kit.OnAction(gtx, kit.ActConfirm, func() {
			if cur >= 0 {
				m.items.Get(nav[cur]).Click() // reported by Update next frame
			}
		})
		// Tab leaves the menu: close it and land on the trigger.
		for _, f := range []event.Tag{m.group.FocusHandle(), m.popup.PanelFocus()} {
			for {
				e, ok := gtx.Event(key.Filter{Focus: f, Name: key.NameTab, Optional: key.ModShift})
				if !ok {
					break
				}
				if ke, ok := e.(key.Event); ok && ke.State == key.Press {
					m.Hide()
					m.returnFocus(gtx, true)
					kit.Notify(gtx)
				}
			}
		}
		return m.group.Layout(gtx, id, w)
	})
}

// rows draws the items as a column of menu rows and dividers, as wide as
// the widest row (at least Theme.MenuMinWidth) within the constraints.
func (m *MenuState) rows(gtx layout.Context, th *kit.Theme, id string, items []MenuItem) layout.Dimensions {
	// As wide as the widest row (at least MenuMinWidth), not the whole cap.
	pad := gtx.Dp(th.Space(kit.Space1))
	w := gtx.Dp(th.MenuMinWidth)
	for _, it := range items {
		if !it.Divider {
			w = max(w, th.MenuRowWidth(gtx, kit.MenuRow{Icon: it.Icon, Label: it.Label, Hint: it.Shortcut}))
		}
	}
	w += 2 * pad
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, max(gtx.Constraints.Min.X, w))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	children := make([]layout.FlexChild, 0, len(items))
	for _, it := range items {
		children = append(children, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			if it.Divider {
				return layout.Inset{Top: th.Space(kit.Space1), Bottom: th.Space(kit.Space1)}.Layout(gtx, kit.HLine(th.Divider))
			}
			return th.Disable(gtx, it.Disabled, func(gtx layout.Context) layout.Dimensions {
				return th.MenuRow(gtx, m.items.Get(it.ID), kit.Part(id, it.ID), kit.MenuRow{Icon: it.Icon, Label: it.Label, Hint: it.Shortcut, Danger: it.Danger})
			})
		}))
	}
	return kit.Flex{Axis: layout.Vertical, Pad: kit.Space1}.Layout(gtx, th, children...)
}

// Select is a dropdown field showing the selected option; it lists the
// options in a floating panel, the selected one checked and focused (so
// arrows start from it). Field id ID/field, options ID/<option ID>, the
// list ID/list (scrolls past Theme.ListMaxHeight). The panel is at least
// as wide as the field.
//
//	if id, ok := v.size.Update(gtx); ok { v.sizeID = id }   // handle pass
//	ui.Select{ID: "size", Options: sizes, Selected: v.sizeID}.Layout(gtx, th, &v.size)
type Select struct {
	ID string // the select's id; see the type doc for its parts
	// Options are the choices: Label is shown in the field, spoken, and drawn
	// unless Option is set; ID addresses the option and is what Update
	// reports. IDs must be unique and stable.
	Options  []Choice
	Selected string // the selected option's ID; "" (or an unknown ID) shows an empty field
	// Option, if set, draws an option's row content (an avatar beside a
	// name); the option's Label still names it for screen readers. It must
	// only draw (see kit.MenuRow.Content).
	Option func(gtx layout.Context, c Choice) layout.Dimensions
	Size   kit.ControlSize // ControlDefault is the theme's
}

// SelectState is a Select's interaction state. The zero value is closed;
// do not copy it after first use.
type SelectState struct {
	menu MenuState
	list kit.List
}

// Opened reports whether the option list is open.
func (s *SelectState) Opened() bool { return s.menu.Opened() }

// Show opens the option list.
func (s *SelectState) Show() { s.menu.Show() }

// Hide closes the option list.
func (s *SelectState) Hide() { s.menu.Hide() }

// Update returns the ID of an option picked since the last call, and
// toggles the dropdown when the field is clicked. Call it in the handle
// pass, then draw with the new selection. A pick closes the list and
// returns focus to the field.
func (s *SelectState) Update(gtx layout.Context) (string, bool) {
	if s.menu.Trigger.Clicked(gtx) {
		s.menu.Toggle()
	}
	return s.menu.picked(gtx)
}

// Layout draws the field with the selected option's label and, while s is
// open, the option list in a popover at least as wide as the field. It
// handles the list's keys; s.Update reports the pick and toggles the list
// on field clicks.
func (sl Select) Layout(gtx layout.Context, th *kit.Theme, s *SelectState) layout.Dimensions {
	m := &s.menu
	m.popup.MatchWidth = true
	m.last = m.last[:0]
	selected, label := -1, ""
	for i, o := range sl.Options {
		m.last = append(m.last, MenuItem{ID: o.ID, Label: o.Label})
		if o.ID == sl.Selected {
			selected, label = i, o.Label
		}
	}
	if d, byKey := m.popup.DidDismiss(); d {
		m.returnFocus(gtx, byKey)
	}
	opening := m.popup.Opening()
	m.opening()
	m.popup.Role = kit.RoleListbox
	return th.Popover(gtx, &m.popup, sl.ID, func(gtx layout.Context) layout.Dimensions {
		return th.Press(gtx, &m.Trigger, kit.Part(sl.ID, "field"), func(gtx layout.Context, st kit.State) layout.Dimensions {
			describe(gtx, kit.RoleCombobox, "") // spoken by its text
			st.Focused = st.Focused || m.Opened()
			return th.SizedField(gtx, sl.Size, st, func(gtx layout.Context) layout.Dimensions {
				return kit.Row(gtx,
					kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return th.Text(gtx, label, th.Type.Body, th.Fg, kit.Regular)
					}),
					kit.Rigid(func(gtx layout.Context) layout.Dimensions { return kit.IconChevronDown(gtx, th.IconSize, th.Muted) }),
				)
			})
		})
	}, func(gtx layout.Context) layout.Dimensions {
		nav := make([]string, len(sl.Options))
		for i, o := range sl.Options {
			nav[i] = o.ID
			m.group.Join(m.items.Get(o.ID), o.ID == m.group.Active)
		}
		return m.keys(gtx, sl.ID, nav, &s.list, func(gtx layout.Context) layout.Dimensions {
			if opening && selected >= 0 {
				// Start on the selected option, quietly: no ring until a key moves.
				m.group.Active = sl.Selected
				m.group.FocusQuiet(gtx)
				s.list.ScrollIntoView(selected)
			}
			return sl.options(gtx, th, s)
		})
	})
}

// row is an option's menu row, with the custom renderer if there is one.
func (sl Select) row(o Choice) kit.MenuRow {
	r := kit.MenuRow{Label: o.Label, Checked: o.ID == sl.Selected, Role: kit.RoleOption}
	if sl.Option != nil {
		r.Content = func(gtx layout.Context) layout.Dimensions { return sl.Option(gtx, o) }
	}
	return r
}

// options draws the open panel: the options as menu rows in a list that
// scrolls past Theme.ListMaxHeight, as wide as the widest option plus room
// for the scrollbar.
func (sl Select) options(gtx layout.Context, th *kit.Theme, s *SelectState) layout.Dimensions {
	pad := gtx.Dp(th.Space(kit.Space1))
	w := 0
	for _, o := range sl.Options {
		w = max(w, th.MenuRowWidth(gtx, sl.row(o)))
	}
	w += 2*pad + gtx.Dp(th.HitSize) // the panel's inset, room for the scrollbar
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, max(gtx.Constraints.Min.X, w))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.UniformInset(th.Space(kit.Space1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(th.ListMaxHeight))
		return th.List(gtx, &s.list, kit.Part(sl.ID, "list"), len(sl.Options), func(gtx layout.Context, i int) layout.Dimensions {
			o := sl.Options[i]
			return th.MenuRow(gtx, s.menu.items.Get(o.ID), kit.Part(sl.ID, o.ID), sl.row(o))
		})
	})
}

// CommandPalette is a modal, searchable action picker over Actions. Call
// state.Open from a shortcut, and wrap the window's Page with it each
// frame. Query field ID/query, results ID/<action ID>.
type CommandPalette struct {
	ID      string        // the dialog's id; see the type doc for its parts
	Actions *kit.Actions  // searched with Actions.Search; must not be nil
	Page    layout.Widget // the window's content, drawn under the dialog
}

// CommandPaletteState is a CommandPalette's state: its dialog, query and
// results. The zero value is closed; like other states it holds event tags,
// so do not copy it after first use.
type CommandPaletteState struct {
	// Dialog is the palette's modal. Prefer Open and Close, which also
	// reset the query and move focus.
	Dialog kit.Dialog
	// Editor holds the query text.
	Editor    widget.Editor
	items     kit.Pool[kit.Clickable]
	matches   []kit.Action
	selected  int
	focusNext bool
}

// Open shows the palette with an empty query.
func (p *CommandPaletteState) Open() {
	p.Editor.SetText("")
	p.selected = 0
	p.focusNext = true
	p.Dialog.Show()
}

// Close hides the palette.
func (p *CommandPaletteState) Close() { p.Dialog.Hide() }

// Layout draws Page and, while p is open, the modal palette over it. It
// handles input itself: typing filters the results, Up/Down move the
// highlight (wrapping), and Enter or a click runs the action with
// Actions.Run and closes the palette. There is no Update.
func (cp CommandPalette) Layout(gtx layout.Context, th *kit.Theme, p *CommandPaletteState) layout.Dimensions {
	if p.Dialog.Opened() {
		cp.handle(gtx, p)
	}
	return th.Modal(gtx, &p.Dialog, cp.ID, cp.Page, func(gtx layout.Context) layout.Dimensions {
		if p.focusNext {
			p.focusNext = false
			gtx.Execute(key.FocusCmd{Tag: &p.Editor})
		}
		return kit.Col(gtx,
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return th.Text(gtx, th.Strings.Commands, th.Type.Title, th.Fg, kit.SemiBold)
			}),
			kit.Rigid(kit.Gap(0, th.Space(kit.Space3))),
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				p.Editor.SingleLine = true
				return th.EditorField(gtx, &p.Editor, kit.Part(cp.ID, "query"), kit.EditorField{Placeholder: th.Strings.CommandPlaceholder, Icon: kit.IconSearch})
			}),
			kit.Rigid(kit.Gap(0, th.Space(kit.Space2))),
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				var rows []layout.FlexChild
				for i, act := range p.matches {
					rows = append(rows, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return th.MenuRow(gtx, p.items.Get(act.ID), kit.Part(cp.ID, act.ID), kit.MenuRow{Label: act.Title, Hint: act.Category, Highlighted: i == p.selected, Role: kit.RoleOption})
					}))
				}
				if len(rows) == 0 {
					rows = append(rows, kit.Rigid(func(gtx layout.Context) layout.Dimensions { return th.Paragraph(gtx, th.Strings.NoCommands, th.Muted) }))
				}
				return kit.Col(gtx, rows...)
			}),
		)
	})
}

// handle runs clicks and the query field's keys (focus stays in the field,
// so Down/Up/Enter are read from it).
func (cp CommandPalette) handle(gtx layout.Context, p *CommandPaletteState) {
	// Actions may have been disabled since the matches were found: search
	// again before any event indexes them.
	cp.search(p)
	for _, act := range p.matches {
		if p.items.Get(act.ID).Clicked(gtx) {
			cp.run(gtx, p, act.ID)
			return
		}
	}
	for {
		e, ok := gtx.Event(key.Filter{Focus: &p.Editor, Name: key.NameDownArrow},
			key.Filter{Focus: &p.Editor, Name: key.NameUpArrow},
			key.Filter{Focus: &p.Editor, Name: key.NameReturn}, key.Filter{Focus: &p.Editor, Name: key.NameEnter})
		if !ok {
			break
		}
		ke, ok := e.(key.Event)
		if !ok || ke.State != key.Press || len(p.matches) == 0 {
			continue
		}
		switch ke.Name {
		case key.NameDownArrow:
			p.selected = (p.selected + 1) % len(p.matches)
		case key.NameUpArrow:
			p.selected = (p.selected - 1 + len(p.matches)) % len(p.matches)
		default:
			cp.run(gtx, p, p.matches[p.selected].ID)
			return
		}
		kit.Notify(gtx)
	}
	for {
		e, ok := p.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := e.(widget.ChangeEvent); ok {
			p.selected = 0
			kit.Notify(gtx)
		}
	}
	cp.search(p)
}

// search finds the results for the query and keeps the selection inside
// them.
func (cp CommandPalette) search(p *CommandPaletteState) {
	p.matches = cp.Actions.Search(p.Editor.Text())
	if len(p.matches) > maxSuggestions {
		p.matches = p.matches[:maxSuggestions]
	}
	p.selected = max(0, min(p.selected, len(p.matches)-1))
}

// run closes the palette and runs the action. Events after it in this
// frame are left unread: the palette is closed.
func (cp CommandPalette) run(gtx layout.Context, p *CommandPaletteState, id string) {
	p.Close()
	cp.Actions.Run(id)
	kit.Notify(gtx)
}
