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
//
// Input methods (Click, Key, Type...) settle the UI and, in strict mode,
// re-render once more to catch a stale frame. Queries (Find, Texts, Role)
// read the last frame's semantic tree and pixels. The Expect methods
// report through the test and need a harness from NewT; on a standalone
// harness from New they panic. Package kittest/conform builds the
// per-control conformance suite on top of it.
package kittest

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"

	"github.com/hhhapz/giokit/gio/f32"
	"github.com/hhhapz/giokit/gio/gpu/headless"
	"github.com/hhhapz/giokit/gio/io/input"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/unit"

	"github.com/hhhapz/giokit/kit"
)

// Harness runs one UI headlessly. Make it with New (standalone: problems are
// recorded for Problems and fatal failures panic) or NewT (attached to a
// test: problems fail the test and fatal failures stop it). Every input
// method settles the UI and, in strict mode, checks the settled frame. The
// harness, like the UI it drives, belongs to one goroutine.
type Harness struct {
	Size   image.Point // window size in pixels
	Metric unit.Metric // the window's scale: PxPerDp and PxPerSp
	Frames int         // total frames drawn
	// Now is the virtual clock handed to every frame (gtx.Now); it starts at
	// 2026-01-01 00:00 UTC. Settle advances it by FrameStep (default 1/60 s)
	// while the UI asks for immediate redraws (animations), so animations run
	// to completion deterministically. Wait advances it too.
	Now       time.Time
	FrameStep time.Duration // the clock step per settled frame

	// Strict re-renders after every input and fails if anything changed, i.e.
	// the settled frame was stale (draw-before-handle, late hover reads,
	// missing Notify). On by default.
	Strict bool
	// Theme enables the missing-glyph audit and ink recognition in the
	// contrast audit. Nil by default.
	Theme *kit.Theme
	// MinTarget is the smallest clickable size the audit accepts (default 16dp).
	MinTarget unit.Dp
	// MinContrast is the smallest text contrast ratio the audit accepts
	// (default WCAG AA: 4.5), MinContrastLarge the same for large text
	// (default 3). 0 turns the check off.
	MinContrast, MinContrastLarge float64

	app       *kit.App // h.App(), drained before each frame
	tb        TB       // the test, or standalone{}
	draw      func(gtx layout.Context)
	router    input.Router
	win       *headless.Window
	kwin      kit.Window // per-frame dispatch tree, as kit.App keeps one
	ops       op.Ops
	cursor    f32.Point
	inputTime time.Duration // time of the last pointer event; see gestureStart
	problems  []Problem
	snaps     []snap
	wake      time.Time
	wakeOK    bool
	// tasksTimeout overrides runTasksTimeout when positive (tests).
	tasksTimeout time.Duration
}

// Harness defaults; see the Harness fields.
const (
	defaultMinTarget        unit.Dp = 16
	defaultMinContrast              = 4.5
	defaultMinContrastLarge         = 3
	defaultFrameStep                = time.Second / 60
)

// epoch is where every harness's virtual clock starts.
var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// New creates a standalone harness of wdp×hdp dp at the given scale (pixels
// per dp) and settles the UI. Problems found while settling, and later, are
// recorded for Problems; a fatal failure panics. It fails only when no
// headless GPU is available. Call Release when done.
func New(wdp, hdp int, scale float32, draw func(gtx layout.Context)) (*Harness, error) {
	h, err := newHarness(wdp, hdp, scale, draw, standalone{})
	if err != nil {
		return nil, err
	}
	h.Settle()
	return h, nil
}

// newHarness creates a harness reporting to tb without drawing a frame.
func newHarness(wdp, hdp int, scale float32, draw func(gtx layout.Context), tb TB) (*Harness, error) {
	size := image.Pt(int(float32(wdp)*scale), int(float32(hdp)*scale))
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		return nil, fmt.Errorf("headless GPU unavailable: %w", err)
	}
	return &Harness{
		Size:             size,
		Metric:           unit.Metric{PxPerDp: scale, PxPerSp: scale},
		Now:              epoch,
		FrameStep:        defaultFrameStep,
		Strict:           true,
		MinTarget:        defaultMinTarget,
		MinContrast:      defaultMinContrast,
		MinContrastLarge: defaultMinContrastLarge,
		tb:               tb,
		draw:             draw,
		win:              win,
	}, nil
}

// NewT is NewTAt at scale 1.
func NewT(t TB, wdp, hdp int, draw func(gtx layout.Context)) *Harness {
	t.Helper()
	return NewTAt(t, wdp, hdp, 1, draw)
}

// NewTAt creates a harness attached to t at the given scale (pixels per dp)
// and settles the UI. Every problem, from the first frame on, fails t. It
// skips t without a GPU (fails it under KITTEST_REQUIRE_GPU). On cleanup it
// saves the final frame to $KITTEST_SHOTS/<ShotName(test)>.png when that
// variable is set, then releases the harness. Scales 1.5 and 2 catch
// hairlines, rings and targets that only break on HiDPI screens.
func NewTAt(t TB, wdp, hdp int, scale float32, draw func(gtx layout.Context)) *Harness {
	t.Helper()
	h, err := newHarness(wdp, hdp, scale, draw, t)
	if err != nil {
		NoGPU(t, err)
		return nil
	}
	t.Cleanup(func() {
		defer h.Release()
		if dir := os.Getenv("KITTEST_SHOTS"); dir != "" {
			if err := h.saveShot(dir, t.Name()); err != nil {
				t.Errorf("kittest: save final frame: %v", err)
			}
		}
	})
	h.Settle()
	return h
}

