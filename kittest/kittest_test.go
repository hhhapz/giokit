package kittest

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

// Each test rebuilds a real bug from giokit's history as a tiny widget and
// checks that the harness catches it without any app-specific assertion.

type C = layout.Context
type D = layout.Dimensions

func kinds(ps []Problem) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Kind)
	}
	return out
}

func harness(t *testing.T, draw func(gtx C)) *Harness {
	t.Helper()
	h, err := New(300, 200, 1, draw)
	if err != nil {
		NoGPU(t, err)
	}
	t.Cleanup(h.Release)
	return h
}

var th = kit.Dark()

func box(gtx C, w, h int) D { return D{Size: image.Pt(w, h)} }

// Bug: hover read before the widget consumed this frame's events, so the
// highlight trailed the cursor by one event.
func TestCatchesLateHoverRead(t *testing.T) {
	var g gesture.Hover
	var last bool
	h := harness(t, func(gtx C) {
		hovered := last // BUG: last frame's value; Update runs after
		last = g.Update(gtx.Source)
		bg := th.Bg
		if hovered {
			bg = th.Panel
		}
		kit.Fill(gtx, bg, func(gtx C) D {
			defer clip.Rect{Max: image.Pt(100, 40)}.Push(gtx.Ops).Pop()
			g.Add(gtx.Ops)
			return box(gtx, 100, 40)
		})
	})
	h.Move(image.Pt(50, 20))
	if !slices.Contains(kinds(h.Problems()), "stale-frame") {
		t.Fatalf("late hover read not caught: %v", h.Problems())
	}
}

// Bug: state drawn before the click that changes it is handled, with no Notify.
func TestCatchesDrawBeforeHandle(t *testing.T) {
	var c widget.Clickable // plain Gio: no Notify on click
	n := 0
	h := harness(t, func(gtx C) {
		kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return th.Text(gtx, string(rune('0'+n)), kit.SizeText, th.Fg, kit.Regular) }),
			kit.Rigid(func(gtx C) D {
				if c.Clicked(gtx) { // BUG: handled after the count was drawn
					n++
				}
				return c.Layout(gtx, func(gtx C) D { kit.Tag(gtx, "btn"); return box(gtx, 60, 30) })
			}),
		)
	})
	h.ClickID("btn")
	if n != 1 || !slices.Contains(kinds(h.Problems()), "stale-frame") {
		t.Fatalf("draw-before-handle not caught (n=%d): %v", n, h.Problems())
	}
}

// Bug: a "fixed-size" box drawn as a Stack child collapsed to a sliver
// (Stack resets Min for Stacked children).
func TestCatchesTinyTarget(t *testing.T) {
	var c kit.Clickable
	h := harness(t, func(gtx C) {
		c.Press(gtx, "check", func(gtx C, st kit.State) D {
			return box(gtx, 4, 22) // what a collapsed Box left: just its glyph's width
		})
	})
	if !slices.Contains(kinds(h.Audit()), "tiny-target") {
		t.Fatalf("collapsed target not caught: %v", h.Audit())
	}
}

func TestCatchesUnlabeledClickable(t *testing.T) {
	var c widget.Clickable
	h := harness(t, func(gtx C) { c.Layout(gtx, func(gtx C) D { return box(gtx, 60, 30) }) })
	if !slices.Contains(kinds(h.Audit()), "unlabeled-clickable") {
		t.Fatalf("unlabeled clickable not caught: %v", h.Audit())
	}
}

