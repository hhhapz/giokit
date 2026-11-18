package x

import (
	"sort"
	"time"

	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
)

// lyricLeadLines is how many lines stay visible above the current line when
// following scrolls to it, so the listener keeps some context.
const lyricLeadLines = 2

// LyricLine starts at At and remains current until the next line starts.
type LyricLine struct {
	At   time.Duration // start time from the beginning of the track
	Text string        // the line; "" draws an empty line
}

// ActiveLyric returns the index of the line current at time at, or -1
// before the first one. lines must be sorted by At.
func ActiveLyric(lines []LyricLine, at time.Duration) int {
	return sort.Search(len(lines), func(i int) bool { return lines[i].At > at }) - 1
}

// Lyrics is a timed, scrollable line view of Lines at time At; the current
// line is highlighted and, while following, kept in view. The scroll region's
// id is ID. Lines must be sorted by At.
type Lyrics struct {
	ID    string        // the scroll region's id
	Lines []LyricLine   // sorted by At
	At    time.Duration // the playback position
}

// LyricsState is a Lyrics' scroll position and follow mode. Follow turns off
// when the listener scrolls the transcript; turning it on jumps to the
// current line. The zero value does not follow.
type LyricsState struct {
	List       kit.List // the scroll position
	Follow     bool     // keep the current line in view; set it to jump there
	lastActive int
	lastFollow bool
	seen       bool
}

// Layout draws the lines, scrolling to the current one when it changes while
// following, and turns Follow off when the listener scrolls.
func (ly Lyrics) Layout(gtx layout.Context, th *kit.Theme, st *LyricsState) layout.Dimensions {
	active := ActiveLyric(ly.Lines, ly.At)
	jumped := st.Follow && (active != st.lastActive || !st.lastFollow) && active >= 0
	if jumped {
		st.List.ScrollTo(max(0, active-lyricLeadLines))
	}
	prior := st.List.Position
	st.lastActive, st.lastFollow = active, st.Follow
	d := th.List(gtx, &st.List, ly.ID, len(ly.Lines), func(gtx layout.Context, i int) layout.Dimensions {
		fg, weight := th.Muted, kit.Regular
		if i == active {
			fg, weight = th.Fg, kit.SemiBold
		}
		inset := layout.Inset{Top: th.Space(kit.Space2), Bottom: th.Space(kit.Space2), Left: th.Space(kit.Space3), Right: th.Space(kit.Space3)}
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return th.ParagraphLine(gtx, ly.Lines[i].Text, fg, weight)
		})
	})
	scrolled := st.List.Position.First != prior.First || st.List.Position.Offset != prior.Offset
	if st.seen && st.Follow && !jumped && scrolled {
		st.Follow = false
		kit.Notify(gtx)
	}
	st.seen = true
	return d
}
