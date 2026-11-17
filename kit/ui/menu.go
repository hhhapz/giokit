package ui

import (
	"image"
	"strconv"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
)

// MenuItem is one row of a Menu or ContextMenu. The zero value of each
// field is the common case.
type MenuItem struct {
	ID       string // reported by Update; item ids are <menu id>/<ID>
	Label    string
	Icon     kit.Icon // optional, leading
	Shortcut string   // optional hint on the right ("Ctrl+S"); bind it in your Keymap
	Danger   bool     // destructive: drawn in Bad
	Disabled bool
	Divider  bool // a separator line; other fields ignored
}

// MenuState is a Menu's or ContextMenu's interaction state.
type MenuState struct {
	kit.Popup
	Trigger kit.Clickable // draw the Menu's trigger with it

	items kit.Pool[kit.Clickable]
	last  []MenuItem
	ctx   int // tag of the context area
	group kit.FocusGroup
}

// Update returns the id of an item chosen since the last call, and toggles
// the menu when its trigger is clicked. Call it in the handle pass. A pick
// closes the menu and returns focus to the trigger.
func (m *MenuState) Update(gtx C) (string, bool) {
	if m.Trigger.Clicked(gtx) {
		m.Toggle()
	}
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
//	ui.Menu{ID: "file", Items: items, Trigger: func(gtx C) D {
//		return ui.Button{ID: "file:open", Text: "File", Style: ui.Ghost}.Layout(gtx, th, &v.file.Trigger)
//	}}.Layout(gtx, th, &v.file)
//
// Keyboard: Enter/Space on the trigger opens it; the open menu is one focus
// group: Down/Up enter and move through the items (Home/End jump); Enter
// picks; Escape closes; Tab closes and returns focus to the trigger.
type Menu struct {
	ID      string
	Items   []MenuItem
	Trigger kit.W
}

func (mn Menu) Layout(gtx C, th *kit.Theme, m *MenuState) D {
	m.last = mn.Items
	kit.SetRole(gtx, mn.ID, kit.RoleMenu)
	m.opening()
	if d, byKey := m.DidDismiss(); d {
		m.returnFocus(gtx, byKey)
	}
	trigger := func(gtx C) D {
		// The trigger keeps its natural size: a caller's minimum (a window's)
		// would stretch it and leave the menu no room.
		gtx.Constraints.Min = image.Point{}
		return mn.Trigger(gtx)
	}
	return th.Popover(gtx, &m.Popup, mn.ID, trigger, func(gtx C) D { return m.panel(gtx, th, mn.ID, mn.Items) })
}

// ContextMenu lays out Content; a right-click inside it opens the menu at
// the pointer. Read picks with MenuState.Update, as for a Menu. The area's
// id is ID/area.
type ContextMenu struct {
	ID      string
	Items   []MenuItem
	Content kit.W
}

func (cm ContextMenu) Layout(gtx C, th *kit.Theme, m *MenuState) D {
	m.last = cm.Items
	kit.SetRole(gtx, cm.ID, kit.RoleMenu)
	m.opening()
	m.DidDismiss()
	// Right presses anywhere inside the content bubble up to this area
	// (rule 1: read them before drawing).
	for {
		e, ok := gtx.Event(pointer.Filter{Target: &m.ctx, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pe, ok := e.(pointer.Event); ok && pe.Buttons.Contain(pointer.ButtonSecondary) {
			m.ShowAt(pe.Position.Round())
			kit.Notify(gtx)
		}
	}
	return th.Popover(gtx, &m.Popup, cm.ID, func(gtx C) D {
		mr := op.Record(gtx.Ops)
		dims := cm.Content(gtx)
		call := mr.Stop()
		defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
		event.Op(gtx.Ops, &m.ctx)
		kit.Tag(gtx, kit.Part(cm.ID, "area"))
		call.Add(gtx.Ops)
		return dims
	}, func(gtx C) D { return m.panel(gtx, th, cm.ID, cm.Items) })
}

// opening points the popup's focus at the item group and, on a fresh open,
// clears the active item until a key moves. Call it before Popover, which
// consumes Opening.
func (m *MenuState) opening() {
	m.FocusOnOpen(m.group.FocusHandle())
	if m.Opening() {
		m.group.Active = ""
	}
}

func (m *MenuState) returnFocus(gtx C, byKey bool) {
	if byKey {
		m.Trigger.Focus(gtx)
	} else {
		m.Trigger.FocusQuiet(gtx)
	}
}

// panel lays out the items and handles their keys.
func (m *MenuState) panel(gtx C, th *kit.Theme, id string, items []MenuItem) D {
	var nav []string
	for _, it := range items {
		if !it.Divider && !it.Disabled {
			nav = append(nav, it.ID)
			m.group.Join(m.items.Get(it.ID), it.ID == m.group.Active)
		}
	}
	return m.keys(gtx, id, nav, nil, func(gtx C) D { return m.rows(gtx, th, id, items) })
}

// keys makes the open panel one focus group over the items in nav (ids into
// m.items): arrows move the active item (entering from the first or last),
// Enter picks it, Tab closes the menu onto its trigger. l, if set, scrolls
// the active item into view.
func (m *MenuState) keys(gtx C, id string, nav []string, l *kit.List, w kit.W) D {
	cur := -1
	for i, n := range nav {
		if n == m.group.Active {
			cur = i
		}
	}
	return kit.KeyContext(gtx, "menu", func(gtx C) D {
		move := func(to int) func() {
			return func() {
				if len(nav) == 0 {
					return
				}
				to = max(0, min(len(nav)-1, to))
				m.group.Active = nav[to]
				if l != nil {
					l.ScrollIntoView(to)
				}
			}
		}
		down, up := cur+1, cur-1
		if cur < 0 {
			down, up = 0, len(nav)-1
		}
		kit.OnAction(gtx, kit.ActMoveDown, move(down))
		kit.OnAction(gtx, kit.ActMoveUp, move(up))
		kit.OnAction(gtx, kit.ActFirst, move(0))
		kit.OnAction(gtx, kit.ActLast, move(len(nav)-1))
		kit.OnAction(gtx, kit.ActConfirm, func() {
			if cur >= 0 {
				m.items.Get(nav[cur]).Click() // reported by Update next frame
			}
		})
		// Tab leaves the menu: close it and land on the trigger.
		for _, f := range []event.Tag{m.group.FocusHandle(), m.PanelFocus()} {
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

func (m *MenuState) rows(gtx C, th *kit.Theme, id string, items []MenuItem) D {
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
		children = append(children, kit.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			if it.Divider {
				return layout.Inset{Top: th.Space(kit.Space1), Bottom: th.Space(kit.Space1)}.Layout(gtx, kit.HLine(th.Divider))
			}
			return th.Disable(gtx, it.Disabled, func(gtx C) D {
				return th.MenuRow(gtx, m.items.Get(it.ID), kit.Part(id, it.ID), kit.MenuRow{Icon: it.Icon, Label: it.Label, Hint: it.Shortcut, Danger: it.Danger})
			})
		}))
	}
	return kit.Flex{Axis: layout.Vertical, Pad: kit.Space1}.Layout(gtx, th, children...)
}

// Select is a dropdown field showing Options[Selected]; it lists the
// options in a floating panel, the selected one checked and focused (so
// arrows start from it). Field id ID/field, options ID/<index>, the list
// ID/list (scrolls past Theme.ListMaxHeight).
type Select struct {
	ID       string
	Options  []string // shown in the field, spoken, and drawn unless Option is set
	Selected int
	// Option, if set, draws option i's row content (an avatar beside a name);
	// Options still name each option for screen readers.
	Option func(gtx C, i int) D
	Size   kit.ControlSize // ControlDefault is the theme's
}

// SelectState is a Select's interaction state.
type SelectState struct {
	MenuState
	list kit.List
}

// Update returns the index of an option picked since the last call, and
// toggles the dropdown when the field is clicked. Call it in the handle
// pass, then draw with the new selection.
func (s *SelectState) Update(gtx C) (int, bool) {
	s.MatchWidth = true
	if s.Trigger.Clicked(gtx) {
		s.Toggle()
	}
	for i := range s.last {
		c := s.items.Get(s.last[i].ID)
		s.group.Join(c, s.last[i].ID == s.group.Active)
		if c.Clicked(gtx) {
			s.Hide()
			s.Trigger.FocusQuiet(gtx)
			return i, true
		}
	}
	return 0, false
}

func (sl Select) Layout(gtx C, th *kit.Theme, s *SelectState) D {
	s.MatchWidth = true
	if len(s.last) != len(sl.Options) {
		s.last = make([]MenuItem, len(sl.Options))
	}
	for i, o := range sl.Options {
		s.last[i] = MenuItem{ID: strconv.Itoa(i), Label: o}
	}
	if d, byKey := s.DidDismiss(); d {
		s.returnFocus(gtx, byKey)
	}
	opening := s.Opening()
	s.opening()
	label := ""
	if sl.Selected >= 0 && sl.Selected < len(sl.Options) {
		label = sl.Options[sl.Selected]
	}
	return th.Popover(gtx, &s.Popup, sl.ID, func(gtx C) D {
		kit.SetRole(gtx, sl.ID, kit.RoleListbox)
		return th.Press(gtx, &s.Trigger, kit.Part(sl.ID, "field"), func(gtx C, st kit.State) D {
			semantic.Button.Add(gtx.Ops)
			kit.SetRole(gtx, kit.Part(sl.ID, "field"), kit.RoleCombobox)
			st.Focused = st.Focused || s.Opened()
			return th.SizedField(gtx, sl.Size, st, func(gtx C) D {
				return kit.Row(gtx,
					kit.Flexed(1, func(gtx C) D { return th.Text(gtx, label, th.Type.Body, th.Fg, kit.Regular) }),
					kit.Rigid(func(gtx C) D { return kit.IconChevronDown(gtx, th.IconSize, th.Muted) }),
				)
			})
		})
	}, func(gtx C) D {
		nav := make([]string, len(sl.Options))
		for i := range sl.Options {
			nav[i] = strconv.Itoa(i)
			s.group.Join(s.items.Get(nav[i]), nav[i] == s.group.Active)
		}
		return s.keys(gtx, sl.ID, nav, &s.list, func(gtx C) D {
			if opening && sl.Selected >= 0 && sl.Selected < len(sl.Options) {
				// Start on the selected option, quietly: no ring until a key moves.
				s.group.Active = strconv.Itoa(sl.Selected)
				s.group.FocusQuiet(gtx)
				s.list.ScrollIntoView(sl.Selected)
			}
			return sl.options(gtx, th, s)
		})
	})
}

// row is option i's menu row, with the custom renderer if there is one.
func (sl Select) row(i int, label string) kit.MenuRow {
	r := kit.MenuRow{Label: label, Checked: i == sl.Selected, Role: kit.RoleOption}
	if sl.Option != nil {
		r.Content = func(gtx C) D { return sl.Option(gtx, i) }
	}
	return r
}

func (sl Select) options(gtx C, th *kit.Theme, s *SelectState) D {
	pad := gtx.Dp(th.Space(kit.Space1))
	w := 0
	for i, o := range sl.Options {
		w = max(w, th.MenuRowWidth(gtx, sl.row(i, o)))
	}
	w += 2*pad + gtx.Dp(th.HitSize) // the panel's inset, room for the scrollbar
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, max(gtx.Constraints.Min.X, w))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.UniformInset(th.Space(kit.Space1)).Layout(gtx, func(gtx C) D {
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(th.ListMaxHeight))
		return th.List(gtx, &s.list, kit.Part(sl.ID, "list"), len(sl.Options), func(gtx C, i int) D {
			id := strconv.Itoa(i)
			return th.MenuRow(gtx, s.items.Get(id), kit.Part(sl.ID, id), sl.row(i, sl.Options[i]))
		})
	})
}

// CommandPalette is a modal, searchable action picker over Actions. Call
// state.Open from a shortcut, and wrap the window's Page with it each
// frame. Query field ID/query, results ID/<action ID>.
type CommandPalette struct {
	ID      string
	Actions *kit.Actions
	Page    kit.W
}

// CommandPaletteState is a CommandPalette's state.
type CommandPaletteState struct {
	Dialog    kit.Dialog
	Editor    widget.Editor
	items     kit.Pool[kit.Clickable]
	matches   []kit.Action
	selected  int
	focusNext bool
}

// maxResults caps the palette's result list.
const maxResults = 8

// Open shows the palette with an empty query.
func (p *CommandPaletteState) Open() {
	p.Editor.SetText("")
	p.selected = 0
	p.focusNext = true
	p.Dialog.Show()
}

// Close hides the palette.
func (p *CommandPaletteState) Close() { p.Dialog.Hide() }

func (cp CommandPalette) Layout(gtx C, th *kit.Theme, p *CommandPaletteState) D {
	if p.Dialog.Opened() {
		cp.handle(gtx, p)
	}
	return th.Modal(gtx, &p.Dialog, cp.ID, cp.Page, func(gtx C) D {
		if p.focusNext {
			p.focusNext = false
			gtx.Execute(key.FocusCmd{Tag: &p.Editor})
		}
		return kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return th.Text(gtx, th.Strings.Commands, th.Type.Title, th.Fg, kit.SemiBold) }),
			kit.Rigid(kit.Gap(0, th.Space(kit.Space3))),
			kit.Rigid(func(gtx C) D {
				p.Editor.SingleLine = true
				return th.EditorField(gtx, &p.Editor, kit.Part(cp.ID, "query"), kit.EditorField{Placeholder: th.Strings.CommandPlaceholder, Icon: kit.IconSearch})
			}),
			kit.Rigid(kit.Gap(0, th.Space(kit.Space2))),
			kit.Rigid(func(gtx C) D {
				var rows []layout.FlexChild
				for i, act := range p.matches {
					rows = append(rows, kit.Rigid(func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return th.MenuRow(gtx, p.items.Get(act.ID), kit.Part(cp.ID, act.ID), kit.MenuRow{Label: act.Title, Hint: act.Category, Highlighted: i == p.selected, Role: kit.RoleOption})
					}))
				}
				if len(rows) == 0 {
					rows = append(rows, kit.Rigid(func(gtx C) D { return th.Paragraph(gtx, th.Strings.NoCommands, th.Muted) }))
				}
				return kit.Col(gtx, rows...)
			}),
		)
	})
}

// handle runs clicks and the query field's keys (focus stays in the field,
// so Down/Up/Enter are read from it).
func (cp CommandPalette) handle(gtx C, p *CommandPaletteState) {
	for _, act := range p.matches {
		if p.items.Get(act.ID).Clicked(gtx) {
			cp.Actions.Run(act.ID)
			p.Close()
		}
	}
	for {
		e, ok := gtx.Event(key.Filter{Focus: &p.Editor, Name: key.NameDownArrow},
			key.Filter{Focus: &p.Editor, Name: key.NameUpArrow}, key.Filter{Focus: &p.Editor, Name: key.NameReturn})
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
			cp.Actions.Run(p.matches[p.selected].ID)
			p.Close()
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
	p.matches = cp.Actions.Search(p.Editor.Text())
	if len(p.matches) > maxResults {
		p.matches = p.matches[:maxResults]
	}
}