// Bug class: a pointer-only custom button looks clickable but Tab skips it.
// The audit must find it before an app ships a keyboard-inaccessible control.
func TestCatchesTabUnreachable(t *testing.T) {
	var pointerOnly gesture.Click
	var good kit.Clickable
	h := harness(t, func(gtx C) {
		good.Clicked(gtx)
		kit.Row(gtx,
			kit.Rigid(func(gtx C) D {
				defer clip.Rect{Max: image.Pt(80, 32)}.Push(gtx.Ops).Pop()
				pointerOnly.Add(gtx.Ops)
				defer clip.Rect{Max: image.Pt(80, 32)}.Push(gtx.Ops).Pop()
				kit.TagLabel(gtx, "pointer-only", "Pointer only")
				semantic.Button.Add(gtx.Ops)
				return box(gtx, 80, 32)
			}),
			kit.Rigid(func(gtx C) D { return ui.Button{ID: "good", Text: "Good"}.Layout(gtx, th, &good) }),
		)
	})
	got := h.TabUnreachable()
	if !slices.Contains(got, "pointer-only") || slices.Contains(got, "good") {
		t.Fatalf("TabUnreachable = %v", got)
	}
	var find func([]A11yNode, string) (A11yNode, bool)
	find = func(nodes []A11yNode, id string) (A11yNode, bool) {
		for _, n := range nodes {
			if n.ID == id {
				return n, true
			}
			if child, ok := find(n.Children, id); ok {
				return child, true
			}
		}
		return A11yNode{}, false
	}
	n, ok := find(h.A11yTree(), "pointer-only")
	if !ok || n.Label != "Pointer only" || n.Class != semantic.Button {
		t.Fatalf("snapshot node: %+v, found %v", n, ok)
	}
}

// Bug class: an audit counting only semantic buttons stops before later
// buttons when focusable non-button regions are in the Tab order.
func TestTabAuditTraversesExtraFocusStops(t *testing.T) {
	var first, last kit.Clickable
	var tags [5]int
	first.FocusOnClick = true
	h := harness(t, func(gtx C) {
		first.Clicked(gtx)
		last.Clicked(gtx)
		for i := range tags {
			for {
				_, ok := gtx.Event(key.FocusFilter{Target: &tags[i]})
				if !ok {
					break
				}
			}
		}
		children := []layout.FlexChild{kit.Rigid(func(gtx C) D { return ui.Button{ID: "first", Text: "First"}.Layout(gtx, th, &first) })}
		for i := range tags {
			i := i
			children = append(children, kit.Rigid(func(gtx C) D {
				defer clip.Rect{Max: image.Pt(12, 32)}.Push(gtx.Ops).Pop()
				event.Op(gtx.Ops, &tags[i])
				return D{Size: image.Pt(12, 32)}
			}))
		}
		children = append(children, kit.Rigid(func(gtx C) D { return ui.Button{ID: "last", Text: "Last"}.Layout(gtx, th, &last) }))
		kit.Row(gtx, children...)
	})
	h.ClickID("first")
	if got := h.TabUnreachable(); len(got) != 0 {
		t.Fatalf("reachable button missed after extra Tab stops: %v", got)
	}
}

// Bug: "↻ Sync" rendered as a tofu box with the bundled Go fonts.
func TestCatchesMissingGlyph(t *testing.T) {
	h := harness(t, func(gtx C) { th.Text(gtx, "↻ Sync", kit.SizeText, th.Fg, kit.Regular) })
	h.Theme = th
	if !slices.Contains(kinds(h.Audit()), "missing-glyph") {
		t.Fatalf("missing glyph not caught: %v", h.Audit())
	}
}

func TestCatchesOcclusionAndDuplicates(t *testing.T) {
	var a, b, c kit.Clickable
	h := harness(t, func(gtx C) {
		a.Press(gtx, "under", func(gtx C, _ kit.State) D { return box(gtx, 80, 40) })
		b.Press(gtx, "over", func(gtx C, _ kit.State) D { return box(gtx, 80, 40) }) // same spot, drawn later
		layout.Inset{Top: 60}.Layout(gtx, func(gtx C) D {
			return c.Press(gtx, "over", func(gtx C, _ kit.State) D { return box(gtx, 80, 40) })
		})
	})
	got := kinds(h.Audit())
	if !slices.Contains(got, "occluded") || !slices.Contains(got, "duplicate-id") {
		t.Fatalf("want occluded + duplicate-id, got %v", h.Audit())
	}
}

func TestCatchesRedrawLoop(t *testing.T) {
	h := harness(t, func(gtx C) { kit.Notify(gtx) })
	if !slices.Contains(kinds(h.Problems()), "no-settle") {
		t.Fatalf("redraw loop not caught: %v", h.Problems())
	}
}

