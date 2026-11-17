// Package kittest drives a giokit UI headlessly: real layout, real input
// routing, real GPU rendering (EGL surfaceless), no window or display.
//
// Typical test:
//
//	h := kittest.NewT(t, 1100, 720, v.Frame) // skips if no GPU (KITTEST_REQUIRE_GPU=1 fails)
//	h.Theme = v.th                          // enables glyph checks
//	h.ClickID("save")                       // input → settle → stale check
//	h.ExpectTextIn("row:42", "Saved")
//	h.ExpectClean()                         // audit: ids, targets, occlusion, glyphs
package kittest

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/hhhapz/giokit/kit"
)

type Harness struct {
	Size   image.Point // pixels
	Metric unit.Metric
	Frames int // total frames drawn
	// Now is the virtual clock handed to every frame (gtx.Now). Settle advances
	// it by FrameStep while the UI asks for immediate redraws (animations), so
	// animations run to completion deterministically. Wait advances it too.
	Now       time.Time
	FrameStep time.Duration

	// Strict re-renders after every input and fails if anything changed, i.e.
	// the settled frame was stale (draw-before-handle, late hover reads,
	// missing Notify). On by default.
	Strict bool
	// Theme enables the missing-glyph audit.
	Theme *kit.Theme
	// MinTarget is the smallest clickable size the audit accepts (default 16dp).
	MinTarget unit.Dp
	// MinContrast is the smallest text contrast ratio the audit accepts
	// (WCAG AA: 4.5), MinContrastLarge the same for large text (AA: 3).
	// 0 turns the check off.
	MinContrast, MinContrastLarge float64

	tb       TB
	draw     func(gtx layout.Context)
	router   input.Router
	win      *headless.Window
	kwin     kit.Window // per-frame dispatch tree, as kit.App keeps one
	ops      op.Ops
	cursor   f32.Point
	problems []Problem
	snaps    []snap
	wake     time.Time
	wakeOK   bool
}

// New creates a harness of wdp×hdp dp at the given scale and settles the UI.
func New(wdp, hdp int, scale float32, draw func(gtx layout.Context)) (*Harness, error) {
	size := image.Pt(int(float32(wdp)*scale), int(float32(hdp)*scale))
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		return nil, fmt.Errorf("headless GPU unavailable: %w", err)
	}
	h := &Harness{Size: size, Metric: unit.Metric{PxPerDp: scale, PxPerSp: scale}, draw: draw, win: win,
		Strict: true, MinTarget: 16, MinContrast: 4.5, MinContrastLarge: 3, Now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), FrameStep: time.Second / 60}
	h.Settle()
	return h, nil
}

// TB is the subset of testing.TB the harness uses. It is declared here so
// that binaries using New for screenshots (-shot) do not link package testing.
type TB interface {
	Helper()
	Name() string
	Error(args ...any)
	Errorf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Skip(args ...any)
	Cleanup(func())
}

// RequireGPU reports whether a missing headless GPU must fail tests instead
// of skipping them. Set KITTEST_REQUIRE_GPU=1 in CI, or a runner without EGL
// reports ok having run nothing.
func RequireGPU() bool {
	v := os.Getenv("KITTEST_REQUIRE_GPU")
	return v != "" && v != "0"
}

// NoGPU skips t, or fails it when RequireGPU is set. Harness constructors call
// it when New returns an error.
func NoGPU(t TB, err error) {
	t.Helper()
	if RequireGPU() {
		t.Fatal(err)
	}
	t.Skip(err)
}

// NewT is New for tests at scale 1: skips without a GPU (fails under
// KITTEST_REQUIRE_GPU), releases on cleanup, reports problems through t, and
// saves the final frame to $KITTEST_SHOTS/<test>.png.
func NewT(t TB, wdp, hdp int, draw func(gtx layout.Context)) *Harness {
	t.Helper()
	return NewTAt(t, wdp, hdp, 1, draw)
}

// NewTAt is NewT at a given scale (pixels per dp): 1.5 and 2 catch hairlines,
// rings and targets that only break on HiDPI screens.
func NewTAt(t TB, wdp, hdp int, scale float32, draw func(gtx layout.Context)) *Harness {
	t.Helper()
	h, err := New(wdp, hdp, scale, draw)
	if err != nil {
		NoGPU(t, err)
	}
	h.tb = t
	t.Cleanup(func() {
		if dir := os.Getenv("KITTEST_SHOTS"); dir != "" {
			os.MkdirAll(dir, 0o755)
			h.Save(filepath.Join(dir, sanitize(t.Name())+".png"))
		}
		h.Release()
	})
	return h
}

