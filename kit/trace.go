package kit

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	// traceBufferSize holds a busy frame's trace lines between flushes.
	traceBufferSize = 64 << 10
	// traceFlushInterval flushes lines written between frames (input,
	// idle), so they reach the log while no frames are drawn.
	traceFlushInterval = 250 * time.Millisecond
)

// traceEnabled is GIOKIT_TRACE: debug logging of frame pacing and list
// state. It is read once, at startup.
var traceEnabled = os.Getenv("GIOKIT_TRACE") != ""

// traceLog is the GIOKIT_TRACE output, shared by every window and
// goroutine: one buffered writer, flushed each frame so that tracing does
// not itself cause stutter. mu guards every field but start.
type traceLog struct {
	mu     sync.Mutex
	w      *bufio.Writer // nil when tracing is off or writing failed
	start  time.Time     // origin of each line's millisecond timestamp
	frame  int           // frames begun so far
	failed bool          // a write failed; tracing stopped
}

// trc is the process's trace log. Tracing is a process-wide diagnostic
// switched on by the environment, so its state is global by design.
var trc = traceLog{start: time.Now(), w: openTrace()}

// openTrace opens the trace output: GIOKIT_TRACE_FILE if set, otherwise
// stderr; nil without GIOKIT_TRACE. A file that cannot be created is
// reported on stderr, and the trace goes to stderr instead.
func openTrace() *bufio.Writer {
	if !traceEnabled {
		return nil
	}
	var out io.Writer = os.Stderr
	if p := os.Getenv("GIOKIT_TRACE_FILE"); p != "" {
		f, err := os.Create(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "giokit: GIOKIT_TRACE_FILE: %v; tracing to stderr\n", err)
		} else {
			out = f
		}
	}
	return bufio.NewWriterSize(out, traceBufferSize)
}

// TraceEnabled reports whether GIOKIT_TRACE is set.
func TraceEnabled() bool { return traceEnabled }

// Tracef writes one GIOKIT_TRACE line; it is a no-op without GIOKIT_TRACE.
// Every line reads
//
//	giokit <ms since start> f<frame> <kind> key=value ...
//
// kv alternates keys and values; a trailing key without a value is dropped.
// float32 values print with 2 decimals, float64 with 3. kit/forktrace
// routes the Gio fork's window events (wl_*, scroll_flush, gpu_frame,
// present) here, so one log interleaves them with kit's frame pacing and
// list state, in order. Safe from any goroutine.
func Tracef(kind string, kv ...any) { tracef(kind, kv...) }

// tracef writes one trace line; see Tracef.
func tracef(kind string, kv ...any) {
	if !traceEnabled {
		return
	}
	var b strings.Builder
	trc.mu.Lock()
	defer trc.mu.Unlock()
	if trc.w == nil {
		return
	}
	fmt.Fprintf(&b, "giokit %10.3f f%d %s", ms(time.Since(trc.start)), trc.frame, kind)
	for i := 0; i+1 < len(kv); i += 2 {
		v := kv[i+1]
		switch x := v.(type) {
		case float32:
			v = fmt.Sprintf("%.2f", x)
		case float64:
			v = fmt.Sprintf("%.3f", x)
		}
		fmt.Fprintf(&b, " %v=%v", kv[i], v)
	}
	b.WriteByte('\n')
	_, err := trc.w.WriteString(b.String())
	trc.check(err)
}

// traceFrame advances the frame counter and flushes the previous frame's
// lines.
func traceFrame() {
	if !traceEnabled {
		return
	}
	trc.mu.Lock()
	defer trc.mu.Unlock()
	trc.frame++
	trc.flush()
}

// traceFlush writes out buffered trace lines.
func traceFlush() {
	if !traceEnabled {
		return
	}
	trc.mu.Lock()
	defer trc.mu.Unlock()
	trc.flush()
}

// flushTraceEvery flushes the trace every d until stop is called.
func flushTraceEvery(d time.Duration) (stop func()) {
	t := time.NewTicker(d)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-t.C:
				traceFlush()
			case <-done:
				return
			}
		}
	}()
	return func() {
		t.Stop()
		close(done)
	}
}

// flush writes out the buffer. l.mu must be held.
func (l *traceLog) flush() {
	if l.w != nil {
		l.check(l.w.Flush())
	}
}

// check stops tracing after the first failed write, reporting it once on
// stderr: a broken trace output must not slow or break the app. l.mu must
// be held.
func (l *traceLog) check(err error) {
	if err == nil || l.failed {
		return
	}
	l.failed, l.w = true, nil
	fmt.Fprintf(os.Stderr, "giokit: trace output failed, tracing stopped: %v\n", err)
}

// ms converts d to fractional milliseconds for trace lines.
func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