// The correct patterns pass every check.
func TestCleanWidgetsPass(t *testing.T) {
	var hv kit.Hover
	var c kit.Clickable
	n := 0
	h := NewT(t, 300, 200, func(gtx C) {
		if c.Clicked(gtx) {
			n++
		}
		kit.Col(gtx,
			kit.Rigid(func(gtx C) D {
				return hv.Layout(gtx, "row", func(gtx C, on bool) D {
					bg := th.Bg
					if on {
						bg = th.Panel
					}
					return kit.Fill(gtx, bg, func(gtx C) D { return box(gtx, 200, 40) })
				})
			}),
			kit.Rigid(func(gtx C) D { return ui.Button{ID: "btn", Text: "Count", Style: ui.Primary}.Layout(gtx, th, &c) }),
			kit.Rigid(func(gtx C) D { return th.Text(gtx, string(rune('0'+n)), kit.SizeText, th.Fg, kit.Regular) }),
		)
	})
	h.Theme = th
	h.Hover("row")
	h.ExpectBg("row", th.Panel)
	h.ClickID("btn")
	h.ExpectText("1")
	h.ExpectBg("row", th.Bg)
	h.ExpectClean()
}

// Bug (LogView, first draft): layout.Stack lays out Stacked children before
// Expanded ones. An overlay drawn as a Stacked child read state that the
// Expanded child (a list) only updated while laying out, so it showed a
// frame late.
func TestCatchesStackOrderLateRead(t *testing.T) {
	var c widget.Clickable // plain Gio: no Notify
	open := false
	h := harness(t, func(gtx C) {
		layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx C) D {
				if c.Clicked(gtx) {
					open = true // mutated while the Stacked child is already drawn
				}
				return c.Layout(gtx, func(gtx C) D { kit.Tag(gtx, "area"); return box(gtx, 200, 100) })
			}),
			layout.Stacked(func(gtx C) D {
				if open {
					return th.Text(gtx, "overlay", kit.SizeText, th.Fg, kit.Regular)
				}
				return D{}
			}),
		)
	})
	h.ClickID("area")
	if !slices.Contains(kinds(h.Problems()), "stale-frame") {
		t.Fatalf("stack-order late read not caught: %v", h.Problems())
	}
}

// Finite animations settle on the virtual clock: no no-settle, and the
// settled frame shows the end state.
func TestAnimationSettles(t *testing.T) {
	var c kit.Clickable
	var tw kit.Tween
	on := false
	h := NewT(t, 300, 200, func(gtx C) {
		if c.Clicked(gtx) {
			on = !on
		}
		target := float32(0)
		if on {
			target = 1
		}
		v := tw.To(gtx, target, 300*time.Millisecond)
		ui.Button{ID: "btn", Text: "Go"}.Layout(gtx, th, &c)
		layout.Inset{Top: 60}.Layout(gtx, func(gtx C) D {
			return kit.Fill(gtx, kit.LerpColor(th.Bg, th.Accent, v), func(gtx C) D {
				defer clip.Rect{Max: image.Pt(60, 30)}.Push(gtx.Ops).Pop() // Tag names the current clip area
				kit.Tag(gtx, "swatch")
				return box(gtx, 60, 30)
			})
		})
	})
	start := h.Now
	h.ClickID("btn")
	if h.Now.Sub(start) < 300*time.Millisecond {
		t.Errorf("virtual clock advanced only %v during a 300ms animation", h.Now.Sub(start))
	}
	h.ExpectBg("swatch", th.Accent)
}