func (h *Harness) Release() { h.win.Release() }

func (h *Harness) frame() {
	h.ops.Reset()
	gtx := layout.Context{
		Ops:         &h.ops,
		Constraints: layout.Exact(h.Size),
		Metric:      h.Metric,
		Now:         h.Now,
		Source:      h.router.Source(),
	}
	gtx = h.kwin.Begin(gtx)
	h.draw(gtx)
	h.kwin.End(gtx)
	h.router.Frame(&h.ops)
	if err := h.win.Frame(&h.ops); err != nil {
		panic(err)
	}
	h.Frames++
	// Consume the redraw request now, as a real window does after each frame.
	// A stale one left behind would make the next key look "handled" (Tab
	// would stop moving focus whenever a spinner had scheduled a frame).
	h.wake, h.wakeOK = h.router.WakeupTime()
}

// maxSettle bounds how long (in virtual frames) a UI may keep requesting
// immediate redraws: 2s at 60 Hz, far longer than any finite animation.
const maxSettle = 120

// Settle draws one frame, then keeps drawing while the UI requests an immediate
// redraw, exactly like a real window, advancing the virtual clock one FrameStep
// per extra frame. It never invents input, so "one click and the screen is
// right" is a real assertion. Redraws scheduled for later (caret blink,
// continuous animations via kit.Loop, delays) are not chased; use Wait.
// Returns frames drawn.
func (h *Harness) Settle() int {
	for n := 1; ; n++ {
		h.frame()
		t, ok := h.wake, h.wakeOK
		if !ok || t.After(h.Now) {
			return n
		}
		if n >= maxSettle {
			h.report(Problem{Kind: "no-settle", Msg: fmt.Sprintf("UI still requesting immediate redraws after %d frames (Notify in every frame? continuous animations must use kit.Loop)", n)})
			return n
		}
		h.Now = h.Now.Add(h.FrameStep)
	}
}

// Wait advances the virtual clock by d and settles: use it for hover delays
// (tooltips) or to step a continuous animation.
func (h *Harness) Wait(d time.Duration) {
	h.Now = h.Now.Add(d)
	h.after(fmt.Sprintf("wait %v", d))
}

// after runs post-input checks.
func (h *Harness) after(action string) {
	h.Settle()
	if h.Strict {
		if diff := h.Stale(); diff != "" {
			h.report(Problem{Kind: "stale-frame", Msg: fmt.Sprintf("after %s the settled frame was stale: %s", action, diff)})
		}
	}
}

// ---- pointer input ----

// Move sends a pointer move to p (pixels).
func (h *Harness) Move(p image.Point) {
	h.cursor = f32.Pt(float32(p.X), float32(p.Y))
	h.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: h.cursor, Time: since()})
	h.after(fmt.Sprintf("move to %v", p))
}

// Click moves to p, then presses and releases the primary button.
func (h *Harness) Click(p image.Point) {
	h.Move(p)
	h.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: h.cursor, Time: since()},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: h.cursor, Time: since()},
	)
	h.after(fmt.Sprintf("click at %v", p))
}

// RightClick moves to p, then presses and releases the secondary button.
func (h *Harness) RightClick(p image.Point) {
	h.Move(p)
	h.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonSecondary, Position: h.cursor, Time: since()},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: h.cursor, Time: since()},
	)
	h.after(fmt.Sprintf("right-click at %v", p))
}

// Scroll scrolls by dy pixels (positive = down) with the pointer at p.
func (h *Harness) Scroll(p image.Point, dy float32) {
	h.Move(p)
	h.router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: h.cursor, Scroll: f32.Pt(0, dy), Time: since()})
	h.after(fmt.Sprintf("scroll %v at %v", dy, p))
}

// Drag presses at from, moves to to in a few steps, and releases there.
func (h *Harness) Drag(from, to image.Point) {
	h.Move(from)
	h.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: h.cursor, Time: since()})
	h.Settle()
	const steps = 4
	for i := 1; i <= steps; i++ {
		p := from.Add(to.Sub(from).Mul(i).Div(steps))
		h.cursor = f32.Pt(float32(p.X), float32(p.Y))
		h.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: h.cursor, Time: since()})
		h.Settle()
	}
	h.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: h.cursor, Time: since()})
	h.after(fmt.Sprintf("drag %v→%v", from, to))
}

