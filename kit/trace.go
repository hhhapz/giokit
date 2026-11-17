package kit

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// GIOKIT_TRACE output. Every line is
//
//	giokit <ms since start> f<frame> <kind> key=value ...
//
// so one stderr log interleaves frame pacing, per-frame list state and, with
// kit/forktrace imported on giokit's Gio fork, Gio's window events (wl_*,
// scroll_flush, gpu_frame, present), in order. Lines go through one buffered writer that
// is flushed each frame, so tracing doesn't itself cause stutter.
//
// Set GIOKIT_TRACE_FILE=path to write there instead of stderr.

// traceEnabled is GIOKIT_TRACE: debug logging of frame pacing and list skips.
var traceEnabled = os.Getenv("GIOKIT_TRACE") != ""

var trc = struct {
	sync.Mutex
	w     *bufio.Writer
	start time.Time
	frame int
}{start: time.Now()}

func init() {
	if !traceEnabled {
		return
	}
	out := os.Stderr
	if p := os.Getenv("GIOKIT_TRACE_FILE"); p != "" {
		if f, err := os.Create(p); err == nil {
			out = f
		}
	}
	trc.w = bufio.NewWriterSize(out, 1<<16)
}

// TraceEnabled reports GIOKIT_TRACE.
func TraceEnabled() bool { return traceEnabled }

// Tracef writes one GIOKIT_TRACE line (kv alternates keys and values); a
// no-op without GIOKIT_TRACE. kit/forktrace routes the fork's window events
// here.
func Tracef(kind string, kv ...any) { tracef(kind, kv...) }

// tracef writes one trace line; kv alternates keys and values.
func tracef(kind string, kv ...any) {
	if !traceEnabled {
		return
	}
	var b strings.Builder
	trc.Lock()
	defer trc.Unlock()
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
	trc.w.WriteString(b.String())
}

// traceFrame advances the frame counter and flushes the previous frame's lines.
func traceFrame() {
	if !traceEnabled {
		return
	}
	trc.Lock()
	trc.frame++
	trc.w.Flush()
	trc.Unlock()
}

func traceFlush() {
	if !traceEnabled {
		return
	}
	trc.Lock()
	trc.w.Flush()
	trc.Unlock()
}
