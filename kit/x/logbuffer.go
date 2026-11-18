package x

import (
	"fmt"
	"time"
)

// LogLevel orders log severity, lowest first. It is independent of
// log/slog's Level: map slog records onto these four levels when feeding a
// LogBuffer.
type LogLevel int

const (
	LogDebug LogLevel = iota // diagnostics, usually hidden
	LogInfo                  // normal operation
	LogWarn                  // something needs attention
	LogError                 // something failed
)

// logLevels lists every defined level in order.
var logLevels = [...]LogLevel{LogDebug, LogInfo, LogWarn, LogError}

// String returns the level's column label ("DEBUG", "INFO", "WARN",
// "ERROR"), or "LogLevel(n)" for an undefined value.
func (l LogLevel) String() string {
	switch l {
	case LogDebug:
		return "DEBUG"
	case LogInfo:
		return "INFO"
	case LogWarn:
		return "WARN"
	case LogError:
		return "ERROR"
	}
	return fmt.Sprint("LogLevel(", int(l), ")")
}

// LogLine is one log record.
type LogLine struct {
	Time   time.Time // when it was logged; zero hides the timestamp
	Level  LogLevel  // colours the line and filters it (LogViewState.MinLevel)
	Source string    // optional: component, logger name
	Text   string    // the message
}

// DefaultLogCap is the capacity of a LogBuffer whose Cap is zero or
// negative.
const DefaultLogCap = 10000

// LogBuffer is a fixed-capacity ring of log lines: once full, each Append
// drops the oldest line. The zero value is an empty buffer of
// DefaultLogCap lines.
//
// Like all app state it belongs to the UI goroutine: append from elsewhere
// via App.Post.
type LogBuffer struct {
	// Cap is the most lines held; zero or negative means DefaultLogCap. It
	// may change at any time and takes effect at the next Append, which
	// first drops the oldest lines beyond the new capacity.
	Cap int

	lines []LogLine
	head  int    // index in lines of the oldest line; nonzero only when full
	total uint64 // lines ever appended; line i of Len has sequence total-Len+i
}

func (b *LogBuffer) capacity() int {
	if b.Cap <= 0 {
		return DefaultLogCap
	}
	return b.Cap
}

// Append adds lines at the newest end, dropping the oldest lines beyond the
// capacity.
func (b *LogBuffer) Append(ls ...LogLine) {
	c := b.capacity()
	if len(b.lines) > c || b.head != 0 && len(b.lines) != c {
		b.resize(c)
	}
	for _, l := range ls {
		if len(b.lines) < c {
			b.lines = append(b.lines, l)
		} else {
			b.lines[b.head] = l
			b.head = (b.head + 1) % c
		}
		b.total++
	}
}

// resize puts the ring in order, keeping at most the newest c lines, after
// Cap changed.
func (b *LogBuffer) resize(c int) {
	n := len(b.lines)
	keep := min(n, c)
	lines := make([]LogLine, 0, keep)
	for i := n - keep; i < n; i++ {
		lines = append(lines, b.At(i))
	}
	b.lines, b.head = lines, 0
}

// Len is the number of lines held.
func (b *LogBuffer) Len() int { return len(b.lines) }

// At returns the i-th oldest line held. It panics unless 0 <= i < Len().
func (b *LogBuffer) At(i int) LogLine {
	if i < 0 || i >= len(b.lines) {
		panic(fmt.Errorf("x: LogBuffer.At(%d) out of range with Len %d", i, len(b.lines)))
	}
	return b.lines[(b.head+i)%len(b.lines)]
}

// Clear drops every line and releases the references the buffer held. It
// keeps the allocated capacity.
func (b *LogBuffer) Clear() {
	clear(b.lines)
	b.lines = b.lines[:0]
	b.head = 0
}

// first is the sequence number of the oldest line held.
func (b *LogBuffer) first() uint64 { return b.total - uint64(len(b.lines)) }
