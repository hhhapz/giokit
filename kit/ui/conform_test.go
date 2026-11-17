package ui_test

import (
	"testing"
	"time"

	"gioui.org/io/semantic"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest/conform"
)

// The conformance table for kit/ui: every interactive component has a row
// (see kittest/conform).
func TestConformance(t *testing.T) {
	button := func(name string, b ui.Button) conform.Case {
		return conform.Case{Name: name, ID: b.ID, Role: semantic.Button, Label: b.Text,
			Build: conform.Clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D { return b.Layout(gtx, th, c) })}
	}
	conform.Run(t,
		button("Button", ui.Button{ID: "save", Text: "Save"}),
		button("Button/Primary", ui.Button{ID: "save", Text: "Save", Style: ui.Primary}),
		button("Button/Ghost", ui.Button{ID: "save", Text: "Save", Style: ui.Ghost}),
		button("Button/Danger", ui.Button{ID: "save", Text: "Save", Style: ui.Danger}),
		button("Button/Solid", ui.Button{ID: "save", Text: "Save", Style: ui.Solid}),
		button("Button/Icon", ui.Button{ID: "next", Text: "Next", Icon: kit.IconChevronRight, Style: ui.Primary}),
		button("Button/Small", ui.Button{ID: "save", Text: "Save", Size: kit.ControlSmall}),
		conform.Case{Name: "IconButton", ID: "close", Role: semantic.Button, Label: "Close",
			Build: conform.Clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return ui.IconButton{ID: "close", Icon: kit.IconX, Label: "Close", Active: true}.Layout(gtx, th, c)
			})},
		conform.Case{Name: "Link", ID: "docs", Role: semantic.Button, Label: "Read the docs",
			Build: conform.Clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return ui.Link{ID: "docs", Text: "Read the docs"}.Layout(gtx, th, c)
			})},
		conform.Case{Name: "Checkbox", ID: "agree", Role: semantic.CheckBox, Label: "I agree",
			Build: conform.Clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return ui.Checkbox{ID: "agree", Label: "I agree"}.Layout(gtx, th, c)
			})},
		conform.Case{Name: "Switch", ID: "wifi", Role: semantic.Switch, Label: "Wi-Fi",
			Build: conform.Clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return ui.Switch{ID: "wifi", Label: "Wi-Fi"}.Layout(gtx, th, c)
			})},
		conform.Case{Name: "Radio", ID: "small", Role: semantic.RadioButton, Label: "Small",
			Build: conform.Clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return ui.Radio{ID: "small", Label: "Small"}.Layout(gtx, th, c)
			})},
		// A field's activation is gaining focus. No hover feedback beyond the
		// text cursor (drawn by the compositor), and Enter/Space type text.
		conform.Case{Name: "TextField", ID: "name", Role: semantic.Editor, Label: "Name", NoHover: true, NoKeys: true,
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				var e widget.Editor
				n, was := new(int), false
				return func(gtx kit.C) {
					th.Disable(gtx, disabled, func(gtx kit.C) kit.D {
						return ui.TextField{ID: "name", Placeholder: "Name"}.Layout(gtx, th, &e)
					})
					if f := gtx.Focused(&e); f && !was {
						*n++
						was = f
					} else {
						was = f
					}
				}, n
			}},
		conform.Case{Name: "NumberField/plus", ID: "qty/plus", Role: semantic.Button, Label: "Increase Quantity",
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				var s ui.NumberFieldState
				n := new(int)
				return func(gtx kit.C) {
					if _, ok := s.Update(gtx); ok {
						*n++
					}
					th.Disable(gtx, disabled, func(gtx kit.C) kit.D {
						return ui.NumberField{ID: "qty", Placeholder: "Quantity"}.Layout(gtx, th, &s)
					})
				}, n
			}},
		conform.Case{Name: "NavItem", ID: "nav:home", Role: semantic.Button, Label: "Home",
			Build: conform.Clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return ui.NavItem{ID: "nav:home", Label: "Home"}.Layout(gtx, th, c)
			})},
		conform.Case{Name: "Disclosure", ID: "more", Role: semantic.Button, Label: "More options",
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				var c ui.Collapsible
				n := new(int)
				return func(gtx kit.C) {
					if c.Update(gtx) {
						*n++
					}
					th.Disable(gtx, disabled, func(gtx kit.C) kit.D {
						return ui.Disclosure{ID: "more", Title: "More options", Open: c.Open}.Layout(gtx, th, &c.Clickable)
					})
				}, n
			}},
		// Tabs are one Tab stop that lands on the selected tab, which shows
		// no hover change: it is already the chosen one.
		conform.Case{Name: "Tabs", ID: "tabs/General", Role: semantic.Button, Label: "General", NoHover: true,
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				var s ui.TabsState
				n, sel := new(int), 0
				return func(gtx kit.C) {
					if i, ok := s.Update(gtx); ok {
						*n++
						sel = i
					}
					th.Disable(gtx, disabled, func(gtx kit.C) kit.D {
						return ui.Tabs{ID: "tabs", Labels: []string{"General", "Editor"}, Selected: sel}.Layout(gtx, th, &s)
					})
				}, n
			}},
		conform.Case{Name: "Stepper/plus", ID: "qty/plus", Role: semantic.Button, Label: "Increase Quantity",
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				var s ui.StepperState
				n := new(int)
				return func(gtx kit.C) {
					s.Minus.Clicked(gtx)
					if s.Plus.Clicked(gtx) {
						*n++
					}
					th.Disable(gtx, disabled, func(gtx kit.C) kit.D {
						return ui.Stepper{ID: "qty", Label: "Quantity", Value: "3", Active: true}.Layout(gtx, th, &s)
					})
				}, n
			}},
		conform.Case{Name: "TextArea", ID: "notes", Role: semantic.Editor, Label: "Notes", NoHover: true, NoKeys: true,
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				var e widget.Editor
				n, was := new(int), false
				return func(gtx kit.C) {
					th.Disable(gtx, disabled, func(gtx kit.C) kit.D {
						return ui.TextArea{ID: "notes", Placeholder: "Notes", Lines: 2}.Layout(gtx, th, &e)
					})
					f := gtx.Focused(&e)
					if f && !was {
						*n++
					}
					was = f
				}, n
			}},
		// A slider has no role class in Gio and no hover change (its knob is
		// already the affordance); Enter and Space don't move it.
		conform.Case{Name: "Slider", ID: "vol", Label: "Volume", NoHover: true, NoKeys: true,
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				var s ui.SliderState
				v, n := float32(0), new(int)
				return func(gtx kit.C) {
					if nv, ok := s.Update(gtx); ok {
						v = nv
						*n++
					}
					gtx.Constraints.Max.X = gtx.Dp(200)
					th.Disable(gtx, disabled, func(gtx kit.C) kit.D {
						return ui.Slider{ID: "vol", Label: "Volume", Value: v}.Layout(gtx, th, &s)
					})
				}, n
			}},
		conform.Case{Name: "Accordion", ID: "faq/a", Role: semantic.Button, Label: "Shipping",
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				var s ui.AccordionState
				open, n := "", new(int)
				return func(gtx kit.C) {
					if id, ok := s.Update(gtx); ok {
						open = id
						*n++
					}
					th.Disable(gtx, disabled, func(gtx kit.C) kit.D {
						return ui.Accordion{ID: "faq", Open: open, Sections: []ui.AccordionSection{{ID: "a", Title: "Shipping"}}}.Layout(gtx, th, &s)
					})
				}, n
			}},
		conform.Case{Name: "Breadcrumbs", ID: "path/home", Role: semantic.Button, Label: "Home",
			Build: conform.Clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return ui.Breadcrumbs{ID: "path", Path: []ui.Breadcrumb{{ID: "home", Label: "Home", Clickable: c}, {ID: "here", Label: "Here"}}}.Layout(gtx, th)
			})},
		conform.Case{Name: "Toolbar", ID: "tb/new", Role: semantic.Button, Label: "New",
			Build: conform.Clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return ui.Toolbar{ID: "tb", Actions: []ui.ToolbarAction{{ID: "new", Text: "New", Icon: kit.IconPlus, Clickable: c}}}.Layout(gtx, th)
			})},
	)
}