// saveShot writes the current frame to dir/<ShotName(test)>.png.
func (h *Harness) saveShot(dir, test string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return h.Save(filepath.Join(dir, ShotName(test)+".png"))
}

// ShotName turns a test name into the file name NewTAt saves its final
// frame under: every byte other than an ASCII letter, digit, '-', '_' or '.'
// becomes '_', so a subtest name ("Gallery/dark theme") stays one flat,
// portable file name.
func ShotName(test string) string {
	b := []byte(test)
	for i, c := range b {
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', c == '-', c == '_', c == '.':
		default:
			b[i] = '_'
		}
	}
	return string(b)
}

// Release frees the harness's GPU resources. The harness is unusable after.
func (h *Harness) Release() { h.win.Release() }

// frame draws one frame: posted callbacks, layout, input routing, render.
func (h *Harness) frame() {
	h.tb.Helper()
	if h.app != nil {
		h.app.Drain() // posted callbacks run before the frame, as in kit.App.Run
	}
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
		h.fatal(fmt.Sprintf("kittest: render frame %d: %v", h.Frames+1, err))
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
// redraw or h.App() has queued callbacks (App.Post triggers a frame), exactly
// like a real window, advancing the virtual clock one FrameStep per extra
// frame. It never invents input, so "one click and the screen is
// right" is a real assertion. Redraws scheduled for later (caret blink,
// continuous animations via kit.Loop, delays) are not chased; use Wait.
// It returns the number of frames drawn and reports no-settle after
// maxSettle frames.
func (h *Harness) Settle() int {
	h.tb.Helper()
	for n := 1; ; n++ {
		h.frame()
		t, ok := h.wake, h.wakeOK
		redraw := ok && !t.After(h.Now)
		if !redraw && (h.app == nil || h.app.Queued() == 0) {
			return n
		}
		if n >= maxSettle {
			h.report(Problem{Kind: ProblemNoSettle, Msg: fmt.Sprintf("UI still requesting immediate redraws after %d frames (Notify in every frame? continuous animations must use kit.Loop)", n)})
			return n
		}
		h.Now = h.Now.Add(h.FrameStep)
	}
}

// App is an App for the code under test: pass it to kit.Spawn and Post so
// work runs off the UI goroutine as in a real window, and its callbacks run
// before the harness's next frame. RunTasks waits for the work to finish.
func (h *Harness) App() *kit.App {
	if h.app == nil {
		h.app = kit.NewApp("kittest", "kittest", 0, 0)
	}
	return h.app
}

// runTasksTimeout bounds, in wall time, how long RunTasks waits for work.
const runTasksTimeout = 10 * time.Second

// RunTasks waits for every kit.Spawn started on h.App() to finish and its
// done callback to run and draw, including work those callbacks start in
// turn: it alternates waiting for running work with settling (which runs
// the callbacks) until no work is running, all under one 10s deadline;
// work still running or queued at the deadline (a callback that keeps
// re-posting itself, say) is a fatal failure.
// A cancelled task's done never runs, but RunTasks still waits for its work
// function to return. Callbacks queued with App.Post, from the test or from
// other callbacks, run in the settle (see Settle); RunTasks returns once no
// work is running and nothing is queued. It cannot see a goroutine of the
// app's own that has not posted yet. Before RunTasks, a test sees the UI as
// the user does while work is in flight.
func (h *Harness) RunTasks() {
	h.tb.Helper()
	if h.app == nil {
		return
	}
	timeout := runTasksTimeout
	if h.tasksTimeout > 0 {
		timeout = h.tasksTimeout
	}
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			h.fatal(fmt.Sprintf("RunTasks: %d task(s) running and %d callback(s) queued after %v", h.app.Pending(), h.app.Queued(), timeout))
			return
		}
		for h.app.Pending() > 0 && !time.Now().After(deadline) {
			time.Sleep(time.Millisecond)
		}
		if h.app.Pending() > 0 {
			continue // past the deadline: report at the top of the loop
		}
		h.after("run tasks")
		if h.app.Pending() == 0 && h.app.Queued() == 0 {
			return
		}
	}
}

// Wait advances the virtual clock by d and settles: use it for hover delays
// (tooltips) or to step a continuous animation.
func (h *Harness) Wait(d time.Duration) {
	h.tb.Helper()
	h.Now = h.Now.Add(d)
	h.after(fmt.Sprintf("wait %v", d))
}

// after settles the UI after input, then checks the settled frame in strict
// mode.
func (h *Harness) after(action string) {
	h.tb.Helper()
	h.Settle()
	if h.Strict {
		if diff := h.Stale(); diff != "" {
			h.report(Problem{Kind: ProblemStaleFrame, Msg: fmt.Sprintf("after %s the settled frame was stale: %s", action, diff)})
		}
	}
}
