package kittest

import (
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hhhapz/giokit/gio/font"
	"github.com/hhhapz/giokit/gio/gesture"
	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/input"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/io/semantic"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

// Each test rebuilds a real bug from giokit's history as a tiny widget and
// checks that the harness catches it without any app-specific assertion.

type C = layout.Context
type D = layout.Dimensions

func kinds(ps []Problem) []ProblemKind {
	var out []ProblemKind
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
	if !slices.Contains(kinds(h.Problems()), ProblemStaleFrame) {
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
	if n != 1 || !slices.Contains(kinds(h.Problems()), ProblemStaleFrame) {
		t.Fatalf("draw-before-handle not caught (n=%d): %v", n, h.Problems())
	}
}

// Bug: a "fixed-size" box drawn as a Stack child collapsed to a sliver
// (Stack resets Min for Stacked children).
func TestCatchesTinyTarget(t *testing.T) {
	var c kit.Clickable
	h := harness(t, func(gtx C) {
		th.Press(gtx, &c, "check", func(gtx C, st kit.State) D {
			return box(gtx, 4, 22) // what a collapsed Box left: just its glyph's width
		})
	})
	if !slices.Contains(kinds(h.Audit()), ProblemTinyTarget) {
		t.Fatalf("collapsed target not caught: %v", h.Audit())
	}
}

func TestCatchesUnlabeledClickable(t *testing.T) {
	var c widget.Clickable
	h := harness(t, func(gtx C) { c.Layout(gtx, func(gtx C) D { return box(gtx, 60, 30) }) })
	if !slices.Contains(kinds(h.Audit()), ProblemUnlabeledClickable) {
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

// Bug class: a custom control with no Gio semantic class (a slider, a split
// handle) is skipped by a reachability check that only knows upstream Gio's
// classes. Its role says it must be reachable.
func TestCatchesTabUnreachableByRole(t *testing.T) {
	var press gesture.Click // a pointer handler and nothing for the keyboard
	h := harness(t, func(gtx C) {
		for {
			if _, ok := press.Update(gtx.Source); !ok {
				break
			}
		}
		layout.UniformInset(8).Layout(gtx, func(gtx C) D {
			defer clip.Rect{Max: image.Pt(120, 24)}.Push(gtx.Ops).Pop()
			press.Add(gtx.Ops)
			kit.TagLabel(gtx, "volume", "Volume")
			kit.RoleSlider.Add(gtx.Ops)
			return box(gtx, 120, 24)
		})
	})
	if got := h.TabUnreachable(); !slices.Contains(got, "volume") {
		t.Fatalf("pointer-only slider not reported: %v", got)
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
	if !slices.Contains(kinds(h.Audit()), ProblemMissingGlyph) {
		t.Fatalf("missing glyph not caught: %v", h.Audit())
	}
}

func TestCatchesOcclusionAndDuplicates(t *testing.T) {
	var a, b, c kit.Clickable
	h := harness(t, func(gtx C) {
		th.Press(gtx, &a, "under", func(gtx C, _ kit.State) D { return box(gtx, 80, 40) })
		th.Press(gtx, &b, "over", func(gtx C, _ kit.State) D { return box(gtx, 80, 40) }) // same spot, drawn later
		layout.Inset{Top: 60}.Layout(gtx, func(gtx C) D {
			return th.Press(gtx, &c, "over", func(gtx C, _ kit.State) D { return box(gtx, 80, 40) })
		})
	})
	got := kinds(h.Audit())
	if !slices.Contains(got, ProblemOccluded) || !slices.Contains(got, ProblemDuplicateID) {
		t.Fatalf("want occluded + duplicate-id, got %v", h.Audit())
	}
}

// Bug class: a control announces its selection before it handles the click
// that changes it. The pixels are right (they are drawn after the click), so
// only a screen reader hears the old state, until the next input. Strict
// mode must compare semantic state, not only ids, labels and pixels.
func TestCatchesSemanticOnlyStale(t *testing.T) {
	var c widget.Clickable // plain Gio: no Notify on click
	on := false
	h := harness(t, func(gtx C) {
		kit.Col(gtx,
			kit.Rigid(func(gtx C) D {
				defer clip.Rect{Max: image.Pt(60, 30)}.Push(gtx.Ops).Pop()
				kit.TagLabel(gtx, "opt", "Option; a|b") // delimiters must not confuse the diff
				semantic.CheckBox.Add(gtx.Ops)
				semantic.SelectedOp(on).Add(gtx.Ops) // BUG: announced before the click below
				return box(gtx, 60, 30)
			}),
			kit.Rigid(func(gtx C) D {
				if c.Clicked(gtx) {
					on = !on
				}
				return c.Layout(gtx, func(gtx C) D { kit.Tag(gtx, "btn"); return box(gtx, 60, 30) })
			}),
		)
	})
	h.ClickID("btn")
	var stale []Problem
	for _, p := range h.Problems() {
		if p.Kind == ProblemStaleFrame {
			stale = append(stale, p)
		}
	}
	if !on || len(stale) == 0 {
		t.Fatalf("selection-only stale frame not caught (on=%v): %v", on, h.Problems())
	}
	if msg := stale[0].Msg; !strings.Contains(msg, `"opt"`) || !strings.Contains(msg, "selected false → true") {
		t.Fatalf("diagnostic does not name the node and field: %s", msg)
	}
}

func TestCatchesRedrawLoop(t *testing.T) {
	h := harness(t, func(gtx C) { kit.Notify(gtx) })
	if !slices.Contains(kinds(h.Problems()), ProblemNoSettle) {
		t.Fatalf("redraw loop not caught: %v", h.Problems())
	}
}

// fakeTB records what a harness reports, so a test can check that a
// failure reaches the test at all.
type fakeTB struct {
	errors   []string
	cleanups []func()
}

func (*fakeTB) Helper()                     {}
func (*fakeTB) Name() string                { return "fake" }
func (f *fakeTB) Error(args ...any)         { f.errors = append(f.errors, fmt.Sprint(args...)) }
func (f *fakeTB) Errorf(s string, a ...any) { f.errors = append(f.errors, fmt.Sprintf(s, a...)) }
func (*fakeTB) Fatal(args ...any)           { panic(fmt.Sprint(args...)) }
func (*fakeTB) Fatalf(s string, a ...any)   { panic(fmt.Sprintf(s, a...)) }
func (*fakeTB) Skip(args ...any)            { panic(fmt.Sprint(args...)) }
func (f *fakeTB) Cleanup(c func())          { f.cleanups = append(f.cleanups, c) }

func (f *fakeTB) runCleanups() {
	for _, c := range slices.Backward(f.cleanups) {
		c()
	}
}

// Bug: NewT settled the first frames before attaching the test, so a UI
// that never settles recorded no-settle where nobody looked, and the test
// passed. Construction failures must fail the test, and the harness must
// still be released.
func TestNewTReportsConstructionProblems(t *testing.T) {
	if _, err := New(10, 10, 1, func(C) {}); err != nil {
		NoGPU(t, err)
	}
	f := new(fakeTB)
	h := NewT(f, 50, 50, func(gtx C) { kit.Notify(gtx) })
	if len(f.errors) == 0 || !strings.Contains(f.errors[0], string(ProblemNoSettle)) {
		t.Fatalf("construction problem not reported: errors %q, recorded %v", f.errors, h.Problems())
	}
	if len(f.cleanups) == 0 {
		t.Fatal("no cleanup registered to release the harness")
	}
	f.runCleanups()
}

// attributionCases fail on purpose, each in a child process
// (KITTEST_ATTRIBUTION names the case), so TestHelperAttribution can read
// which source line the failure was reported at.
var attributionCases = map[string]func(t *testing.T){
	"construction": func(t *testing.T) { NewT(t, 50, 50, func(gtx C) { kit.Notify(gtx) }) },
	"expect-text":  func(t *testing.T) { NewT(t, 50, 50, func(C) {}).ExpectText("missing") },
	"expect-no-id": func(t *testing.T) {
		NewT(t, 50, 50, func(gtx C) {
			defer clip.Rect{Max: image.Pt(20, 20)}.Push(gtx.Ops).Pop()
			kit.Tag(gtx, "here")
		}).ExpectNoID("here")
	},
	"expect-clean": func(t *testing.T) {
		var c widget.Clickable
		NewT(t, 100, 50, func(gtx C) { c.Layout(gtx, func(gtx C) D { return box(gtx, 60, 30) }) }).ExpectClean()
	},
	"click-missing": func(t *testing.T) { NewT(t, 50, 50, func(C) {}).ClickID("missing") },
	"stale-after-click": func(t *testing.T) {
		var c widget.Clickable
		n := 0
		h := NewT(t, 100, 80, func(gtx C) {
			kit.Col(gtx,
				kit.Rigid(func(gtx C) D { return th.Text(gtx, string(rune('0'+n)), kit.SizeText, th.Fg, kit.Regular) }),
				kit.Rigid(func(gtx C) D {
					if c.Clicked(gtx) {
						n++
					}
					return c.Layout(gtx, func(gtx C) D { kit.Tag(gtx, "btn"); return box(gtx, 60, 30) })
				}),
			)
		})
		h.ClickID("btn")
	},
}

func TestHelperAttributionChild(t *testing.T) {
	name := os.Getenv("KITTEST_ATTRIBUTION")
	if name == "" {
		t.Skip("run by TestHelperAttribution")
	}
	attributionCases[name](t)
}

// Bug: assertion helpers marked only an inner method as a helper, so a
// failure pointed into kittest instead of the test's own line.
func TestHelperAttribution(t *testing.T) {
	if os.Getenv("KITTEST_ATTRIBUTION") != "" {
		t.Skip()
	}
	if _, err := New(10, 10, 1, func(C) {}); err != nil {
		NoGPU(t, err)
	}
	location := regexp.MustCompile(`(?m)^\s+(\w+\.go):\d+: `)
	for name := range attributionCases {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperAttributionChild$", "-test.v")
			cmd.Env = append(os.Environ(), "KITTEST_ATTRIBUTION="+name)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the case did not fail:\n%s", out)
			}
			found := location.FindAllStringSubmatch(string(out), -1)
			if len(found) == 0 {
				t.Fatalf("no failure location in:\n%s", out)
			}
			for _, m := range found {
				if m[1] != "kittest_test.go" {
					t.Fatalf("failure reported at %s, not the test's line:\n%s", m[1], out)
				}
			}
		})
	}
}

// The correct patterns pass every check.
func TestCleanWidgetsPass(t *testing.T) {
	var hv kit.HoverArea
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
	if !slices.Contains(kinds(h.Problems()), ProblemStaleFrame) {
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
			kit.Rigid(func(gtx C) D {
				return bare.Layout(gtx, th.Shaper, font.Font{Typeface: th.UI}, th.Type.Body, op.CallOp{}, op.CallOp{})
			}),
		)
	})
	var unlabeled int
	for _, p := range h.Audit() {
		if p.Kind == ProblemUnlabeledClickable {
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
	if len(ps) != 1 || ps[0].Kind != ProblemLowContrast || !strings.Contains(ps[0].Msg, `"faint"`) {
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
	if leak, ok := h.FocusEscape("dlg", 3); !ok || leak.To != "page" {
		t.Fatalf("leaky dialog: FocusEscape = %v, %v, want it to reach \"page\"", leak, ok)
	}
	leaky = false
	dlg.Show()
	h.Settle()
	if leak, ok := h.FocusEscape("dlg", 3); ok {
		t.Fatalf("th.Modal: %v", leak)
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
		if e, ok := s.Update(gtx); ok {
			sel = e.Selected
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
		card := func(c *kit.Clickable, name string) layout.Widget {
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
	if !slices.Contains(kinds(h.Audit()), ProblemDuplicateID) {
		t.Fatal("unscoped twins not reported as duplicates")
	}
	scoped = true
	h.Settle()
	if got := kinds(h.Audit()); slices.Contains(got, ProblemDuplicateID) {
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
				return th.Press(gtx, &tiny, "tiny", func(gtx C, st kit.State) D {
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
		if p.Kind == ProblemTinyTarget {
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
	h := NewT(t, 300, 120, func(gtx C) {
		c.Clicked(gtx)
		gtx.Constraints.Min = image.Point{}
		kit.Group(gtx, "group", "Actions", 0, func(gtx C) D {
			return ui.Button{ID: "group/save", Text: "Save"}.Layout(gtx, th, &c)
		})
	})
	h.Tab(false)
	if got := h.FocusedID(); got != "group/save" {
		t.Fatalf("FocusedID %q, want the button inside the same-sized group", got)
	}
}

// Bug (found in an app's settings page): ClickID on a control laid out past
// the end of a scrolled list clicked whatever covered that point instead,
// and nothing failed until a later value assertion. Pointer helpers must
// refuse a widget with no visible area, naming the problem.
func TestPointerRefusesClippedWidget(t *testing.T) {
	var l kit.List
	var cs [4]kit.Clickable
	clicked := -1
	h := harness(t, func(gtx C) {
		for i := range cs {
			if cs[i].Clicked(gtx) {
				clicked = i
			}
		}
		th.List(gtx, &l, "list", len(cs), func(gtx C, i int) D {
			// a tall row whose button sits at its bottom: row 1 starts inside
			// the 200px window, but its button lies below it
			return layout.Inset{Top: 150}.Layout(gtx, func(gtx C) D {
				return ui.Button{ID: fmt.Sprint("row:", i), Text: "Toggle"}.Layout(gtx, th, &cs[i])
			})
		})
	})
	if _, ok := h.Find("row:1"); !ok {
		t.Fatal("setup: row 1 should be laid out (it starts inside the window)")
	}
	msg := func() (msg string) {
		defer func() {
			if r := recover(); r != nil {
				msg = fmt.Sprint(r)
			}
		}()
		h.ClickID("row:1")
		return ""
	}()
	if !strings.Contains(msg, "not visible") {
		t.Fatalf("ClickID on a clipped widget did not fail (clicked %d): %q", clicked, msg)
	}
	h.ClickID("row:0") // the visible one still works
	if clicked != 0 {
		t.Fatalf("visible row: clicked %d", clicked)
	}
}

// Bug class: a key handler that captures its starting position once per
// frame applies the same move twice when two keys arrive in one frame.
// Key settles between keys and so never shows it; Keys delivers a batch.
func TestKeysBatchInOneFrame(t *testing.T) {
	var batches [][]key.Name
	h := harness(t, func(gtx C) {
		var batch []key.Name
		for {
			e, ok := gtx.Event(key.Filter{Name: key.NameDownArrow}, key.Filter{Name: key.NameUpArrow})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				batch = append(batch, ke.Name)
			}
		}
		if batch != nil {
			batches = append(batches, batch)
		}
	})
	h.Keys(key.NameDownArrow, key.NameUpArrow, key.NameDownArrow)
	if len(batches) != 1 || len(batches[0]) != 3 {
		t.Fatalf("Keys delivered %v, want one frame of three presses", batches)
	}
}

// The pointer helpers deliver what Gio's gestures see: a double click is
// one click then a NumClicks-2 click, a modifier click carries its keys, a
// right click its button.
func TestPointerVariants(t *testing.T) {
	var g gesture.Click
	var clicks []gesture.ClickEvent
	h := harness(t, func(gtx C) {
		for {
			e, ok := g.Update(gtx.Source)
			if !ok {
				break
			}
			if e.Kind == gesture.KindClick {
				clicks = append(clicks, e)
			}
		}
		defer clip.Rect{Max: image.Pt(100, 40)}.Push(gtx.Ops).Pop()
		g.Add(gtx.Ops)
		kit.TagLabel(gtx, "target", "Target")
	})
	h.DoubleClickID("target")
	if len(clicks) != 2 || clicks[1].NumClicks != 2 {
		t.Fatalf("double click: %+v", clicks)
	}
	// Two separate clicks are never a double click, however fast the
	// machine runs them: event times come from the harness's clock.
	clicks = nil
	h.ClickID("target")
	h.ClickID("target")
	if len(clicks) != 2 || clicks[0].NumClicks != 1 || clicks[1].NumClicks != 1 {
		t.Fatalf("two clicks: %+v", clicks)
	}
	clicks = nil
	h.ClickIDMods("target", key.ModShift)
	if len(clicks) != 1 || clicks[0].Modifiers != key.ModShift {
		t.Fatalf("shift click: %+v", clicks)
	}
	clicks = nil
	h.RightClickID("target") // gesture.Click takes the primary button only
	if len(clicks) != 0 {
		t.Fatalf("right click reached a primary-only gesture: %+v", clicks)
	}
}

// Tests can address controls as a user perceives them: role and label.
func TestByRole(t *testing.T) {
	var save, cancel kit.Clickable
	saved := false
	h := harness(t, func(gtx C) {
		if save.Clicked(gtx) {
			saved = true
		}
		cancel.Clicked(gtx)
		kit.Row(gtx,
			kit.Rigid(func(gtx C) D { return ui.Button{ID: "dlg/ok", Text: "Save"}.Layout(gtx, th, &save) }),
			kit.Rigid(func(gtx C) D { return ui.Button{ID: "dlg/cancel", Text: "Cancel"}.Layout(gtx, th, &cancel) }),
		)
	})
	if id := h.ByRole(kit.RoleButton, "Cancel"); id != "dlg/cancel" {
		t.Fatalf("ByRole: %q", id)
	}
	h.ClickRole(kit.RoleButton, "Save")
	if !saved {
		t.Fatal("ClickRole did not click Save")
	}
	msg := func() (msg string) {
		defer func() { msg = fmt.Sprint(recover()) }()
		h.ByRole(kit.RoleButton, "Delete")
		return ""
	}()
	if !strings.Contains(msg, `no Button labelled "Delete"`) {
		t.Fatalf("missing control: %q", msg)
	}
}

// With h.App(), Spawn runs off the UI goroutine: the test sees the loading
// state, then RunTasks delivers the result.
func TestRunTasks(t *testing.T) {
	var load kit.Clickable
	status := "idle"
	release := make(chan struct{})
	var h *Harness
	h = harness(t, func(gtx C) {
		if load.Clicked(gtx) {
			status = "loading"
			kit.Spawn(h.App(), func(context.Context) (string, error) { <-release; return "done", nil },
				func(v string, _ error) { status = v })
		}
		kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return ui.Button{ID: "load", Text: "Load"}.Layout(gtx, th, &load) }),
			kit.Rigid(func(gtx C) D { return th.Text(gtx, status, th.Type.Body, th.Fg, kit.Regular) }),
		)
	})
	h.ClickID("load")
	if status != "loading" {
		t.Fatalf("before the task finishes: %q", status)
	}
	close(release)
	h.RunTasks()
	if status != "done" {
		t.Fatalf("after RunTasks: %q", status)
	}
	if _, ok := h.FindText("done"); !ok {
		t.Fatal("the result was not drawn")
	}
}

// Bug: RunTasks waited for the running tasks once, then settled. A done
// callback that started follow-up work (load, then load details) left that
// work running on return, and the test saw a half-loaded screen. RunTasks
// drains chains of tasks; a cancelled task's done never runs.
func TestRunTasksChainsAndCancels(t *testing.T) {
	status := "idle"
	h := harness(t, func(gtx C) { th.Text(gtx, status, th.Type.Body, th.Fg, kit.Regular) })
	// Started from the test goroutine, which owns the UI here.
	kit.Spawn(h.App(), func(context.Context) (string, error) { return "list", nil },
		func(v string, _ error) {
			status = v
			kit.Spawn(h.App(), func(context.Context) (string, error) {
				time.Sleep(20 * time.Millisecond)
				return "details", nil
			}, func(v string, _ error) { status = v })
		})
	cancelledDone := false
	cancelled := kit.Spawn(h.App(), func(ctx context.Context) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}, func(string, error) { cancelledDone = true })
	cancelled.Cancel()
	h.RunTasks()
	if status != "details" {
		t.Fatalf("after RunTasks: %q, want the follow-up task's result", status)
	}
	if cancelledDone {
		t.Fatal("a cancelled task's done ran")
	}
	if n := h.App().Pending(); n != 0 {
		t.Fatalf("%d task(s) still running after RunTasks", n)
	}
	if _, ok := h.FindText("details"); !ok {
		t.Fatal("the follow-up result was not drawn")
	}
}

// Bug: a posted callback that posted a follow-up left it queued after the
// settle, since nothing requested a redraw. A window draws a frame for every
// Post; the harness drew none, so RunTasks returned early and the strict
// check's extra frame ran the follow-up and reported a stale frame.
func TestPostedCallbacksRunInTheSettle(t *testing.T) {
	status := "idle"
	h := harness(t, func(gtx C) { th.Text(gtx, status, th.Type.Body, th.Fg, kit.Regular) })
	h.App().Post(func() {
		status = "first"
		h.App().Post(func() { status = "follow-up" })
	})
	h.RunTasks()
	if status != "follow-up" {
		t.Fatalf("after RunTasks: %q, want the follow-up callback's result", status)
	}
	if n := h.App().Queued(); n != 0 {
		t.Fatalf("%d callback(s) still queued after RunTasks", n)
	}
	if _, ok := h.FindText("follow-up"); !ok {
		t.Fatal("the follow-up result was not drawn")
	}
	if p := h.Problems(); len(p) != 0 {
		t.Fatalf("problems: %v", p)
	}
}

// The tree walkers assume Router.AppendSemantics returns each node once, root
// first, with every other node reachable through Children. Stock and forked
// Gio must both keep that shape, or every query silently misses nodes.
func TestSemanticTreeShape(t *testing.T) {
	var a, b kit.Clickable
	h := harness(t, func(gtx C) {
		kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return ui.Button{ID: "a", Text: "A"}.Layout(gtx, th, &a) }),
			kit.Rigid(func(gtx C) D {
				return th.Press(gtx, &b, "b", func(gtx C, _ kit.State) D {
					defer clip.Rect{Max: image.Pt(20, 10)}.Push(gtx.Ops).Pop()
					kit.TagLabel(gtx, "b/inner", "Inner")
					return box(gtx, 40, 20)
				})
			}),
		)
	})
	all := h.router.AppendSemantics(nil)
	walked := h.nodes()
	if len(all) == 0 || len(walked) != len(all) {
		t.Fatalf("walk from the root reached %d nodes, AppendSemantics returned %d", len(walked), len(all))
	}
	ids := map[input.SemanticID]bool{}
	for _, n := range walked {
		if ids[n.ID] {
			t.Fatalf("node %d visited twice", n.ID)
		}
		ids[n.ID] = true
	}
	for _, n := range all {
		if !ids[n.ID] {
			t.Fatalf("node %d (%q) unreachable from the root", n.ID, n.Desc.Description)
		}
	}
	tree := h.semantics()
	inner, _ := h.node(func(d input.SemanticDesc) bool { return d.Description == "b/inner" })
	outer, _ := h.node(func(d input.SemanticDesc) bool { return d.Description == "b" })
	if !tree.within(inner.ID, outer.ID) || tree.within(outer.ID, inner.ID) {
		t.Fatalf("ancestry of b/inner under b is wrong: %+v", inner)
	}
}

// Bug: RunTasks checked its deadline only while spawned work was running, so
// a posted callback that re-posted itself kept it settling forever.
func TestRunTasksStopsSelfRepostingCallback(t *testing.T) {
	h := harness(t, func(gtx C) {})
	h.Strict = false
	h.tasksTimeout = 50 * time.Millisecond
	var repost func()
	repost = func() { h.App().Post(repost) }
	h.App().Post(repost)
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		h.RunTasks()
	}()
	select {
	case r := <-done:
		if r == nil || !strings.Contains(fmt.Sprint(r), "queued") {
			t.Fatalf("RunTasks returned without a fatal failure: %v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunTasks did not stop at its deadline")
	}
}