// Composite controls: open, navigate, pick, dismiss, focus returns.
func TestConformanceComposite(t *testing.T) {
	items := []ui.MenuItem{{ID: "new", Label: "New"}, {ID: "open", Label: "Open"}, {Divider: true}, {ID: "quit", Label: "Quit", Danger: true}}
	conform.RunComposite(t,
		conform.Composite{Name: "Menu", Trigger: "file:btn", Panel: "file", Item: "file/new",
			Build: func(th *kit.Theme) (func(kit.C), *int) {
				var m ui.MenuState
				n := new(int)
				return func(gtx kit.C) {
					if _, ok := m.Update(gtx); ok {
						*n++
					}
					ui.Menu{ID: "file", Items: items, Trigger: func(gtx kit.C) kit.D {
						return ui.Button{ID: "file:btn", Text: "File"}.Layout(gtx, th, &m.Trigger)
					}}.Layout(gtx, th, &m)
				}, n
			}},
		conform.Composite{Name: "DateField", Trigger: "due/field", Panel: "due", Item: "due/2026-11-27",
			Build: func(th *kit.Theme) (func(kit.C), *int) {
				var s ui.DateFieldState
				v, n := time.Date(2026, 11, 20, 0, 0, 0, 0, time.Local), new(int)
				return func(gtx kit.C) {
					if d, ok := s.Update(gtx); ok {
						v = d
						*n++
					}
					gtx.Constraints.Max.X = gtx.Dp(220)
					ui.DateField{ID: "due", Value: v}.Layout(gtx, th, &s)
				}, n
			}},
		conform.Composite{Name: "Select", Trigger: "size/field", Panel: "size", Item: "size/2",
			Build: func(th *kit.Theme) (func(kit.C), *int) {
				var s ui.SelectState
				sel, n := 1, new(int)
				return func(gtx kit.C) {
					if i, ok := s.Update(gtx); ok {
						sel = i
						*n++
					}
					gtx.Constraints.Max.X = gtx.Dp(200)
					ui.Select{ID: "size", Options: []string{"Small", "Medium", "Large"}, Selected: sel}.Layout(gtx, th, &s)
				}, n
			}},
	)
}