// Continuous animations (kit.Loop) schedule frames instead of requesting
// them immediately: the UI still settles, and Tab still moves focus (a
// leftover scheduled wakeup once made every key look "handled").
func TestLoopSettlesAndTabStillWorks(t *testing.T) {
	var a, b kit.Clickable
	var focusedB bool
	h := NewT(t, 300, 200, func(gtx C) {
		kit.Loop(gtx, time.Second)
		kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return ui.Button{ID: "a", Text: "A"}.Layout(gtx, th, &a) }),
			kit.Rigid(func(gtx C) D { return ui.Button{ID: "b", Text: "B"}.Layout(gtx, th, &b) }),
		)
		focusedB = gtx.Focused(b.FocusTag())
	})
	h.Tab(false)
	h.Tab(false)
	if !focusedB {
		t.Fatal("two Tabs did not focus the second button while a Loop animation ran")
	}
}

// Editors draw their own hit area; kit.TextField's id names it, a bare
// editor is flagged.
func TestEditorLabeling(t *testing.T) {
	var labeled, bare widget.Editor
	h := harness(t, func(gtx C) {
		kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return ui.TextField{ID: "name", Placeholder: "Name"}.Layout(gtx, th, &labeled) }),
			kit.Rigid(func(gtx C) D { return material.Editor(th.Material(), &bare, "bare").Layout(gtx) }),
		)
	})
	var unlabeled int
	for _, p := range h.Audit() {
		if p.Kind == "unlabeled-clickable" {
			unlabeled++
		}
	}
	if unlabeled != 1 {
		t.Fatalf("want exactly the bare editor flagged, got %v", h.Audit())
	}
}

// A list whose rows change height as they reach the top jumps the content
// mid-scroll; kit.List's continuity check must count that as a skip.
func TestListContinuityCatchesJumps(t *testing.T) {
	var l kit.List
	h := harness(t, func(gtx C) {
		th.List(gtx, &l, "list", 50, func(gtx C, i int) D {
			hgt := 30
			if i == l.Position.First { // BUG: the top row grows when it becomes first
				hgt = 60
			}
			return box(gtx, 200, hgt)
		})
	})
	h.Strict = false
	at := Center(h.must("list"))
	for i := 0; i < 12; i++ {
		h.Scroll(at, 7)
	}
	if l.Jumps() == 0 {
		t.Fatal("content jump not detected")
	}
	var ok kit.List // the same list with stable rows: no jumps
	h2 := harness(t, func(gtx C) {
		th.List(gtx, &ok, "list", 50, func(gtx C, i int) D { return box(gtx, 200, 30) })
	})
	h2.Strict = false
	for i := 0; i < 12; i++ {
		h2.Scroll(at, 7)
	}
	if ok.Jumps() != 0 {
		t.Fatalf("stable list reported %d jumps", ok.Jumps())
	}
}

// Bug class: tertiary gray text (placeholders, captions, key hints) on a
// raised surface reads fine on a bright dev screen and fails WCAG AA. The
// contrast audit measures the pixels; disabled text is exempt.
func TestCatchesLowContrastText(t *testing.T) {
	disabled, faint := false, th.Dim
	h := harness(t, func(gtx C) {
		kit.Fill(gtx, th.Surface, func(gtx C) D {
			return kit.Col(gtx,
				kit.Rigid(func(gtx C) D { return th.Text(gtx, "readable", th.Type.Body, th.Fg, kit.Regular) }),
				kit.Rigid(func(gtx C) D {
					return th.Disable(gtx, disabled, func(gtx C) D {
						return th.Text(gtx, "faint", th.Type.Body, faint, kit.Regular)
					})
				}),
			)
		})
	})
	// th's palette is Legible, so stock Dim passes: the bug is a color that
	// skipped it (hand-picked, or from a token file).
	faint = kit.WithAlpha(th.Dim, 110)
	h.Settle()
	h.MinContrast = 0
	if ps := h.Audit(); len(ps) != 0 {
		t.Fatalf("contrast audit ran while off: %v", ps)
	}
	h.MinContrast = 4.5 // the default
	ps := h.Audit()
	if len(ps) != 1 || ps[0].Kind != "low-contrast" || !strings.Contains(ps[0].Msg, `"faint"`) {
		t.Fatalf("want one low-contrast finding for \"faint\", got %v", ps)
	}
	disabled = true
	h.Settle()
	if ps := h.Audit(); len(ps) != 0 {
		t.Fatalf("disabled text should be exempt: %v", ps)
	}
}

