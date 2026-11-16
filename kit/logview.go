package kit

import (
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// LogLevel orders log severity.
type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
)

func (l LogLevel) String() string {
	return [...]string{"DEBUG", "INFO", "WARN", "ERROR"}[max(0, min(int(l), 3))]
}

// LogLine is one log record.
type LogLine struct {
	Time   time.Time
	Level  LogLevel
	Source string // optional: component, logger name
	Text   string
}

// LogBuffer is a fixed-capacity ring of log lines. Like all app state it
// belongs to the UI goroutine: append from elsewhere via App.Post.
type LogBuffer struct {
	Cap   int // 0 → 10000
	lines []LogLine
	head  int    // index of the oldest line once full
	total uint64 // lines ever appended; line i of Len has sequence total-Len+i
}

func (b *LogBuffer) Append(ls ...LogLine) {
	if b.Cap == 0 {
		b.Cap = 10000
	}
	for _, l := range ls {
		if len(b.lines) < b.Cap {
			b.lines = append(b.lines, l)
		} else {
			b.lines[b.head] = l
			b.head = (b.head + 1) % b.Cap
		}
		b.total++
	}
}

func (b *LogBuffer) Len() int { return len(b.lines) }

// At returns the i-th oldest line still held.
func (b *LogBuffer) At(i int) LogLine { return b.lines[(b.head+i)%len(b.lines)] }

// Clear drops every line.
func (b *LogBuffer) Clear() { b.lines, b.head = b.lines[:0], 0 }

func (b *LogBuffer) first() uint64 { return b.total - uint64(len(b.lines)) }

// LogView shows a LogBuffer: virtualized, monospace, colored by level. It
// follows the tail while scrolled to the bottom; scrolling up pauses that and
// shows a "jump to latest" button. Filtering is incremental, so it stays
// cheap on large buffers.
type LogView struct {
	Filter   string   // case-insensitive substring; "" shows everything
	MinLevel LogLevel // hide lines below this level
	NoTime   bool     // hide the timestamp column

	list    List
	jump    Clickable
	key     string   // filter key the index was built for
	matched []uint64 // sequence numbers of matching lines
	scanned uint64   // lines scanned so far (sequence)
	init    bool
}

// Following reports whether the view is pinned to the newest line.
func (v *LogView) Following() bool { return !v.list.Position.BeforeEnd }

func (v *LogView) refresh(b *LogBuffer) {
	k := strings.ToLower(v.Filter) + "\x00" + v.MinLevel.String()
	if k != v.key {
		v.key, v.matched, v.scanned = k, v.matched[:0], b.first()
	}
	first := b.first()
	if v.scanned < first {
		v.scanned = first
	}
	drop := 0
	for drop < len(v.matched) && v.matched[drop] < first {
		drop++
	}
	if drop > 0 {
		v.matched = append(v.matched[:0], v.matched[drop:]...)
	}
	f := strings.ToLower(v.Filter)
	for seq := v.scanned; seq < b.total; seq++ {
		l := b.At(int(seq - first))
		if l.Level < v.MinLevel {
			continue
		}
		if f != "" && !strings.Contains(strings.ToLower(l.Text), f) && !strings.Contains(strings.ToLower(l.Source), f) {
			continue
		}
		v.matched = append(v.matched, seq)
	}
	v.scanned = b.total
}

// Count is the number of lines of b the view shows. It syncs the filter
// index first, so it is safe to draw above the view (a toolbar) in the same
// frame.
func (v *LogView) Count(b *LogBuffer) int { v.refresh(b); return len(v.matched) }

func (t *Theme) levelColor(l LogLevel) color.NRGBA {
	switch l {
	case LogDebug:
		return t.Dim
	case LogWarn:
		return t.Warn
	case LogError:
		return t.Bad
	}
	return t.Accent
}

