package ui

import (
	"github.com/hhhapz/giokit/gio/layout"

	"github.com/hhhapz/giokit/kit"
)

// Shortcut is one row of a ShortcutList: what it does and the chords that
// do it.
type Shortcut struct {
	Title  string   // what the shortcut does
	Chords []string // display chords (kit.FormatChord's form), any of which works
}

// ShortcutGroup is a titled set of rows in a ShortcutList.
type ShortcutGroup struct {
	Title string // a caption over the rows; "" for none
	Items []Shortcut
}

// ShortcutsFrom lists a keymap's bindings as rows, in the order bound,
// merging bindings that share a title into one row with several chords.
func ShortcutsFrom(km *kit.Keymap) []Shortcut {
	var out []Shortcut
	at := map[string]int{}
	for _, b := range km.Bindings() {
		if i, ok := at[b.Title]; ok {
			out[i].Chords = append(out[i].Chords, b.Chord)
			continue
		}
		at[b.Title] = len(out)
		out = append(out, Shortcut{Title: b.Title, Chords: []string{b.Chord}})
	}
	return out
}

// ShortcutList lays out groups of keyboard shortcuts, for a help dialog:
// each group's caption, then a row per shortcut with its title on the left
// and its chords as key caps on the right. It fills the width and takes no
// input.
type ShortcutList struct {
	Groups []ShortcutGroup
}

// Layout draws the groups top to bottom.
func (s ShortcutList) Layout(gtx layout.Context, th *kit.Theme) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	groups := make([]layout.FlexChild, 0, len(s.Groups))
	for _, g := range s.Groups {
		groups = append(groups, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			rows := make([]layout.FlexChild, 0, len(g.Items)+1)
			if g.Title != "" {
				rows = append(rows, kit.Rigid(func(gtx layout.Context) layout.Dimensions { return th.Caption(gtx, g.Title) }))
			}
			for _, it := range g.Items {
				rows = append(rows, kit.Rigid(func(gtx layout.Context) layout.Dimensions { return shortcutRow(gtx, th, it) }))
			}
			return kit.Flex{Axis: layout.Vertical, Gap: kit.Space1p5}.Layout(gtx, th, rows...)
		}))
	}
	return kit.Flex{Axis: layout.Vertical, Gap: kit.Space4}.Layout(gtx, th, groups...)
}

// shortcutRow is a title and its chords, the chords right-aligned.
func shortcutRow(gtx layout.Context, th *kit.Theme, it Shortcut) layout.Dimensions {
	caps := make([]layout.FlexChild, 0, 2*len(it.Chords))
	for i, c := range it.Chords {
		if i > 0 {
			caps = append(caps, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return th.Text(gtx, "/", th.Type.Small, th.Dim, kit.Regular)
			}))
		}
		caps = append(caps, kit.Rigid(func(gtx layout.Context) layout.Dimensions { return Kbd{Chord: c}.Layout(gtx, th) }))
	}
	return kit.Flex{Align: layout.Middle, Gap: kit.Space3}.Layout(gtx, th,
		kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return th.Text(gtx, it.Title, th.Type.Body, th.Fg, kit.Regular)
		}),
		kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Flex{Align: layout.Middle, Gap: kit.Space1}.Layout(gtx, th, caps...)
		}),
	)
}