// Bug class: a "modal" drawn over the page without blocking it. Clicks
// may be swallowed by a scrim, but Tab walks straight out of the dialog
// into the page behind. FocusEscape catches it; th.Modal passes.
func TestCatchesLeakyFocusTrap(t *testing.T) {
	var page, ok, cancel kit.Clickable
	var dlg kit.Dialog
	leaky := true
	h := harness(t, func(gtx C) {
		page.Clicked(gtx)
		ok.Clicked(gtx)
		cancel.Clicked(gtx)
		pageW := func(gtx C) D {
			return kit.Fill(gtx, th.Bg, func(gtx C) D { return ui.Button{ID: "page", Text: "Page button"}.Layout(gtx, th, &page) })
		}
		dialogW := func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(func(gtx C) D { return ui.Button{ID: "cancel", Text: "Cancel"}.Layout(gtx, th, &cancel) }),
				kit.Rigid(func(gtx C) D { return ui.Button{ID: "ok", Text: "OK", Style: ui.Solid}.Layout(gtx, th, &ok) }),
			)
		}
		if !leaky {
			th.Modal(gtx, &dlg, "dlg", pageW, dialogW)
			return
		}
		// BUG: the page stays live under the "dialog".
		pageW(gtx)
		layout.Inset{Top: 80, Left: 60}.Layout(gtx, func(gtx C) D {
			return kit.Surface{Bg: th.Overlay}.Layout(gtx, func(gtx C) D {
				defer clip.Rect{Max: image.Pt(200, 60)}.Push(gtx.Ops).Pop()
				kit.Tag(gtx, "dlg")
				return dialogW(gtx)
			})
		})
	})
	if out := h.FocusEscape("dlg", 3); out != "page" {
		t.Fatalf("leaky dialog: FocusEscape = %q, want it to reach \"page\"", out)
	}
	leaky = false
	dlg.Show()
	h.Settle()
	if out := h.FocusEscape("dlg", 3); out != "" {
		t.Fatalf("th.Modal: focus escaped to %q", out)
	}
	if h.FocusedID() == "" {
		t.Fatal("FocusedID: nothing focused inside the dialog after tabbing")
	}
}

// The audit lets an open popup's backdrop and panel cover the page: that's
// their job, not an occlusion bug. (Real overlaps are still reported:
// TestCatchesOcclusionAndDuplicates.)
func TestLayersAndRealOcclusion(t *testing.T) {
	var p kit.Popup
	var trig, under, item kit.Clickable
	h := harness(t, func(gtx C) {
		if trig.Clicked(gtx) {
			p.Toggle()
		}
		under.Clicked(gtx)
		item.Clicked(gtx)
		kit.Fill(gtx, th.Bg, func(gtx C) D {
			return kit.Col(gtx,
				kit.Rigid(func(gtx C) D {
					return th.Popover(gtx, &p, "pop", func(gtx C) D { return ui.Button{ID: "trig", Text: "Open"}.Layout(gtx, th, &trig) },
						func(gtx C) D {
							return kit.Pad(30, 30, func(gtx C) D { return ui.Button{ID: "item", Text: "Item", Style: ui.Ghost}.Layout(gtx, th, &item) })(gtx)
						})
				}),
				kit.Rigid(func(gtx C) D { return ui.Button{ID: "under", Text: "Under the popup"}.Layout(gtx, th, &under) }),
				kit.Rigid(func(gtx C) D { return th.Text(gtx, "hidden text", th.Type.Body, th.Dim, kit.Regular) }),
			)
		})
	})
	h.Theme = th
	h.ClickID("trig")
	if ps := h.Audit(); len(ps) != 0 {
		t.Fatalf("open popup over the page: %v", ps)
	}
	if !p.Opened() {
		t.Fatal("popup didn't open")
	}
}