// LogView lays out v over b. ids: id (scroll region), <id>/jump.
func (t *Theme) LogView(gtx C, v *LogView, b *LogBuffer, id string) D {
	if !v.init {
		v.init = true
		v.list.ScrollToEnd = true
		v.list.Position.BeforeEnd = false
	}
	if v.jump.Clicked(gtx) {
		v.list.ScrollTo(len(v.matched))
		v.list.Position.BeforeEnd = false
	}
	v.refresh(b)
	first := b.first()
	size := t.Type.Small
	// The list is laid out first and the jump button after it: whether to
	// show the button depends on the scroll position the list just applied.
	// (layout.Stack would lay out the button first and show it a frame late.)
	gtx.Constraints.Min = gtx.Constraints.Max
	dims := Fill(gtx, t.Bg, func(gtx C) D {
		if len(v.matched) == 0 {
			return layout.Center.Layout(gtx, func(gtx C) D {
				msg := "No log lines yet"
				if v.Filter != "" || v.MinLevel > 0 {
					msg = "No lines match the filter"
				}
				return t.Text(gtx, msg, t.Type.Body, t.Dim, Regular)
			})
		}
		levelW := t.levelWidth(gtx, size)
		return t.List(gtx, &v.list, id, len(v.matched), func(gtx C, i int) D {
			return t.logRow(gtx, v, b.At(int(v.matched[i]-first)), size, levelW)
		})
	})
	if !v.Following() && len(v.matched) > 0 {
		m := op.Record(gtx.Ops)
		gtx := gtx
		gtx.Constraints.Min = image.Point{}
		d := t.IconLabelButton(gtx, &v.jump, Part(id, "jump"), IconArrowDown, "Latest", Primary)
		call := m.Stop()
		off := image.Pt(dims.Size.X-d.Size.X-gtx.Dp(24), dims.Size.Y-d.Size.Y-gtx.Dp(12))
		st := op.Offset(off).Push(gtx.Ops)
		call.Add(gtx.Ops)
		st.Pop()
	}
	return dims
}

// levelWidth is the level column's width: the widest level name in the mono
// face at size, plus a gap, so the column follows the font and text size.
func (t *Theme) levelWidth(gtx C, size unit.Sp) int {
	m := op.Record(gtx.Ops) // measured, then dropped
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
	w := 0
	for _, lv := range []LogLevel{LogDebug, LogInfo, LogWarn, LogError} {
		w = max(w, t.Code(g, lv.String(), size, t.Fg).Size.X)
	}
	m.Stop()
	return w + gtx.Sp(size)/2
}

func (t *Theme) logRow(gtx C, v *LogView, l LogLine, size unit.Sp, levelW int) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Top: 1, Bottom: 1, Left: t.Space(10), Right: t.Space(10)}.Layout(gtx, func(gtx C) D {
		return Row(gtx,
			Rigid(func(gtx C) D {
				if v.NoTime || l.Time.IsZero() {
					return D{}
				}
				return layout.Inset{Right: 10}.Layout(gtx, func(gtx C) D {
					return t.Code(gtx, l.Time.Format("15:04:05.000"), size, t.Dim)
				})
			}),
			Rigid(func(gtx C) D {
				gtx.Constraints = layout.Exact(image.Pt(levelW, gtx.Constraints.Max.Y))
				gtx.Constraints.Min.Y = 0
				d := t.Code(gtx, l.Level.String(), size, t.levelColor(l.Level))
				return D{Size: image.Pt(levelW, d.Size.Y), Baseline: d.Baseline}
			}),
			Rigid(func(gtx C) D {
				if l.Source == "" {
					return D{}
				}
				return layout.Inset{Right: 8}.Layout(gtx, func(gtx C) D { return t.Code(gtx, l.Source, size, t.Muted) })
			}),
			Flexed(1, func(gtx C) D {
				fg := t.Fg
				if l.Level == LogDebug {
					fg = t.Muted
				}
				return t.Code(gtx, l.Text, size, fg)
			}),
		)
	})
}

// ListPos exposes the list position (tests and debugging).
func (v *LogView) ListPos() layout.Position { return v.list.Position }
