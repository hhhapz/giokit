package kit

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"
)

// App runs a single-window program. All app state belongs to the UI goroutine
// (GPUI: the main thread owns entities); other goroutines go through Post/Spawn.
type App struct {
	ID, Title     string // ID: Wayland app_id / X11 class; match StartupWMClass in .desktop
	Width, Height unit.Dp
	win           *app.Window

	mu      sync.Mutex
	posts   []func()
	running []func()
}

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

// Drain runs pending posts. Run calls it each frame; harnesses may call it too.
func (a *App) Drain() {
	a.mu.Lock()
	a.posts, a.running = a.running[:0], a.posts
	a.mu.Unlock()
	for i, f := range a.running {
		f()
		a.running[i] = nil
	}
}

// Spawn runs work off the UI goroutine and delivers its result to done on the
// UI goroutine (GPUI: cx.spawn). With a nil App (tests), work runs inline.
func Spawn[T any](a *App, work func(context.Context) (T, error), done func(T, error)) {
	if a == nil {
		done(work(context.Background()))
		return
	}
	go func() {
		v, err := work(context.Background())
		a.Post(func() { done(v, err) })
	}()
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
		app.GiokitTrace = tracef
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
