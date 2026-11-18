package x

import (
	"slices"
	"strconv"
	"testing"
)

func appendN(b *LogBuffer, from, to int) {
	for i := from; i < to; i++ {
		b.Append(LogLine{Text: strconv.Itoa(i)})
	}
}

// texts returns the buffer's lines oldest first.
func texts(b *LogBuffer) []string {
	var out []string
	for i := range b.Len() {
		out = append(out, b.At(i).Text)
	}
	return out
}

func mustPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: no panic", name)
		}
	}()
	f()
}

func TestLogBufferAtBounds(t *testing.T) {
	b := LogBuffer{Cap: 3}
	appendN(&b, 0, 5) // wrapped: holds 2, 3, 4
	mustPanic(t, "At(Len())", func() { b.At(b.Len()) })
	mustPanic(t, "At(-1)", func() { b.At(-1) })
	var empty LogBuffer
	mustPanic(t, "empty At(0)", func() { empty.At(0) })
}

func TestLogBufferCapChange(t *testing.T) {
	b := LogBuffer{Cap: 4}
	appendN(&b, 0, 6) // wrapped: 2 3 4 5
	b.Cap = 8
	appendN(&b, 6, 8)
	if got, want := texts(&b), []string{"2", "3", "4", "5", "6", "7"}; !slices.Equal(got, want) {
		t.Fatalf("after growing Cap: %v, want %v", got, want)
	}
	appendN(&b, 8, 11) // wraps at the new capacity: 3 .. 10
	if got, want := texts(&b), []string{"3", "4", "5", "6", "7", "8", "9", "10"}; !slices.Equal(got, want) {
		t.Fatalf("after wrapping at grown Cap: %v, want %v", got, want)
	}
	b.Cap = 3
	appendN(&b, 11, 12)
	if got, want := texts(&b), []string{"9", "10", "11"}; !slices.Equal(got, want) {
		t.Fatalf("after shrinking Cap: %v, want %v", got, want)
	}
	appendN(&b, 12, 14)
	if got, want := texts(&b), []string{"11", "12", "13"}; !slices.Equal(got, want) {
		t.Fatalf("after wrapping at shrunk Cap: %v, want %v", got, want)
	}
}

func TestLogBufferNonPositiveCap(t *testing.T) {
	for _, c := range []int{0, -1} {
		b := LogBuffer{Cap: c}
		appendN(&b, 0, 3)
		if b.Len() != 3 || b.Cap != c {
			t.Errorf("Cap %d: Len %d, Cap now %d", c, b.Len(), b.Cap)
		}
	}
}

// The view's sequence index must survive a Cap change: matched lines that
// were dropped leave the count.
func TestLogViewAfterCapShrink(t *testing.T) {
	b := LogBuffer{Cap: 4}
	appendN(&b, 0, 6)
	var v LogViewState
	if n := v.Count(&b); n != 4 {
		t.Fatalf("count = %d", n)
	}
	b.Cap = 2
	appendN(&b, 6, 7)
	if n := v.Count(&b); n != 2 {
		t.Fatalf("count after shrink = %d, want 2", n)
	}
}

func TestLogBufferClearDropsReferences(t *testing.T) {
	b := LogBuffer{Cap: 4}
	appendN(&b, 0, 6)
	b.Clear()
	if b.Len() != 0 {
		t.Fatalf("Len after Clear = %d", b.Len())
	}
	for i, l := range b.lines[:cap(b.lines)] {
		if l.Text != "" {
			t.Fatalf("Clear kept line %d: %q", i, l.Text)
		}
	}
	appendN(&b, 6, 8)
	if got, want := texts(&b), []string{"6", "7"}; !slices.Equal(got, want) {
		t.Fatalf("after Clear: %v, want %v", got, want)
	}
}

func TestLogLevelString(t *testing.T) {
	for l, want := range map[LogLevel]string{LogDebug: "DEBUG", LogError: "ERROR", -1: "LogLevel(-1)", 7: "LogLevel(7)"} {
		if got := l.String(); got != want {
			t.Errorf("LogLevel(%d).String() = %q, want %q", int(l), got, want)
		}
	}
}
