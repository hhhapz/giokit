package kit

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/unit"
)

// App runs a single-window program. All app state belongs to the UI goroutine
// (GPUI: the main thread owns entities); other goroutines go through Post/Spawn.
type App struct {
	ID, Title     string // ID: Wayland app_id / X11 class; match StartupWMClass in .desktop
	Width, Height unit.Dp
	win           *app.Window
	// OnPanic, if set, receives a panic from a Post callback (Spawn's done
	// included) instead of letting it crash the program; the other queued
	// callbacks still run. Log it and keep going, or show an error.
	OnPanic func(recovered any)
	// OnClose, if set, runs once the window has closed (by the user or Quit),
	// just before the process exits: save state, close databases. Deferred
	// calls in main never run, since Gio's app.Main does not return on every
	// platform. err is the window's error, if it failed.
	OnClose func(err error)

	mu       sync.Mutex
	posts    []func()
	running  []func()
	inflight atomic.Int64 // Spawn work not yet posted back
}

// Pending reports Spawn work still running (not yet posted back to the UI
// goroutine). Harnesses wait for it to reach 0.
func (a *App) Pending() int { return int(a.inflight.Load()) }

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

// Quit closes the window; OnClose runs, then the process exits. Safe from
// any goroutine.
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
// The UI goroutine owns a Task, like all state.
type Task struct {
	cancel    context.CancelFunc
	cancelled bool
	finished  bool
}

// Cancel cancels the work's context and drops its result: done never runs
// after Cancel. Safe on a nil or finished Task.
func (t *Task) Cancel() {
	if t == nil || t.finished {
		return
	}
	t.cancelled = true
	t.cancel()
}

// Running reports whether the work has neither finished nor been cancelled.
func (t *Task) Running() bool { return t != nil && !t.finished && !t.cancelled }

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

// Run opens the window and calls frame for every frame. It does not return.
//
// Set GIOKIT_TRACE=1 to log frame pacing to stderr: one line per second with
// frame count, interval stats and build time, plus a line for every frame
// that arrived late (> 1.5× the median interval). Use it to diagnose stutter
// the headless harness can't see (compositor pacing, vsync, scaling).
func (a *App) Run(frame func(gtx C)) {
	app.ID = a.ID
	trace := traceEnabled
	var tr tracer
	if trace {
		go func() { // lines between frames (input, idle) still reach the log
			for range time.Tick(250 * time.Millisecond) {
				traceFlush()
			}
		}()
	}
	go func() {
		a.win.Option(app.Title(a.Title), app.Size(a.Width, a.Height))
		var ops op.Ops
		var win Window
		for {
			switch e := a.win.Event().(type) {
			case app.DestroyEvent:
				traceFlush()
				if a.OnClose != nil {
					a.OnClose(e.Err)
				}
				if e.Err != nil {
					fmt.Fprintln(os.Stderr, e.Err)
					os.Exit(1)
				}
				os.Exit(0)
			case app.FrameEvent:
				t0 := time.Now()
				if trace {
					traceFrame()
					tracef("frame", "now_ms", ms(e.Now.Sub(trc.start)), "lag_ms", ms(t0.Sub(e.Now)),
						"size", fmt.Sprintf("%dx%d", e.Size.X, e.Size.Y), "dpi", e.Metric.PxPerDp)
				}
				a.Drain()
				gtx := win.Begin(app.NewContext(&ops, e))
				frame(gtx)
				win.End(gtx)
				t1 := time.Now()
				e.Frame(gtx.Ops)
				if trace {
					tracef("frame_end", "build_ms", ms(t1.Sub(t0)), "submit_ms", ms(time.Since(t1)))
					tr.frame(e.Now, t1.Sub(t0))
				}
			}
		}
	}()
	app.Main()
}

// tracer collects frame timings for GIOKIT_TRACE.
type tracer struct {
	last, window time.Time
	gaps, builds []time.Duration
}

func (t *tracer) frame(now time.Time, build time.Duration) {
	if !t.last.IsZero() {
		gap := now.Sub(t.last)
		if len(t.gaps) > 8 {
			s := append([]time.Duration(nil), t.gaps...)
			sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
			if med := s[len(s)/2]; gap > med*3/2 && gap < time.Second {
				tracef("LATE_FRAME", "gap_ms", ms(gap), "median_ms", ms(med), "build_ms", ms(build))
			}
		}
		if gap < time.Second { // idle periods are not stutter
			t.gaps = append(t.gaps, gap)
		}
	}
	t.builds = append(t.builds, build)
	t.last = now
	if t.window.IsZero() {
		t.window = now
	}
	if now.Sub(t.window) >= time.Second && len(t.gaps) > 0 {
		s := append([]time.Duration(nil), t.gaps...)
		sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
		b := append([]time.Duration(nil), t.builds...)
		sort.Slice(b, func(i, j int) bool { return b[i] < b[j] })
		tracef("pacing", "frames", len(t.builds), "gap_med_ms", ms(s[len(s)/2]), "gap_max_ms", ms(s[len(s)-1]),
			"build_med_ms", ms(b[len(b)/2]), "build_max_ms", ms(b[len(b)-1]))
		t.window, t.gaps, t.builds = now, t.gaps[:0], t.builds[:0]
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
