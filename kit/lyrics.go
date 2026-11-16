package kit

import (
	"time"

	"gioui.org/layout"
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

// Lyrics is a timed, scrollable line view. Follow may be turned off when the
// listener browses the transcript; turning it on jumps to the current line.
type Lyrics struct {
	List       List
	Follow     bool
	lastActive int
	lastFollow bool
	seen       bool
}

func (t *Theme) Lyrics(gtx C, l *Lyrics, id string, lines []LyricLine, at time.Duration) D {
	active := ActiveLyric(lines, at)
	jumped := l.Follow && (active != l.lastActive || !l.lastFollow) && active >= 0
	if jumped {
		l.List.ScrollTo(max(0, active-2))
	}
	prior := l.List.Position
	l.lastActive, l.lastFollow = active, l.Follow
	d := t.List(gtx, &l.List, id, len(lines), func(gtx C, i int) D {
		line := lines[i]
		fg, weight := t.Muted, Regular
		if i == active {
			fg, weight = t.Fg, SemiBold
		}
		return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return t.ParagraphLine(gtx, line.Text, fg, weight)
		})
	})
	if l.seen && l.Follow && !jumped && (l.List.Position.First != prior.First || l.List.Position.Offset != prior.Offset) {
		l.Follow = false
		Notify(gtx)
	}
	l.seen = true
	return d
}
