package ui_test

import (
	"testing"
	"time"

	"github.com/hhhapz/giokit/gio/io/semantic"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest/conform"
)

// The conformance table for kit/ui: every interactive component has a row
// (see kittest/conform).
func TestConformance(t *testing.T) {
	button := func(name string, b ui.Button) conform.Case {
		return conform.Case{Name: name, ID: b.ID, Class: semantic.Button, Role: kit.RoleButton, Label: b.Text,
			Build: conform.Clickable(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable) layout.Dimensions {
				return b.Layout(gtx, th, c)
			})}
	}
	conform.Run(t,
		button("Button", ui.Button{ID: "save", Text: "Save"}),
		button("Button/Primary", ui.Button{ID: "save", Text: "Save", Style: ui.Primary}),
		button("Button/Ghost", ui.Button{ID: "save", Text: "Save", Style: ui.Ghost}),
		button("Button/Danger", ui.Button{ID: "save", Text: "Save", Style: ui.Danger}),
		button("Button/Solid", ui.Button{ID: "save", Text: "Save", Style: ui.Solid}),
		button("Button/Icon", ui.Button{ID: "next", Text: "Next", Icon: kit.IconChevronRight, Style: ui.Primary}),
		button("Button/Small", ui.Button{ID: "save", Text: "Save", Size: kit.ControlSmall}),
		conform.Case{Name: "IconButton", ID: "close", Class: semantic.Button, Role: kit.RoleButton, Label: "Close",
			Build: conform.Clickable(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable) layout.Dimensions {
				return ui.IconButton{ID: "close", Icon: kit.IconX, Label: "Close", Active: true}.Layout(gtx, th, c)
			})},
		conform.Case{Name: "Link", ID: "docs", Class: semantic.Button, Role: kit.RoleLink, Label: "Read the docs",
			Build: conform.Clickable(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable) layout.Dimensions {
				return ui.Link{ID: "docs", Text: "Read the docs"}.Layout(gtx, th, c)
			})},
		conform.Case{Name: "Checkbox", ID: "agree", Class: semantic.CheckBox, Role: kit.RoleCheckbox, Label: "I agree",
			Build: conform.Clickable(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable) layout.Dimensions {
				return ui.Checkbox{ID: "agree", Label: "I agree"}.Layout(gtx, th, c)
			})},
		conform.Case{Name: "Switch", ID: "wifi", Class: semantic.Switch, Role: kit.RoleSwitch, Label: "Wi-Fi",
			Build: conform.Clickable(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable) layout.Dimensions {
				return ui.Switch{ID: "wifi", Label: "Wi-Fi"}.Layout(gtx, th, c)
			})},
		conform.Case{Name: "Radio", ID: "small", Class: semantic.RadioButton, Role: kit.RoleRadio, Label: "Small",
			Build: conform.Clickable(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable) layout.Dimensions {
				return ui.Radio{ID: "small", Label: "Small"}.Layout(gtx, th, c)
			})},
		// A field's activation is gaining focus. No hover feedback beyond the
		// text cursor (drawn by the compositor), and Enter/Space type text.
		conform.Case{Name: "TextField", ID: "name", Class: semantic.Editor, Role: kit.RoleTextbox, Label: "Name", NoHover: true, NoKeys: true,
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				var e widget.Editor
				n, was := new(int), false
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions {
						return ui.TextField{ID: "name", Placeholder: "Name"}.Layout(gtx, th, &e)
					})
					f := gtx.Focused(&e)
					if f && !was {
						*n++
					}
					was = f
				}}
			}},
		conform.Case{Name: "NumberField/plus", ID: "qty/plus", Class: semantic.Button, Role: kit.RoleButton, Label: "Increase Quantity",
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				var s ui.NumberFieldState
				n := new(int)
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					if _, ok := s.Update(gtx); ok {
						*n++
					}
					th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions {
						return ui.NumberField{ID: "qty", Placeholder: "Quantity"}.Layout(gtx, th, &s)
					})
				}}
			}},
		conform.Case{Name: "NavItem", ID: "nav:home", Class: semantic.Button, Role: kit.RoleLink, Label: "Home",
			Build: conform.Clickable(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable) layout.Dimensions {
				return ui.NavItem{ID: "nav:home", Label: "Home"}.Layout(gtx, th, c)
			})},
		conform.Case{Name: "Disclosure", ID: "more", Class: semantic.Button, Role: kit.RoleButton, Label: "More options",
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				var c ui.Collapsible
				n := new(int)
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					if c.Update(gtx) {
						*n++
					}
					th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions {
						return ui.Disclosure{ID: "more", Title: "More options", Open: c.Open}.Layout(gtx, th, &c.Clickable)
					})
				}}
			}},
		// Tabs are one Tab stop that lands on the selected tab, which shows
		// no hover change: it is already the chosen one.
		conform.Case{Name: "Tabs", ID: "tabs/general", Class: semantic.Button, Role: kit.RoleTab, Label: "General", NoHover: true,
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				var s ui.TabsState
				n, sel := new(int), "general"
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					if i, ok := s.Update(gtx); ok {
						*n++
						sel = i
					}
					th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions {
						return ui.Tabs{ID: "tabs", Items: []ui.Choice{{ID: "general", Label: "General"}, {ID: "editor", Label: "Editor"}}, Selected: sel}.Layout(gtx, th, &s)
					})
				}}
			}},
		conform.Case{Name: "Stepper/plus", ID: "qty/plus", Class: semantic.Button, Role: kit.RoleButton, Label: "Increase Quantity",
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				var s ui.StepperState
				n := new(int)
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					s.Minus.Clicked(gtx)
					if s.Plus.Clicked(gtx) {
						*n++
					}
					th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions {
						return ui.Stepper{ID: "qty", Label: "Quantity", Value: "3", Active: true}.Layout(gtx, th, &s)
					})
				}}
			}},
		conform.Case{Name: "TextArea", ID: "notes", Class: semantic.Editor, Role: kit.RoleTextbox, Label: "Notes", NoHover: true, NoKeys: true,
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				var e widget.Editor
				n, was := new(int), false
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions {
						return ui.TextArea{ID: "notes", Placeholder: "Notes", Lines: 2}.Layout(gtx, th, &e)
					})
					f := gtx.Focused(&e)
					if f && !was {
						*n++
					}
					was = f
				}}
			}},
		// A slider has no role class in Gio and no hover change (its knob is
		// already the affordance); Enter and Space don't move it.
		conform.Case{Name: "Slider", ID: "vol", Role: kit.RoleSlider, Label: "Volume", NoHover: true, NoKeys: true,
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				var s ui.SliderState
				v, n := float32(0), new(int)
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					if nv, ok := s.Update(gtx); ok {
						v = nv
						*n++
					}
					gtx.Constraints.Max.X = gtx.Dp(200)
					th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions {
						return ui.Slider{ID: "vol", Label: "Volume", Value: v}.Layout(gtx, th, &s)
					})
				}}
			}},
		conform.Case{Name: "Accordion", ID: "faq/a", Class: semantic.Button, Role: kit.RoleButton, Label: "Shipping",
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				var s ui.AccordionState
				open, n := "", new(int)
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					if id, ok := s.Update(gtx); ok {
						open = id
						*n++
					}
					th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions {
						return ui.Accordion{ID: "faq", Open: open, Sections: []ui.AccordionSection{{ID: "a", Title: "Shipping"}}}.Layout(gtx, th, &s)
					})
				}}
			}},
		// The reorder buttons; the drag handle (no click activation) is
		// covered by TestReorderHandle.
		conform.Case{Name: "ReorderList/up", ID: "q/b/up", Class: semantic.Button, Role: kit.RoleButton, Label: "Move Second up",
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				var s ui.ReorderListState
				items := []ui.Choice{{ID: "a", Label: "First"}, {ID: "b", Label: "Second"}}
				n := new(int)
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					if _, _, ok := s.Update(gtx); ok {
						*n++ // counted, not applied: the button stays where it is
					}
					th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions {
						return ui.ReorderList{ID: "q", Items: items}.Layout(gtx, th, &s)
					})
				}}
			}},
		conform.Case{Name: "Breadcrumbs", ID: "path/home", Class: semantic.Button, Role: kit.RoleLink, Label: "Home",
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				var s ui.BreadcrumbsState
				n := new(int)
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					if id, ok := s.Update(gtx); ok && id == "home" {
						*n++
					}
					th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions {
						return ui.Breadcrumbs{ID: "path", Path: []ui.Breadcrumb{{ID: "home", Label: "Home"}, {ID: "here", Label: "Here"}}}.Layout(gtx, th, &s)
					})
				}}
			}},
		conform.Case{Name: "Toolbar", ID: "tb/new", Class: semantic.Button, Role: kit.RoleButton, Label: "New",
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				var s ui.ToolbarState
				n := new(int)
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					if id, ok := s.Update(gtx); ok && id == "new" {
						*n++
					}
					th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions {
						return ui.Toolbar{ID: "tb", Actions: []ui.ToolbarAction{{ID: "new", Text: "New", Icon: kit.IconPlus}}}.Layout(gtx, th, &s)
					})
				}}
			}},
	)
}

