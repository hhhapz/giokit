package x

import (
	"time"

	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
)

// LyricLine starts at At and remains current until the next line starts.
type LyricLine struct {
	At   time.Duration
	Text string
}

// ActiveLyric returns the current line, or -1 before the first one.
func ActiveLyric(lines []LyricLine, at time.Duration) int {
	lo, hi := 0, len(lines)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if lines[m].At <= at {
			lo = m + 1
		} else {
			hi = m
		}
	}
	return lo - 1
}

// Lyrics is a timed, scrollable line view of Lines at time At; the current
// line is highlighted and, while following, kept in view. The scroll region's
// id is ID.
type Lyrics struct {
	ID    string
	Lines []LyricLine
	At    time.Duration
}

// LyricsState is a Lyrics' scroll position and follow mode. Follow turns off
// when the listener scrolls the transcript; turning it on jumps to the
// current line.
type LyricsState struct {
	List       kit.List
	Follow     bool
	lastActive int
	lastFollow bool
	seen       bool
}

func (ly Lyrics) Layout(gtx C, t *kit.Theme, l *LyricsState) D {
	id, lines, at := ly.ID, ly.Lines, ly.At
	active := ActiveLyric(lines, at)
	jumped := l.Follow && (active != l.lastActive || !l.lastFollow) && active >= 0
	if jumped {
		l.List.ScrollTo(max(0, active-2))
	}
	prior := l.List.Position
	l.lastActive, l.lastFollow = active, l.Follow
	d := t.List(gtx, &l.List, id, len(lines), func(gtx C, i int) D {
		line := lines[i]
		fg, weight := t.Muted, kit.Regular
		if i == active {
			fg, weight = t.Fg, kit.SemiBold
		}
		return layout.Inset{Top: t.Space(kit.Space2), Bottom: t.Space(kit.Space2), Left: t.Space(kit.Space3), Right: t.Space(kit.Space3)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return t.ParagraphLine(gtx, line.Text, fg, weight)
		})
	})
	if l.seen && l.Follow && !jumped && (l.List.Position.First != prior.First || l.List.Position.Offset != prior.Offset) {
		l.Follow = false
		kit.Notify(gtx)
	}
	l.seen = true
	return d
}
