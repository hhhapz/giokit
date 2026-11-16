package kit_test

import (
	"testing"

	"gioui.org/io/semantic"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kittest/conform"
)

// clickable builds a case around one Clickable: draw lays the control out,
// and every click it reports counts as an activation.
func clickable(draw func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D) func(*kit.Theme, bool) (func(kit.C), *int) {
	return func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
		var c kit.Clickable
		n := new(int)
		return func(gtx kit.C) {
			if c.Clicked(gtx) {
				*n++
			}
			th.Disable(gtx, disabled, func(gtx kit.C) kit.D { return draw(th, gtx, &c) })
		}, n
	}
}

// The conformance table: every interactive control has a row. A new
// control lands with its row green (see kittest/conform).
func TestConformance(t *testing.T) {
	conform.Run(t,
		conform.Case{Name: "Button", ID: "save", Role: semantic.Button, Label: "Save",
			Build: clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return th.Button(gtx, c, "save", "Save", kit.Secondary)
			})},
		conform.Case{Name: "Button/Solid", ID: "save", Role: semantic.Button, Label: "Save",
			Build: clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return th.Button(gtx, c, "save", "Save", kit.Solid)
			})},
		conform.Case{Name: "IconLabelButton", ID: "next", Role: semantic.Button, Label: "Next",
			Build: clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return th.IconLabelButton(gtx, c, "next", kit.IconChevronRight, "Next", kit.Primary)
			})},
		conform.Case{Name: "IconButtonLabel", ID: "close", Role: semantic.Button, Label: "Close",
			Build: clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return th.IconButtonLabel(gtx, c, "close", "Close", kit.IconX, true)
			})},
		conform.Case{Name: "CheckboxLabel", ID: "agree", Role: semantic.CheckBox, Label: "I agree",
			Build: clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return th.CheckboxLabel(gtx, c, "agree", "I agree", false)
			})},
		conform.Case{Name: "Switch", ID: "wifi", Role: semantic.Switch, Label: "Wi-Fi",
			Build: clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return th.Switch(gtx, c, "wifi", "Wi-Fi", false)
			})},
		conform.Case{Name: "Radio", ID: "small", Role: semantic.RadioButton, Label: "Small",
			Build: clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return th.Radio(gtx, c, "small", "Small", false)
			})},
		conform.Case{Name: "Link", ID: "docs", Role: semantic.Button, Label: "Read the docs",
			Build: clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return th.Link(gtx, c, "docs", "Read the docs")
			})},
		conform.Case{Name: "NavItem", ID: "nav:home", Role: semantic.Button, Label: "Home",
			Build: clickable(func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D {
				return th.NavItem(gtx, c, "nav:home", "Home", "", th.Dim, false)
			})},
		conform.Case{Name: "Disclosure", ID: "more", Role: semantic.Button, Label: "More options",
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				var c kit.Collapsible
				n := new(int)
				return func(gtx kit.C) {
					was := c.Open
					th.Disable(gtx, disabled, func(gtx kit.C) kit.D {
						return th.Disclosure(gtx, &c, "more", "More options", "", func(gtx kit.C) kit.D { return kit.D{} })
					})
					if c.Open != was {
						*n++
					}
				}, n
			}},
		conform.Case{Name: "Tabs", ID: "tabs/Editor", Role: semantic.Button, Label: "Editor",
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				var s kit.Tabs
				n, sel := new(int), 0
				return func(gtx kit.C) {
					if i, ok := s.Clicked(gtx); ok {
						*n++
						sel = i
					}
					th.Disable(gtx, disabled, func(gtx kit.C) kit.D {
						return th.Tabs(gtx, &s, "tabs", []string{"General", "Editor"}, sel)
					})
				}, n
			}},
		conform.Case{Name: "Stepper/plus", ID: "qty/plus", Role: semantic.Button,
			Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
				var minus, plus kit.Clickable
				n := new(int)
				return func(gtx kit.C) {
					minus.Clicked(gtx)
					if plus.Clicked(gtx) {
						*n++
					}
					th.Disable(gtx, disabled, func(gtx kit.C) kit.D {
						return th.Stepper(gtx, &minus, &plus, "qty", "3", th.Fg, true)
					})
				}, n
			}},
	)
}