// Composite controls: open, navigate, pick, dismiss, focus returns.
func TestConformanceComposite(t *testing.T) {
	items := []ui.MenuItem{{ID: "new", Label: "New"}, {ID: "open", Label: "Open"}, {Divider: true}, {ID: "quit", Label: "Quit", Danger: true}}
	conform.RunComposite(t,
		conform.Composite{Name: "Menu", Trigger: "file:btn", Panel: "file", Item: "file/new",
			Build: func(th *kit.Theme) conform.Control {
				var m ui.MenuState
				n := new(int)
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					if _, ok := m.Update(gtx); ok {
						*n++
					}
					ui.Menu{ID: "file", Items: items, Trigger: func(gtx layout.Context) layout.Dimensions {
						return ui.Button{ID: "file:btn", Text: "File"}.Layout(gtx, th, &m.Trigger)
					}}.Layout(gtx, th, &m)
				}}
			}},
		conform.Composite{Name: "DateField", Trigger: "due/field", Panel: "due", Item: "due/2026-11-27",
			Build: func(th *kit.Theme) conform.Control {
				var s ui.DateFieldState
				v, n := time.Date(2026, 11, 20, 0, 0, 0, 0, time.Local), new(int)
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					if d, ok := s.Update(gtx); ok {
						v = d
						*n++
					}
					gtx.Constraints.Max.X = gtx.Dp(220)
					ui.DateField{ID: "due", Value: v}.Layout(gtx, th, &s)
				}}
			}},
		conform.Composite{Name: "Select", Trigger: "size/field", Panel: "size", Item: "size/l",
			Build: func(th *kit.Theme) conform.Control {
				var s ui.SelectState
				sel, n := "m", new(int)
				return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
					if i, ok := s.Update(gtx); ok {
						sel = i
						*n++
					}
					gtx.Constraints.Max.X = gtx.Dp(200)
					ui.Select{ID: "size", Options: []ui.Choice{{ID: "s", Label: "Small"}, {ID: "m", Label: "Medium"}, {ID: "l", Label: "Large"}}, Selected: sel}.Layout(gtx, th, &s)
				}}
			}},
	)
}