// DragID drags the widget tagged id by (dx, dy) pixels from its center.
func (h *Harness) DragID(id string, dx, dy int) {
	c := Center(h.must(id))
	h.Drag(c, c.Add(image.Pt(dx, dy)))
}

// ScrollID scrolls the region tagged id by dy pixels.
func (h *Harness) ScrollID(id string, dy float32) { h.Scroll(Center(h.must(id)), dy) }

// Hover moves to the center of the widget tagged id.
func (h *Harness) Hover(id string) { h.Move(Center(h.must(id))) }

// ClickID clicks the center of the widget tagged id.
func (h *Harness) ClickID(id string) { h.Click(Center(h.must(id))) }

// ---- keyboard input ----

// Key presses and releases a key. Unhandled Tab / Shift+Tab move focus, as in a real window.
func (h *Harness) Key(name key.Name, mods key.Modifiers) {
	press := key.Event{Name: name, Modifiers: mods, State: key.Press}
	dir := key.FocusDirection(-1)
	switch {
	case name == key.NameTab && mods == 0:
		dir = key.FocusForward
	case name == key.NameTab && mods == key.ModShift:
		dir = key.FocusBackward
	}
	if dir != -1 {
		h.router.Queue(input.SystemEvent{Event: press})
		if _, handled := h.router.WakeupTime(); !handled {
			h.router.MoveFocus(dir)
		}
	} else {
		h.router.Queue(press)
	}
	h.router.Queue(key.Event{Name: name, Modifiers: mods, State: key.Release})
	h.after(fmt.Sprintf("key %v%v", mods, name))
}

// Chord presses a chord written like a kit.Keymap binding ("Ctrl+1",
// "Shortcut+K", "Escape").
func (h *Harness) Chord(chord string) {
	n, m, err := kit.ParseChord(chord)
	if err != nil {
		h.fatal(err.Error())
	}
	h.Key(n, m)
}

// Tab moves focus forward (or backward with back=true).
func (h *Harness) Tab(back bool) {
	if back {
		h.Key(key.NameTab, key.ModShift)
	} else {
		h.Key(key.NameTab, 0)
	}
}

// Type inserts text into the focused editor, replacing its selection.
func (h *Harness) Type(s string) {
	// As app.Window's EditorInsert does: replace the selection, then put the
	// caret after the inserted text (without the second event the caret
	// stayed in front of text typed mid-field).
	sel := h.router.EditorState().Selection.Range
	start := min(sel.Start, sel.End) + utf8.RuneCountInString(s)
	h.router.Queue(
		key.EditEvent{Range: sel, Text: s},
		key.SelectionEvent{Start: start, End: start},
	)
	h.after(fmt.Sprintf("type %q", s))
}

// Cursor is the pointer cursor the UI requests at the current pointer position.
func (h *Harness) Cursor() pointer.Cursor { return h.router.Cursor() }

// ---- queries ----

// Find returns the pixel bounds of the widget tagged with semantic id.
func (h *Harness) Find(id string) (image.Rectangle, bool) {
	n, ok := h.node(func(d input.SemanticDesc) bool { return d.Description == id })
	return n.Desc.Bounds, ok
}

// Disabled reports whether the widget tagged id is disabled (kit's
// Theme.Disable), itself or through an ancestor. It fails the test if the id
// isn't on screen.
func (h *Harness) Disabled(id string) bool {
	h.must(id)
	byID := map[input.SemanticID]input.SemanticNode{}
	var target input.SemanticNode
	for _, n := range h.nodes() {
		byID[n.ID] = n
		if n.Desc.Description == id {
			target = n
		}
	}
	for n, ok := target, true; ok; n, ok = byID[n.ParentID] {
		if n.Desc.Disabled {
			return true
		}
		if n.ParentID == 0 {
			break
		}
	}
	return false
}

