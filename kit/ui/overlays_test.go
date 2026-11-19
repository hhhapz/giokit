package ui_test

import (
	"image"
	"testing"

	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// Components that subtract chrome (margins, a divider) from their space
// stay within constraints smaller than that chrome, down to zero: sizes
// are clamped, never negative.
func TestTinyConstraints(t *testing.T) {
	th := kit.Dark()
	for _, px := range []int{0, 1, 8, 40} {
		// check is a pane or body that fails on negative constraints.
		check := func(name string) layout.Widget {
			return func(gtx layout.Context) layout.Dimensions {
				if c := gtx.Constraints; c.Min.X < 0 || c.Min.Y < 0 || c.Max.X < 0 || c.Max.Y < 0 {
					t.Errorf("%s in %dpx: constraints %v", name, px, c)
				}
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}
		}
		var split ui.SplitState
		var toasts ui.Toasts
		toasts.Push(kit.StatusGood, "Saved", "All changes saved")
		var dialog kit.Dialog
		dialog.Show()
		cases := map[string]func(gtx layout.Context) layout.Dimensions{
			"Split": func(gtx layout.Context) layout.Dimensions {
				return ui.Split{ID: "split", A: check("Split.A"), B: check("Split.B")}.Layout(gtx, th, &split)
			},
			"ToastHost": func(gtx layout.Context) layout.Dimensions {
				return ui.ToastHost{Page: kit.Grow}.Layout(gtx, th, &toasts)
			},
			"Modal": func(gtx layout.Context) layout.Dimensions {
				return th.Modal(gtx, &dialog, "dlg", kit.Grow, func(gtx layout.Context) layout.Dimensions {
					return ui.DialogContent{Title: "Delete?", Body: check("Modal body")}.Layout(gtx, th)
				})
			},
		}
		for name, w := range cases {
			kittest.NewT(t, 200, 200, func(gtx layout.Context) {
				gtx.Constraints = layout.Exact(image.Pt(px, px))
				if d := w(gtx); d.Size.X > px || d.Size.Y > px || d.Size.X < 0 || d.Size.Y < 0 {
					t.Errorf("%s in %dpx: size %v", name, px, d.Size)
				}
			})
		}
	}
}

// A tooltip shows while its control holds keyboard focus, with no hover,
// as a tooltip node; Escape hides it until focus moves on.
func TestTooltipKeyboard(t *testing.T) {
	th := kit.Dark()
	var tip ui.TooltipState
	var btn, other kit.Clickable
	h := kittest.NewT(t, 300, 200, func(gtx layout.Context) {
		kit.Col(gtx,
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Tooltip{ID: "save:tip", Text: "Save the file", Content: func(gtx layout.Context) layout.Dimensions {
					return ui.Button{ID: "save", Text: "Save"}.Layout(gtx, th, &btn)
				}}.Layout(gtx, th, &tip)
			}),
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Button{ID: "other", Text: "Other"}.Layout(gtx, th, &other)
			}),
		)
	})
	h.ExpectNoID("save:tip")
	h.Tab(false)
	if h.FocusedID() != "save" {
		t.Fatalf("focused %q", h.FocusedID())
	}
	h.ExpectID("save:tip")
	if r := h.Role("save:tip"); r != kit.RoleTooltip {
		t.Errorf("role %q", r)
	}
	h.Key(key.NameEscape, 0)
	h.ExpectNoID("save:tip")
	h.Tab(false)
	h.Tab(true)
	h.ExpectID("save:tip")
}

// Two hosts in one scope keep their toasts apart by host ID. (Both stack
// at the window's corner; the outer host's toast is drawn on top.)
func TestToastHostIDs(t *testing.T) {
	th := kit.Dark()
	var a, b ui.Toasts
	a.Push(kit.StatusInfo, "A", "")
	b.Push(kit.StatusInfo, "B", "")
	h := kittest.NewT(t, 600, 400, func(gtx layout.Context) {
		ui.ToastHost{ID: "outer", Page: func(gtx layout.Context) layout.Dimensions {
			return ui.ToastHost{ID: "inner", Page: kit.Grow}.Layout(gtx, th, &b)
		}}.Layout(gtx, th, &a)
	})
	h.ExpectID("outer/1")
	h.ExpectID("inner/1")
	h.ClickID("outer/1/dismiss")
	h.ExpectNoID("outer/1")
	h.ExpectID("inner/1")
}
