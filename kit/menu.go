package kit

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
	"gioui.org/unit"
)

// MenuItem is one row of a Menu or ContextArea. The zero value of each
// field is the common case.
type MenuItem struct {
	ID       string // reported by Clicked; item ids are <menu id>/<ID>
	Label    string
	Icon     Icon   // optional, leading
	Shortcut string // optional hint on the right ("Ctrl+S"); bind it in your Keymap
	Danger   bool   // destructive: drawn in Bad
	Disabled bool
	Divider  bool // a separator line; other fields ignored
}

// Menu is the state of a dropdown menu (th.Menu) or a context menu
// (th.ContextArea).
//
//	if id, ok := v.file.Clicked(gtx); ok { ... }   // handle pass
//	th.Menu(gtx, &v.file, "file", func(gtx C) D {   // layout
//		return th.Button(gtx, &v.file.Trigger, "file:open", "File", kit.Ghost)
//	}, items)
//
// Keyboard: Enter/Space on the trigger opens it; the open menu is one focus
// group: Down/Up enter and move through the items (Home/End jump); Enter
// picks; Escape closes; Tab closes and returns focus to the trigger.
type Menu struct {
	Popup
	Trigger Clickable // draw your trigger with it (th.Menu only)

	items Pool[Clickable]
	last  []MenuItem
	ctx   int // tag of the context area
	group FocusGroup
}

// Clicked returns the id of an item chosen since the last call, and toggles
// the menu when its trigger is clicked. Call it in your handle pass. A pick
// closes the menu and returns focus to the trigger.
func (m *Menu) Clicked(gtx C) (string, bool) {
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

// Menu lays out trigger (draw it with &m.Trigger) and, while open, the
// items in a floating panel below it.
func (t *Theme) Menu(gtx C, m *Menu, id string, trigger W, items []MenuItem) D {
	m.last = items
	m.opening()
	if d, byKey := m.DidDismiss(); d {
		m.returnFocus(gtx, byKey)
	}
	return t.Popover(gtx, &m.Popup, id, trigger, func(gtx C) D { return t.menuItems(gtx, m, id, items) })
}

// opening resets the active item when the menu has just been opened: a
// fresh menu has none until a key moves. Call it before Popover, which
// clears focusNext.
func (m *Menu) opening() {
	m.focusTo = &m.group.focus // the open menu focuses its group, not the panel
	if m.focusNext {
		m.group.Active = ""
	}
}

func (m *Menu) returnFocus(gtx C, byKey bool) {
	if byKey {
		m.Trigger.Focus(gtx)
	} else {
		m.Trigger.FocusQuiet(gtx)
	}
}

// ContextArea lays out w; a right-click inside it opens the menu at the
// pointer. Read picks with m.Clicked, as for th.Menu. The area's id is
// <id>/area.
func (t *Theme) ContextArea(gtx C, m *Menu, id string, w W, items []MenuItem) D {
	m.last = items
	m.opening()
	m.DidDismiss()
	// Right presses anywhere inside w bubble up to this area (rule 1: read
	// them before drawing).
	for {
		e, ok := gtx.Event(pointer.Filter{Target: &m.ctx, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pe, ok := e.(pointer.Event); ok && pe.Buttons.Contain(pointer.ButtonSecondary) {
			m.ShowAt(pe.Position.Round())
			Notify(gtx)
		}
	}
	return t.Popover(gtx, &m.Popup, id, func(gtx C) D {
		mr := op.Record(gtx.Ops)
		dims := w(gtx)
		call := mr.Stop()
		defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
		event.Op(gtx.Ops, &m.ctx)
		Tag(gtx, Part(id, "area"))
		call.Add(gtx.Ops)
		return dims
	}, func(gtx C) D { return t.menuItems(gtx, m, id, items) })
}

// menuItems lays out the panel's rows and handles its keys.
func (t *Theme) menuItems(gtx C, m *Menu, id string, items []MenuItem) D {
	var nav []string
	for _, it := range items {
		if !it.Divider && !it.Disabled {
			nav = append(nav, it.ID)
			m.items.Get(it.ID).group = &m.group
		}
	}
	return m.keys(gtx, id, nav, nil, func(gtx C) D { return t.menuRows(gtx, m, id, items) })
}

// keys makes the open panel one focus group over the items in nav (ids into
// m.items): arrows move the active item (entering from the first or last),
// Enter picks it, Tab closes the menu onto its trigger. l, if set, scrolls
// the active item into view.
func (m *Menu) keys(gtx C, id string, nav []string, l *List, w W) D {
	cur := -1
	for i, n := range nav {
		if n == m.group.Active {
			cur = i
		}
	}
	return KeyContext(gtx, "menu", func(gtx C) D {
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
		OnAction(gtx, ActMoveDown, move(down))
		OnAction(gtx, ActMoveUp, move(up))
		OnAction(gtx, ActFirst, move(0))
		OnAction(gtx, ActLast, move(len(nav)-1))
		OnAction(gtx, ActConfirm, func() {
			if cur >= 0 {
				m.items.Get(nav[cur]).Click() // reported by Clicked next frame
			}
		})
		// Tab leaves the menu: close it and land on the trigger.
		for _, f := range []event.Tag{&m.group.focus, &m.focus} {
			for {
				e, ok := gtx.Event(key.Filter{Focus: f, Name: key.NameTab, Optional: key.ModShift})
				if !ok {
					break
				}
				if ke, ok := e.(key.Event); ok && ke.State == key.Press {
					m.Hide()
					m.returnFocus(gtx, true)
					Notify(gtx)
				}
			}
		}
		return m.group.Layout(gtx, id, w)
	})
}

func (t *Theme) menuRows(gtx C, m *Menu, id string, items []MenuItem) D {
	// As wide as the widest row (at least 180dp), not the whole cap.
	w := gtx.Dp(180)
	for _, it := range items {
		if !it.Divider {
			w = max(w, t.menuRowWidth(gtx, it.Icon != nil, it.Label, it.Shortcut, false))
		}
	}
	w += gtx.Dp(8) // the panel's inset
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, max(gtx.Constraints.Min.X, w))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	children := make([]layout.FlexChild, 0, len(items))
	for _, it := range items {
		it := it
		children = append(children, Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			if it.Divider {
				return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, HLine(t.Divider))
			}
			return t.Disable(gtx, it.Disabled, func(gtx C) D {
				return t.menuRow(gtx, m.items.Get(it.ID), Part(id, it.ID), it.Icon, it.Label, it.Shortcut, it.Danger, false)
			})
		}))
	}
	return layout.Inset{Top: 4, Bottom: 4, Left: 4, Right: 4}.Layout(gtx, func(gtx C) D {
		return Col(gtx, children...)
	})
}