// Bug class: focus held by a list row is lost when that row scrolls out of a
// virtualized list and is no longer laid out, so arrow keys stop working.
// A focus group holds focus on the container instead.
func TestFocusGroupSurvivesVirtualizedRows(t *testing.T) {
	var s ui.SelectableListState
	sel := ""
	choices := make([]ui.Choice, 100)
	for i := range choices {
		choices[i] = ui.Choice{ID: fmt.Sprint(i), Label: fmt.Sprint("Item ", i)}
	}
	h := harness(t, func(gtx C) {
		if id, ok := s.Update(gtx); ok {
			sel = id
		}
		ui.SelectableList{ID: "list", Items: choices, Selected: sel}.Layout(gtx, th, &s)
	})
	h.ClickID("list/0")
	h.Key(key.NameEnd, 0) // row 99 is far below the 200px viewport
	if sel != "99" || h.FocusedID() != "list/99" {
		t.Fatalf("after End: selected %q, focus %q", sel, h.FocusedID())
	}
	if r, ok := h.Find("list/99"); !ok || r.Max.Y > h.Size.Y {
		t.Fatalf("End did not scroll the last row into view: %v %v", r, ok)
	}
	h.Key(key.NameUpArrow, 0)
	if sel != "98" {
		t.Fatalf("arrow after scrolling out: selected %q", sel)
	}
}

// Bug class: two instances of a composite on one screen produce the same ids,
// so tests cannot tell them apart and the audit reports duplicates. Scope
// gives each instance its own path without the component knowing.
func TestScopeSeparatesInstances(t *testing.T) {
	var a, b kit.Clickable
	scoped, bClicks := false, 0
	h := harness(t, func(gtx C) {
		a.Clicked(gtx)
		if b.Clicked(gtx) {
			bClicks++
		}
		card := func(c *kit.Clickable, name string) kit.W {
			return func(gtx C) D {
				w := func(gtx C) D { return ui.Button{ID: "save", Text: "Save"}.Layout(gtx, th, c) }
				if scoped {
					return kit.Scope(gtx, name, w)
				}
				return w(gtx)
			}
		}
		kit.Row(gtx, kit.Rigid(card(&a, "invoice:1")), kit.Rigid(card(&b, "invoice/2")))
	})
	if !slices.Contains(kinds(h.Audit()), "duplicate-id") {
		t.Fatal("unscoped twins not reported as duplicates")
	}
	scoped = true
	h.Settle()
	if got := kinds(h.Audit()); slices.Contains(got, "duplicate-id") {
		t.Fatalf("scoped twins still collide: %v", got)
	}
	if _, ok := h.Find("invoice:1/save"); !ok {
		t.Fatal("scoped id invoice:1/save not found")
	}
	h.ClickID("invoice%2F2/save") // a "/" inside a scope name is escaped
	if bClicks != 1 {
		t.Fatalf("second instance clicked %d times", bClicks)
	}
}

// Bug class: Escape closes the wrong layer (the dialog behind an open
// popover, or nothing when focus sits on the popover's trigger). Escape is
// an action resolved from the focus outward, so the innermost layer that
// contains focus closes first, and only then the one around it.
func TestEscapeClosesInnermostLayer(t *testing.T) {
	var d kit.Dialog
	var p kit.Popup
	var trigger, item kit.Clickable
	h := harness(t, func(gtx C) {
		if trigger.Clicked(gtx) {
			p.Toggle()
		}
		item.Clicked(gtx)
		th.Modal(gtx, &d, "dialog", func(gtx C) D { return D{Size: gtx.Constraints.Max} }, func(gtx C) D {
			return th.Popover(gtx, &p, "pop", func(gtx C) D {
				return ui.Button{ID: "trigger", Text: "Open"}.Layout(gtx, th, &trigger)
			}, func(gtx C) D { return ui.Button{ID: "item", Text: "Item"}.Layout(gtx, th, &item) })
		})
	})
	d.Show()
	h.Settle()
	h.ClickID("trigger")
	if !p.Opened() {
		t.Fatal("popover did not open")
	}
	h.Key(key.NameEscape, 0)
	if p.Opened() || !d.Opened() {
		t.Fatalf("first Escape: popover open %v, dialog open %v; want only the popover closed", p.Opened(), d.Opened())
	}
	h.Key(key.NameEscape, 0)
	if d.Opened() {
		t.Fatal("second Escape did not close the dialog")
	}
}

