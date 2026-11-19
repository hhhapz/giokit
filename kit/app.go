package kit

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hhhapz/giokit/gio/app"
	"github.com/hhhapz/giokit/gio/io/system"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/unit"
)

// App runs a single-window program. All app state belongs to the UI goroutine
// (GPUI: the main thread owns entities); other goroutines go through Post and
// Spawn.
//
// Create an App with NewApp: the zero App has no window and is not usable.
// An App runs one window once; set its fields before calling Run.
type App struct {
	// ID is the Wayland app_id and X11 class; match StartupWMClass in the
	// app's .desktop file.
	ID string
	// Title is the window title.
	Title string
	// Width and Height are the initial window size.
	Width, Height unit.Dp
	// OnPanic, if set, receives a panic from a Post callback (Spawn's done
	// included) instead of letting it crash the program; the other queued
	// callbacks still run. Log it and keep going, or show an error.
	OnPanic func(recovered any)

	win      *app.Window
	mu       sync.Mutex
	posts    []func()
	running  []func()
	inflight atomic.Int64 // Spawn work not yet posted back
}

// Pending reports Spawn work still running (not yet posted back to the UI
// goroutine). Harnesses wait for it to reach 0.
func (a *App) Pending() int { return int(a.inflight.Load()) }

// Queued reports callbacks posted with Post (Spawn's done included) that
// have not run yet. A window draws a frame for each; a harness that drives
// frames itself keeps drawing while Queued is above 0. Safe from any
// goroutine.
func (a *App) Queued() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.posts)
}

// NewApp returns an App for a window titled title, of initial size w by h,
// with id as its Wayland app_id and X11 class (see App.ID). The window
// opens when Run is called.
func NewApp(id, title string, w, h unit.Dp) *App {
	return &App{ID: id, Title: title, Width: w, Height: h, win: new(app.Window)}
}

// Post runs f on the UI goroutine before the next frame, and triggers that frame.
// It never blocks, so it is safe from the UI goroutine too (a post made while
// draining runs in the next frame). The queue is unbounded: a producer that
// outruns the frame rate should coalesce its updates itself.
func (a *App) Post(f func()) {
	a.mu.Lock()
	a.posts = append(a.posts, f)
	a.mu.Unlock()
	a.win.Invalidate()
}

// Quit closes the window, which makes Run return. Safe from any goroutine.
func (a *App) Quit() { a.win.Perform(system.ActionClose) }

// Invalidate requests a frame. Safe from any goroutine: wire a
// paint.Stream's OnFrame to it so new video frames get drawn.
func (a *App) Invalidate() { a.win.Invalidate() }

// Drain runs pending posts. Run calls it each frame; harnesses may call it too.
func (a *App) Drain() {
	a.mu.Lock()
	a.posts, a.running = a.running[:0], a.posts
	a.mu.Unlock()
	for i, f := range a.running {
		a.call(f)
		a.running[i] = nil
	}
}

// call runs f, handing a panic to OnPanic when one is set.
func (a *App) call(f func()) {
	if a.OnPanic != nil {
		defer func() {
			if r := recover(); r != nil {
				a.OnPanic(r)
			}
		}()
	}
	f()
}

// Task is a running Spawn. Cancel it when its result is no longer wanted:
// a view closed, or a newer query replaced it (type-ahead):
//
//	v.search.Cancel()
//	v.search = kit.Spawn(app, func(ctx context.Context) ([]Hit, error) { return find(ctx, q) },
//		func(hits []Hit, err error) { v.hits = hits })
//
// Tasks come from Spawn. The zero Task is not running and has no work to
// cancel; Cancel on it does nothing. The UI goroutine owns a Task, like all
// state: Cancel and Running are not safe from the work function.
type Task struct {
	cancel    context.CancelFunc
	cancelled bool
	finished  bool
}

// Cancel cancels the work's context and drops its result: done never runs
// after Cancel. Safe on a nil or finished Task.
func (t *Task) Cancel() {
	if t == nil || t.finished || t.cancel == nil {
		return
	}
	t.cancelled = true
	t.cancel()
}

// Running reports whether the work has neither finished nor been cancelled.
func (t *Task) Running() bool {
	return t != nil && t.cancel != nil && !t.finished && !t.cancelled
}

// Spawn runs work off the UI goroutine and delivers its result to done on the
// UI goroutine (GPUI: cx.spawn), unless the returned Task was cancelled
// first. With a nil App (tests), work runs inline and the Task is finished.
func Spawn[T any](a *App, work func(context.Context) (T, error), done func(T, error)) *Task {
	ctx, cancel := context.WithCancel(context.Background())
	t := &Task{cancel: cancel}
	if a == nil {
		v, err := work(ctx)
		t.finished = true
		cancel()
		done(v, err)
		return t
	}
	a.inflight.Add(1)
	go func() {
		v, err := work(ctx)
		defer a.inflight.Add(-1) // after the Post, so Pending 0 means "drained next"
		a.Post(func() {
			if t.cancelled {
				return
			}
			t.finished = true
			cancel()
			done(v, err)
		})
	}()
	return t
}

// Run opens the window and calls frame for every frame, on the calling
// goroutine, which becomes the UI goroutine. It returns once the window has
// closed (by the user or Quit), with the window's error, or nil after a
// normal close. Run it on its own goroutine and hand the main goroutine to
// Main; when Run returns, save state and exit the process:
//
//	go func() {
//		if err := a.Run(v.Frame); err != nil {
//			log.Fatal(err)
//		}
//		os.Exit(0)
//	}()
//	kit.Main()
//
// Set GIOKIT_TRACE=1 to log frame pacing to stderr: one line per second with
// frame count, interval stats and build time, plus a line for every frame
// that arrived late (> 1.5× the median interval). Use it to diagnose stutter
// the headless harness can't see (compositor pacing, vsync, scaling).
func (a *App) Run(frame func(gtx layout.Context)) error {
	app.ID = a.ID
	if traceEnabled {
		stop := flushTraceEvery(traceFlushInterval)
		defer stop()
	}
	a.win.Option(app.Title(a.Title), app.Size(a.Width, a.Height))
	var ops op.Ops
	var win Window
	var pacing framePacing
	for {
		switch e := a.win.Event().(type) {
		case app.DestroyEvent:
			traceFlush()
			return e.Err
		case app.FrameEvent:
			start := time.Now()
			if traceEnabled {
				traceFrame()
				tracef("frame", "now_ms", ms(e.Now.Sub(trc.start)), "lag_ms", ms(start.Sub(e.Now)),
					"size", fmt.Sprintf("%dx%d", e.Size.X, e.Size.Y), "dpi", e.Metric.PxPerDp)
			}
			a.Drain()
			gtx := win.Begin(app.NewContext(&ops, e))
			frame(gtx)
			win.End(gtx)
			built := time.Now()
			e.Frame(gtx.Ops)
			if traceEnabled {
				tracef("frame_end", "build_ms", ms(built.Sub(start)), "submit_ms", ms(time.Since(built)))
				pacing.frame(e.Now, built.Sub(start))
			}
		}
	}
}

// Main hands the main goroutine to the platform's event loop (Gio's
// app.Main). Call it at the end of main, after starting App.Run on another
// goroutine. It does not return on every platform, so deferred calls in
// main may never run: exit from the goroutine that ran Run.
func Main() { app.Main() }
