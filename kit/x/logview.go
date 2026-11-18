// Package x holds specialised widgets built on kit and kit/ui: a log view
// and a lyrics view. They follow kit/ui's props-struct shape.
package x

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

// logTimeFormat is the timestamp column's layout: wall-clock time to the
// millisecond.
const logTimeFormat = "15:04:05.000"

const (
	// rowPadY is the space above and below each log line: dense, but lines
	// do not touch.
	rowPadY unit.Dp = 1
	// measureMax is a constraint, in pixels, that no level name reaches.
	measureMax = 1 << 20
)

// LogView shows Buffer: virtualized, monospace, coloured by level. It
// follows the tail while scrolled to the bottom; scrolling up pauses that and
// shows a "jump to latest" button (ID/jump). Filtering is incremental, so
// it stays cheap on large buffers. The scroll region's id is ID.
type LogView struct {
	ID     string     // the scroll region's id; the jump button is ID/jump
	Buffer *LogBuffer // the lines; must not be nil
}

// LogViewState is a LogView's filter, scroll position and filter index.
// The filter fields are state-owned values: set them directly and the index
// rebuilds at the next Count or Layout. The zero value shows every line and
// follows the tail. A LogViewState indexes one LogBuffer; use a new state
// for a different buffer.
type LogViewState struct {
	Filter   string   // case-insensitive substring of Text or Source; "" shows everything
	MinLevel LogLevel // hide lines below this level
	NoTime   bool     // hide the timestamp column

	list    kit.List
	jump    kit.Clickable
	key     filterKey // filter the index was built for
	matched []uint64  // sequence numbers of matching lines
	scanned uint64    // lines scanned so far (sequence)
	init    bool
}

// filterKey identifies the filter a LogViewState's index matches.
type filterKey struct {
	text     string // lower-cased Filter
	minLevel LogLevel
}

// Following reports whether the view is pinned to the newest line.
func (v *LogViewState) Following() bool { return !v.list.Position.BeforeEnd }

// refresh brings the index up to date with b: it restarts after a filter
// change, forgets lines b has dropped and scans lines appended since.
func (v *LogViewState) refresh(b *LogBuffer) {
	k := filterKey{text: strings.ToLower(v.Filter), minLevel: v.MinLevel}
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
	for seq := v.scanned; seq < b.total; seq++ {
		l := b.At(int(seq - first))
		if l.Level < k.minLevel {
			continue
		}
		if k.text != "" && !strings.Contains(strings.ToLower(l.Text), k.text) && !strings.Contains(strings.ToLower(l.Source), k.text) {
			continue
		}
		v.matched = append(v.matched, seq)
	}
	v.scanned = b.total
}

// Count is the number of lines of b the view shows. It syncs the filter
// index first, so it is safe to draw above the view (a toolbar) in the same
// frame.
func (v *LogViewState) Count(b *LogBuffer) int {
	v.refresh(b)
	return len(v.matched)
}

func levelColor(th *kit.Theme, l LogLevel) color.NRGBA {
	switch l {
	case LogDebug:
		return th.Dim
	case LogWarn:
		return th.Warn
	case LogError:
		return th.Bad
	}
	return th.Accent
}

// Layout handles the jump button, syncs the filter index and draws the
// view. It fills the constraints.
func (lv LogView) Layout(gtx layout.Context, th *kit.Theme, v *LogViewState) layout.Dimensions {
	b, id := lv.Buffer, lv.ID
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
	size := th.Type.Small
	// The list is laid out first and the jump button after it: whether to
	// show the button depends on the scroll position the list just applied.
	// (layout.Stack would lay out the button first and show it a frame late.)
	gtx.Constraints.Min = gtx.Constraints.Max
	dims := kit.Fill(gtx, th.Bg, func(gtx layout.Context) layout.Dimensions {
		if len(v.matched) == 0 {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				msg := th.Strings.NoLogLines
				if v.Filter != "" || v.MinLevel > LogDebug {
					msg = th.Strings.NoLogMatches
				}
				return th.Text(gtx, msg, th.Type.Body, th.Dim, kit.Regular)
			})
		}
		levelW := levelWidth(gtx, th, size)
		return th.List(gtx, &v.list, id, len(v.matched), func(gtx layout.Context, i int) layout.Dimensions {
			return logRow(gtx, th, v, b.At(int(v.matched[i]-first)), size, levelW)
		})
	})
	if !v.Following() && len(v.matched) > 0 {
		m := op.Record(gtx.Ops)
		gtx := gtx
		gtx.Constraints.Min = image.Point{}
		d := ui.Button{ID: kit.Part(id, "jump"), Text: th.Strings.Latest, Icon: kit.IconArrowDown, Style: ui.Primary}.Layout(gtx, th, &v.jump)
		call := m.Stop()
		margin := image.Pt(gtx.Dp(th.Space(kit.Space6)), gtx.Dp(th.Space(kit.Space3)))
		st := op.Offset(dims.Size.Sub(d.Size).Sub(margin)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		st.Pop()
	}
	return dims
}

// levelWidth is the level column's width: the widest level name in the mono
// face at size, plus a gap, so the column follows the font and text size.
func levelWidth(gtx layout.Context, th *kit.Theme, size unit.Sp) int {
	m := op.Record(gtx.Ops) // measured, then dropped
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(measureMax, measureMax)}
	w := 0
	for _, lv := range logLevels {
		w = max(w, th.Code(g, lv.String(), size, th.Fg).Size.X)
	}
	m.Stop()
	return w + gtx.Sp(size)/2
}

func logRow(gtx layout.Context, th *kit.Theme, v *LogViewState, l LogLine, size unit.Sp, levelW int) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Top: rowPadY, Bottom: rowPadY, Left: th.Space(kit.Space2p5), Right: th.Space(kit.Space2p5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return kit.Row(gtx,
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				if v.NoTime || l.Time.IsZero() {
					return layout.Dimensions{}
				}
				return layout.Inset{Right: th.Space(kit.Space2p5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return th.Code(gtx, l.Time.Format(logTimeFormat), size, th.Dim)
				})
			}),
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(image.Pt(levelW, gtx.Constraints.Max.Y))
				gtx.Constraints.Min.Y = 0
				d := th.Code(gtx, l.Level.String(), size, levelColor(th, l.Level))
				return layout.Dimensions{Size: image.Pt(levelW, d.Size.Y), Baseline: d.Baseline}
			}),
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				if l.Source == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Right: th.Space(kit.Space2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return th.Code(gtx, l.Source, size, th.Muted)
				})
			}),
			kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				fg := th.Fg
				if l.Level == LogDebug {
					fg = th.Muted
				}
				return th.Code(gtx, l.Text, size, fg)
			}),
		)
	})
}

// ListPos returns the list's scroll position as of the last layout, for
// tests and debugging.
func (v *LogViewState) ListPos() layout.Position { return v.list.Position }