// focusedTagID returns the semantic id of the widget with keyboard focus, or ""
// when nothing (or nothing with an id) has it: the widget whose bounds
// match the focused handler's area, or else the smallest one containing it.
func (h *Harness) focusedTagID() string {
	src := h.router.Source()
	tag := src.FocusedTag()
	if tag == nil {
		return ""
	}
	b, ok := src.Bounds(tag)
	if !ok {
		return ""
	}
	// Nodes come parents first, so the last exact match is the innermost:
	// a group exactly the size of its only control names the control.
	exact, best, area := "", "", -1
	for _, n := range h.nodes() {
		d := n.Desc
		if d.Description == "" {
			continue
		}
		if d.Bounds == b {
			exact = d.Description
			continue
		}
		if b.In(d.Bounds) {
			if a := d.Bounds.Dx() * d.Bounds.Dy(); area < 0 || a < area {
				best, area = d.Description, a
			}
		}
	}
	if exact != "" {
		return exact
	}
	return best
}

// FocusedID is the id of the focused widget. When focus is on a focus group
// (a list, tree, table or menu), it is the group's active item if that item
// is laid out, as a screen reader would announce it; otherwise the group.
func (h *Harness) FocusedID() string {
	id := h.focusedTagID()
	if a := h.kwin.GroupActive()[id]; a != "" {
		return a
	}
	return id
}

// FindText returns the bounds of the first text label exactly equal to s.
func (h *Harness) FindText(s string) (image.Rectangle, bool) {
	n, ok := h.node(func(d input.SemanticDesc) bool { return d.Label == s })
	return n.Desc.Bounds, ok
}

// TextsIn returns the labels whose origin lies inside the widget tagged id
// (labels report unbounded heights, so containment would miss them).
func (h *Harness) TextsIn(id string) []string {
	r := h.must(id)
	var out []string
	for _, n := range h.nodes() {
		if n.Desc.Label != "" && n.Desc.Bounds.Min.In(r) {
			out = append(out, n.Desc.Label)
		}
	}
	return out
}

// Texts returns every label on screen, in draw order.
func (h *Harness) Texts() []string {
	var out []string
	for _, n := range h.nodes() {
		if n.Desc.Label != "" {
			out = append(out, n.Desc.Label)
		}
	}
	return out
}

// IDs returns every semantic id on screen, in draw order.
func (h *Harness) IDs() []string {
	var out []string
	for _, n := range h.nodes() {
		if n.Desc.Description != "" {
			out = append(out, n.Desc.Description)
		}
	}
	return out
}

func (h *Harness) must(id string) image.Rectangle {
	r, ok := h.Find(id)
	if !ok {
		h.fatal(fmt.Sprintf("no widget %q on screen; ids: %v", id, h.IDs()))
	}
	return r
}

// nodes flattens the semantic tree (pre-order).
func (h *Harness) nodes() []input.SemanticNode {
	var out []input.SemanticNode
	var walk func([]input.SemanticNode)
	walk = func(ns []input.SemanticNode) {
		for _, n := range ns {
			out = append(out, n)
			walk(n.Children)
		}
	}
	all := h.router.AppendSemantics(nil)
	if len(all) > 0 {
		walk(all[:1]) // the root; the rest of the slice repeats its descendants
	}
	return out
}

func (h *Harness) node(match func(input.SemanticDesc) bool) (input.SemanticNode, bool) {
	for _, n := range h.nodes() {
		if match(n.Desc) {
			return n, true
		}
	}
	return input.SemanticNode{}, false
}

// ---- pixels ----

// Image returns the current frame.
func (h *Harness) Image() *image.RGBA {
	img := image.NewRGBA(image.Rectangle{Max: h.Size})
	if err := h.win.Screenshot(img); err != nil {
		panic(err)
	}
	return img
}

// Pixel samples the current frame at p.
func (h *Harness) Pixel(p image.Point) color.NRGBA {
	c := h.Image().RGBAAt(p.X, p.Y)
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// BgColor samples a widget's background 3px inside its left edge at mid
// height, where content rarely is. Use it to check hover/selection fills.
func (h *Harness) BgColor(id string) color.NRGBA {
	r := h.must(id)
	return h.Pixel(image.Pt(r.Min.X+3, Center(r).Y))
}

// Save writes the current frame as a PNG.
func (h *Harness) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, h.Image())
}

func Center(r image.Rectangle) image.Point { return r.Min.Add(r.Size().Div(2)) }

// Near reports whether two colors match within tol per channel (GPU rounding).
func Near(a, b color.NRGBA, tol uint8) bool {
	d := func(x, y uint8) bool { return max(x, y)-min(x, y) <= tol }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && d(a.A, b.A)
}

var start = time.Now()

func since() time.Duration { return time.Since(start) }

func sanitize(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c == '/' || c == ' ' {
			b[i] = '_'
		}
	}
	return string(b)
}
