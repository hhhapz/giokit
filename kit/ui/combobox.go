package ui

import (
	"strings"

	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/widget"

	"github.com/hhhapz/giokit/kit"
)

// Combobox is a text field that suggests Options as you type. Any text is
// allowed: the options are suggestions, and taking one puts its Label in
// the field. Read the value with state.Editor.Text().
//
// Typing filters the options (case-insensitive substring of the Label),
// showing at most eight. An option whose Label equals the whole text, case
// aside, is not suggested: the field already holds it. Down/Up choose
// (wrapping through "no choice"), Down on a closed list shows every
// option, Enter (Return or keypad) or a click takes the choice, Escape
// closes the list.
//
// Like the other editors, a Combobox handles its input in Layout (see the
// package doc): its suggestions depend on Options. Ids: the field ID, the
// list ID/list, suggestions ID/opt/<option ID>.
type Combobox struct {
	ID          string          // the field's id; see the type doc for its parts
	Placeholder string          // dim hint text drawn while the editor is empty
	Options     []Choice        // IDs must be unique and stable
	Size        kit.ControlSize // ControlDefault is the theme's
}

// ComboboxState is a Combobox's interaction state: its editor and the
// suggestion list. The zero value is an empty, closed field; do not copy
// it after first use.
type ComboboxState struct {
	// Editor holds the field's text, the combobox's value. Read and set it
	// directly; Layout sets its SingleLine.
	Editor widget.Editor

	popup   kit.Popup
	items   kit.Pool[kit.Clickable]
	matches []Choice
	hi      int // keyboard choice in matches, -1 for none
}

// Opened reports whether the suggestion list is open.
func (s *ComboboxState) Opened() bool { return s.popup.Opened() }

// maxSuggestions caps a suggestion list: a combobox's matches and the
// command palette's results.
const maxSuggestions = 8

// Layout handles this frame's input (suggestion clicks, keys, edits)
// before drawing, then draws the field and, while open, the suggestions in
// a popover as wide as the field. Taking a suggestion writes its Label
// into s.Editor; there is no Update.
func (c Combobox) Layout(gtx layout.Context, th *kit.Theme, s *ComboboxState) layout.Dimensions {
	s.Editor.SingleLine = true
	s.popup.KeepFocus = true
	s.popup.MatchWidth = true
	for _, m := range s.matches {
		if s.items.Get(m.ID).Clicked(gtx) {
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
	if !gtx.Focused(&s.Editor) && s.popup.Opened() && !s.popup.Opening() {
		s.popup.Hide() // tabbed away
	}
	s.popup.Role = kit.RoleListbox
	return th.Popover(gtx, &s.popup, kit.Part(c.ID, "list"), func(gtx layout.Context) layout.Dimensions {
		return th.EditorField(gtx, &s.Editor, c.ID, kit.EditorField{Placeholder: c.Placeholder, Size: c.Size, Role: kit.RoleCombobox})
	}, func(gtx layout.Context) layout.Dimensions { return c.suggestions(gtx, th, s) })
}

// suggestions lays out the matches, as wide as the field or the widest
// match.
func (c Combobox) suggestions(gtx layout.Context, th *kit.Theme, s *ComboboxState) layout.Dimensions {
	w := 0
	for _, m := range s.matches {
		w = max(w, th.MenuRowWidth(gtx, kit.MenuRow{Label: m.Label}))
	}
	pad := th.Space(kit.Space1)
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, max(gtx.Constraints.Min.X, w+2*gtx.Dp(pad)))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	rows := make([]layout.FlexChild, 0, len(s.matches))
	for i, m := range s.matches {
		rows = append(rows, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			row := kit.MenuRow{Label: m.Label, Highlighted: i == s.hi, Role: kit.RoleOption}
			return th.MenuRow(gtx, s.items.Get(m.ID), kit.Part(c.ID, "opt", m.ID), row)
		}))
	}
	return kit.Flex{Axis: layout.Vertical, Pad: kit.Space1}.Layout(gtx, th, rows...)
}

// keys moves the keyboard choice and takes it on Enter. Focus stays in the
// editor, so these are read from it directly.
func (s *ComboboxState) keys(gtx layout.Context, options []Choice) {
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
			if !s.popup.Opened() {
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
			if s.popup.Opened() && s.hi >= 0 && s.hi < len(s.matches) {
				s.take(gtx, s.matches[s.hi])
			}
		}
	}
}

// filter recomputes the matches and opens or closes the list. all shows
// every option (Down on a closed list).
func (s *ComboboxState) filter(options []Choice, all bool) {
	q := strings.ToLower(strings.TrimSpace(s.Editor.Text()))
	s.matches = s.matches[:0]
	for _, o := range options {
		lo := strings.ToLower(o.Label)
		if all || (q != "" && strings.Contains(lo, q) && lo != q) {
			s.matches = append(s.matches, o)
		}
		if len(s.matches) == maxSuggestions {
			break
		}
	}
	if len(s.matches) > 0 {
		s.popup.Show()
	} else {
		s.popup.Hide()
	}
}

// take puts an option's label in the field and closes the list.
func (s *ComboboxState) take(gtx layout.Context, o Choice) {
	s.Editor.SetText(o.Label)
	s.Editor.SetCaret(s.Editor.Len(), s.Editor.Len())
	s.popup.Hide()
	s.matches = s.matches[:0]
	gtx.Execute(key.FocusCmd{Tag: &s.Editor})
	kit.Notify(gtx)
}