// Focus-group collections: one Tab stop on the selection, arrows select.
func TestConformanceGroup(t *testing.T) {
	sel := func(build func(th *kit.Theme, selected string) layout.Widget, update func(gtx layout.Context) (ui.SelectionEvent, bool)) conform.Builder {
		return func(th *kit.Theme, disabled bool) conform.Control {
			n, selected := new(int), "a"
			return conform.Control{Activations: n, Frame: func(gtx layout.Context) {
				if e, ok := update(gtx); ok {
					selected = e.Selected
					*n++
				}
				th.Disable(gtx, disabled, build(th, selected))
			}}
		}
	}
	items := []ui.Choice{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}, {ID: "c", Label: "Gamma"}}
	conform.RunGroup(t,
		conform.Group{Name: "SelectableList", First: "files/a", Second: "files/b",
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				list := new(ui.SelectableListState)
				return sel(func(th *kit.Theme, s string) layout.Widget {
					return func(gtx layout.Context) layout.Dimensions {
						return ui.SelectableList{ID: "files", Items: items, Selected: s}.Layout(gtx, th, list)
					}
				}, list.Update)(th, disabled)
			}},
		conform.Group{Name: "Tree", First: "tree/a", Second: "tree/b",
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				tree := new(ui.TreeState)
				roots := []ui.TreeNode{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}}
				return sel(func(th *kit.Theme, s string) layout.Widget {
					return func(gtx layout.Context) layout.Dimensions {
						return ui.Tree{ID: "tree", Roots: roots, Selected: s}.Layout(gtx, th, tree)
					}
				}, tree.Update)(th, disabled)
			}},
		conform.Group{Name: "DataTable", First: "t/a", Second: "t/b",
			Build: func(th *kit.Theme, disabled bool) conform.Control {
				table := new(ui.DataTableState)
				rows := []ui.DataRow{{ID: "a", Cells: []string{"Alpha", "1"}}, {ID: "b", Cells: []string{"Beta", "2"}}}
				return sel(func(th *kit.Theme, s string) layout.Widget {
					return func(gtx layout.Context) layout.Dimensions {
						return ui.DataTable{ID: "t", Columns: []ui.DataColumn{{ID: "name", Title: "Name"}, {ID: "n", Title: "N"}}, Rows: rows, Selected: s}.Layout(gtx, th, table)
					}
				}, table.Update)(th, disabled)
			}},
	)
}
