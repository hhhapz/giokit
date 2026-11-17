package ui

import (
	"strconv"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
)

// Combobox is a text field that suggests Options as you type. Any text is
// allowed: the options are suggestions. Read the value with
// state.Editor.Text().
//
// Typing filters (case-insensitive substring); Down/Up choose, Enter or a
// click takes the choice, Escape closes the list. Ids: the field ID,
// options ID/opt/<index> into the current matches, the list ID/list.
type Combobox struct {
	ID          string
	Placeholder string
	Options     []string
	Size        kit.ControlSize // ControlDefault is the theme's
}

// ComboboxState is a Combobox's interaction state: its editor and list.
type ComboboxState struct {
	kit.Popup
	Editor widget.Editor

	items   kit.Pool[kit.Clickable]
	matches []string
	hi      int // keyboard choice in matches, -1 for none
}

// maxMatches caps the suggestion list.
const maxMatches = 8

// Layout handles this frame's input (picks, keys, edits; rule 1) and lays
// out the field with its suggestions below it.
func (c Combobox) Layout(gtx C, th *kit.Theme, s *ComboboxState) D {
	s.Editor.SingleLine = true
	s.KeepFocus = true
	for i, m := range s.matches {
		if s.items.Get(strconv.Itoa(i)).Clicked(gtx) {
			s.take(gtx, m)
		}
	}
	s.keys(gtx, c.Options)
	for {
		e, ok := s.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := e.(widget.ChangeEvent); ok {
			s.filter(c.Options, false)
			s.hi = -1
			kit.Notify(gtx)
		}
	}
	if !gtx.Focused(&s.Editor) && s.Opened() && !s.Opening() {
		s.Hide() // tabbed away
	}
	return th.Popover(gtx, &s.Popup, kit.Part(c.ID, "list"), func(gtx C) D {
		d := th.EditorField(gtx, &s.Editor, c.ID, kit.EditorField{Placeholder: c.Placeholder, Size: c.Size})
		kit.SetRole(gtx, c.ID, kit.RoleCombobox)
		kit.SetRole(gtx, kit.Part(c.ID, "list"), kit.RoleListbox)
		return d
	}, func(gtx C) D {
		w := 0 // the field's width, or the widest match if wider
		for _, m := range s.matches {
			w = max(w, th.MenuRowWidth(gtx, kit.MenuRow{Label: m}))
		}
		pad := th.Space(kit.Space1)
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, max(gtx.Constraints.Min.X, w+2*gtx.Dp(pad)))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		rows := make([]layout.FlexChild, 0, len(s.matches))
		for i, m := range s.matches {
			rows = append(rows, kit.Rigid(func(gtx C) D {
				id := strconv.Itoa(i)
				return th.MenuRow(gtx, s.items.Get(id), kit.Part(c.ID, "opt", id), kit.MenuRow{Label: m, Highlighted: i == s.hi, Role: kit.RoleOption})
			}))
		}
		return kit.Flex{Axis: layout.Vertical, Pad: kit.Space1}.Layout(gtx, th, rows...)
	})
}

// keys moves the keyboard choice and takes it on Enter. Focus stays in the
// editor, so these are read from it directly.
func (s *ComboboxState) keys(gtx C, options []string) {
	for {
		e, ok := gtx.Event(
			key.Filter{Focus: &s.Editor, Name: key.NameDownArrow}, key.Filter{Focus: &s.Editor, Name: key.NameUpArrow},
			key.Filter{Focus: &s.Editor, Name: key.NameReturn}, key.Filter{Focus: &s.Editor, Name: key.NameEnter},
		)
		if !ok {
			return
		}
		ke, ok := e.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case key.NameDownArrow, key.NameUpArrow:
			if !s.Opened() {
				s.filter(options, true)
				s.hi = -1
			}
			if n := len(s.matches); n > 0 {
				d := 1
				if ke.Name == key.NameUpArrow {
					d = -1
				}
				// n+1 positions: the matches, then "none" (-1).
				s.hi = (s.hi + d + n + 1) % (n + 1)
				if s.hi == n {
					s.hi = -1
				}
			}
			kit.Notify(gtx)
		default: // Enter
			if s.Opened() && s.hi >= 0 && s.hi < len(s.matches) {
				s.take(gtx, s.matches[s.hi])
			}
		}
	}
}

// filter recomputes the matches and opens or closes the list. all shows
// every option (Down on an empty or exact field).
func (s *ComboboxState) filter(options []string, all bool) {
	q := strings.ToLower(strings.TrimSpace(s.Editor.Text()))
	s.matches = s.matches[:0]
	for _, o := range options {
		lo := strings.ToLower(o)
		if all || (q != "" && strings.Contains(lo, q) && lo != q) {
			s.matches = append(s.matches, o)
		}
		if len(s.matches) == maxMatches {
			break
		}
	}
	s.MatchWidth = true
	if len(s.matches) > 0 {
		s.Show()
	} else {
		s.Hide()
	}
}

func (s *ComboboxState) take(gtx C, v string) {
	s.Editor.SetText(v)
	s.Editor.SetCaret(s.Editor.Len(), s.Editor.Len())
	s.Hide()
	s.matches = s.matches[:0]
	gtx.Execute(key.FocusCmd{Tag: &s.Editor})
	kit.Notify(gtx)
}