// Bug class (false positive): a button scrolled half out of a list, under a
// header, was reported occluded by the header, and a clipped sliver of a
// label as low contrast. Clipped by scrolling is not covered.
func TestAuditIgnoresHalfScrolledRows(t *testing.T) {
	var l kit.List
	var header kit.Clickable
	rows := make([]kit.Clickable, 30)
	h := harness(t, func(gtx C) {
		header.Clicked(gtx)
		for i := range rows {
			rows[i].Clicked(gtx)
		}
		kit.Fill(gtx, th.Bg, func(gtx C) D {
			return kit.Col(gtx,
				kit.Rigid(func(gtx C) D { return ui.Button{ID: "header", Text: "Header"}.Layout(gtx, th, &header) }),
				kit.Flexed(1, func(gtx C) D {
					return th.List(gtx, &l, "list", len(rows), func(gtx C, i int) D {
						return ui.Button{ID: fmt.Sprint("row:", i), Text: fmt.Sprint("Row ", i)}.Layout(gtx, th, &rows[i])
					})
				}),
			)
		})
	})
	l.Position.First, l.Position.Offset = 2, 13 // row 2 half under the header
	h.Settle()
	if got := kinds(h.Audit()); len(got) != 0 {
		t.Fatalf("half-scrolled rows reported: %v", h.Audit())
	}
}

// Bug: at small control sizes a text field's editor (its only click target)
// was 15px tall, so clicking the field's padding did nothing. The whole field
// is the target now; the audit judges the field, and still flags a bare
// tiny clickable.
func TestTextFieldWholeTarget(t *testing.T) {
	small := th.Derive(func(t *kit.Theme) { t.Controls, t.Type = kit.ControlSmall, t.Type.Scaled(0.9) })
	var e widget.Editor
	var tiny kit.Clickable
	h := harness(t, func(gtx C) {
		tiny.Clicked(gtx)
		kit.Col(gtx,
			kit.Rigid(kit.Fixed(200, func(gtx C) D { return ui.TextField{ID: "name", Placeholder: "Name"}.Layout(gtx, small, &e) })),
			kit.Rigid(func(gtx C) D {
				return tiny.Press(gtx, "tiny", func(gtx C, st kit.State) D {
					semantic.Button.Add(gtx.Ops)
					semantic.LabelOp("Tiny").Add(gtx.Ops)
					return D{Size: image.Pt(40, 10)}
				})
			}),
		)
	})
	r, _ := h.Find("name")
	h.Click(image.Pt(r.Min.X+2, r.Min.Y+2)) // the padding, outside the text line
	if h.FocusedID() != "name" {
		t.Fatalf("a press on the field's padding did not focus it (focus %q)", h.FocusedID())
	}
	var tinies []string
	for _, p := range h.Audit() {
		if p.Kind == "tiny-target" {
			tinies = append(tinies, p.ID)
		}
	}
	if !slices.Equal(tinies, []string{"tiny"}) {
		t.Fatalf("tiny targets = %q, want only the bare tiny button", tinies)
	}
}

// A named group exactly the size of its only control (a one-action toolbar)
// gives two nodes with the focused bounds: FocusedID must name the control,
// the innermost, not the group. Found by the Toolbar conformance row.
func TestFocusedIDPrefersInnermost(t *testing.T) {
	th := kit.Dark()
	var c kit.Clickable
	h := NewT(t, 300, 120, func(gtx kit.C) {
		c.Clicked(gtx)
		gtx.Constraints.Min = image.Point{}
		kit.Group(gtx, "group", "Actions", func(gtx kit.C) kit.D {
			return ui.Button{ID: "group/save", Text: "Save"}.Layout(gtx, th, &c)
		})
	})
	h.Tab(false)
	if got := h.FocusedID(); got != "group/save" {
		t.Fatalf("FocusedID %q, want the button inside the same-sized group", got)
	}
}