// Focus-group collections: one Tab stop on the selection, arrows select.
func TestConformanceGroup(t *testing.T) {
	sel := func(build func(th *kit.Theme, selected string) kit.W, update func(gtx kit.C) (string, bool)) func(*kit.Theme, bool) (func(kit.C), *int) {
		return func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
			n, selected := new(int), "a"
			return func(gtx kit.C) {
				if id, ok := update(gtx); ok {
					selected = id
					*n++
				}
				th.Disable(gtx, disabled, build(th, selected))
			}, n
		}
	}
	items := []ui.Choice{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}, {ID: "c", Label: "Gamma"}}
	var list ui.SelectableListState
	var tree ui.TreeState
	var table ui.DataTableState
	conform.RunGroup(t,
		conform.Group{Name: "SelectableList", First: "files/a", Second: "files/b",
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				list = ui.SelectableListState{}
				return sel(func(th *kit.Theme, s string) kit.W {
					return func(gtx kit.C) kit.D {
						return ui.SelectableList{ID: "files", Items: items, Selected: s}.Layout(gtx, th, &list)
					}
				}, list.Update)(th, disabled)
			}},
		conform.Group{Name: "Tree", First: "tree/a", Second: "tree/b",
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				tree = ui.TreeState{}
				roots := []ui.TreeNode{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}}
				return sel(func(th *kit.Theme, s string) kit.W {
					return func(gtx kit.C) kit.D {
						return ui.Tree{ID: "tree", Roots: roots, Selected: s}.Layout(gtx, th, &tree)
					}
				}, tree.Update)(th, disabled)
			}},
		conform.Group{Name: "DataTable", First: "t/a", Second: "t/b",
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				table = ui.DataTableState{}
				rows := []ui.DataRow{{ID: "a", Cells: []string{"Alpha", "1"}}, {ID: "b", Cells: []string{"Beta", "2"}}}
				return sel(func(th *kit.Theme, s string) kit.W {
					return func(gtx kit.C) kit.D {
						return ui.DataTable{ID: "t", Columns: []kit.Column{{Title: "Name"}, {Title: "N"}}, Rows: rows, Selected: s}.Layout(gtx, th, &table)
					}
				}, table.Update)(th, disabled)
			}},
	)
}