// menuRow is one highlighted-on-hover-or-focus row: icon, label, hint, and
// a check mark for selected Select options.
func (t *Theme) menuRow(gtx C, c *Clickable, id string, icon Icon, label, hint string, danger, checked bool) D {
	return t.menuRowHi(gtx, c, id, icon, label, hint, danger, checked, false)
}

// menuRowHi is menuRow, drawn highlighted when hi (a combobox's keyboard
// choice, while focus stays in its field).
func (t *Theme) menuRowHi(gtx C, c *Clickable, id string, icon Icon, label, hint string, danger, checked, hi bool) D {
	return t.press(gtx, c, id, func(gtx C, st State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		fg := t.Fg
		if danger {
			fg = t.Bad
		}
		lit := st.Hover
		if st.Focused || st.Pressed || hi {
			lit = 1
		}
		s := t.Styled(Surface{Bg: WithAlpha(t.tint(t.Overlay, PressTint), uint8(255*lit)), Radius: t.RadiusSm()})
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return s.Layout(gtx, func(gtx C) D {
			return layout.Inset{Top: t.control(5), Bottom: t.control(6), Left: t.Space(8), Right: t.Space(8)}.Layout(gtx, func(gtx C) D {
				return Row(gtx,
					Rigid(func(gtx C) D {
						if icon == nil {
							return D{}
						}
						return layout.Inset{Right: 8}.Layout(gtx, func(gtx C) D { return icon(gtx, 14, fg) })
					}),
					Flexed(1, func(gtx C) D { return t.Text(gtx, label, t.Type.Body, fg, Regular) }),
					Rigid(func(gtx C) D {
						if hint == "" {
							return D{}
						}
						return layout.Inset{Left: 16}.Layout(gtx, func(gtx C) D { return t.Text(gtx, hint, t.Type.Small, t.Dim, Regular) })
					}),
					Rigid(func(gtx C) D {
						if !checked {
							return D{}
						}
						return layout.Inset{Left: 12}.Layout(gtx, func(gtx C) D { return IconCheck(gtx, 14, t.Accent) })
					}),
				)
			})
		})
	})
}

// Select is the state of a single-choice dropdown (th.Select).
type Select struct {
	Menu
	list    List
	changed int
	picked  bool
}

// Changed returns the index of an option picked since the last call, and
// toggles the dropdown when the field is clicked. Call it in your handle
// pass, then draw with the new selection.
func (s *Select) Changed(gtx C) (int, bool) {
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

// Select is a dropdown field showing options[selected]; it lists the
// options in a floating panel, the selected one checked and focused (so
// arrows start from it). Option ids are <id>/<index>.
func (t *Theme) Select(gtx C, s *Select, id string, options []string, selected int) D {
	s.MatchWidth = true
	if len(s.last) != len(options) {
		s.last = make([]MenuItem, len(options))
	}
	for i, o := range options {
		s.last[i] = MenuItem{ID: itoa(i), Label: o}
	}
	if d, byKey := s.DidDismiss(); d {
		s.returnFocus(gtx, byKey)
	}
	opening := s.focusNext
	s.opening()
	label := ""
	if selected >= 0 && selected < len(options) {
		label = options[selected]
	}
	return t.Popover(gtx, &s.Popup, id, func(gtx C) D {
		return t.selectField(gtx, &s.Trigger, Part(id, "field"), label, s.open)
	}, func(gtx C) D {
		nav := make([]string, len(options))
		for i := range options {
			nav[i] = itoa(i)
			s.items.Get(nav[i]).group = &s.group
		}
		return s.keys(gtx, id, nav, &s.list, func(gtx C) D {
			if opening && selected >= 0 && selected < len(options) {
				// Start on the selected option, quietly: no ring until a key moves.
				s.group.Active = itoa(selected)
				s.group.focus.quiet = true
				s.list.ScrollIntoView(selected)
			}
			return t.selectOptions(gtx, s, id, options, selected)
		})
	})
}

func (t *Theme) selectOptions(gtx C, s *Select, id string, options []string, selected int) D {
	{
		w := 0
		for i, o := range options {
			w = max(w, t.menuRowWidth(gtx, false, o, "", i == selected))
		}
		w += gtx.Dp(8) + gtx.Dp(16) // the panel's inset, room for the scrollbar
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, max(gtx.Constraints.Min.X, w))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Top: 4, Bottom: 4, Left: 4, Right: 4}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(300))
			return t.List(gtx, &s.list, Part(id, "list"), len(options), func(gtx C, i int) D {
				return t.menuRow(gtx, s.items.Get(itoa(i)), Part(id, itoa(i)), nil, options[i], "", false, i == selected)
			})
		})
	}
}

// menuRowWidth is a menu row's natural width, matching menuRow's layout.
func (t *Theme) menuRowWidth(gtx C, icon bool, label, hint string, checked bool) int {
	text := func(s string, size unit.Sp) int {
		if s == "" {
			return 0
		}
		m := op.Record(gtx.Ops) // measured, then dropped
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
		d := t.Text(g, s, size, t.Fg, Regular)
		m.Stop()
		return d.Size.X
	}
	w := gtx.Dp(t.Space(8))*2 + text(label, t.Type.Body)
	if icon {
		w += gtx.Dp(14 + 8)
	}
	if hint != "" {
		w += gtx.Dp(16) + text(hint, t.Type.Small)
	}
	if checked {
		w += gtx.Dp(12 + 14)
	}
	return w
}

// selectField is the closed dropdown: a field-like button with a chevron.
func (t *Theme) selectField(gtx C, c *Clickable, id, label string, open bool) D {
	return t.press(gtx, c, id, func(gtx C, st State) D {
		semantic.Button.Add(gtx.Ops)
		s := t.Styled(Surface{Bg: t.tint(t.Surface, PressTint/2*st.Hover), Border: t.Border, Radius: t.Radius})
		if st.Focused || open {
			s.Focus = t.Focus
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return s.Layout(gtx, func(gtx C) D {
			return layout.Inset{Top: t.control(7), Bottom: t.control(7), Left: t.Space(10), Right: t.Space(8)}.Layout(gtx, func(gtx C) D {
				return Row(gtx,
					Flexed(1, func(gtx C) D { return t.Text(gtx, label, t.Type.Body, t.Fg, Regular) }),
					Rigid(func(gtx C) D { return IconChevronDown(gtx, 14, t.Muted) }),
				)
			})
		})
	})
}

func itoa(i int) string { return strconv.Itoa(i) }
