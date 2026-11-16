package kit

import (
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
)

// Combobox is a text field that suggests options as you type (th.Combobox).
// It owns its editor's events: read the value with Editor.Text().
type Combobox struct {
	Popup
	Editor widget.Editor

	items   Pool[Clickable]
	matches []string
	hi      int // keyboard choice in matches, -1 for none
}

// Combobox lays out the field and, while the text matches options, a list
// of them below it. Typing filters (case-insensitive substring); Down/Up
// choose, Enter or a click takes the choice, Escape closes the list. Any
// text is allowed: the options are suggestions. Ids: the field id, options
// <id>/opt/<index> into the current matches, the list <id>/list.
func (t *Theme) Combobox(gtx C, cb *Combobox, id, placeholder string, options []string) D {
	cb.Editor.SingleLine = true
	cb.KeepFocus = true
	// Input first (rule 1): picks, keys, then edits.
	for i, m := range cb.matches {
		if cb.items.Get(itoa(i)).Clicked(gtx) {
			cb.take(gtx, m)
		}
	}
	for {
		e, ok := gtx.Event(
			key.Filter{Focus: &cb.Editor, Name: key.NameDownArrow}, key.Filter{Focus: &cb.Editor, Name: key.NameUpArrow},
			key.Filter{Focus: &cb.Editor, Name: key.NameReturn}, key.Filter{Focus: &cb.Editor, Name: key.NameEnter},
		)
		if !ok {
			break
		}
		ke, ok := e.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case key.NameDownArrow, key.NameUpArrow:
			if !cb.open {
				cb.filter(options, true)
				cb.hi = -1
			}
			if len(cb.matches) > 0 {
				d := 1
				if ke.Name == key.NameUpArrow {
					d = -1
				}
				cb.hi = (cb.hi + d + len(cb.matches) + 1) % (len(cb.matches) + 1)
				if cb.hi == len(cb.matches) {
					cb.hi = -1
				}
			}
			Notify(gtx)
		default: // Enter
			if cb.open && cb.hi >= 0 && cb.hi < len(cb.matches) {
				cb.take(gtx, cb.matches[cb.hi])
			}
		}
	}
	for {
		e, ok := cb.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := e.(widget.ChangeEvent); ok {
			cb.filter(options, false)
			cb.hi = -1
			Notify(gtx)
		}
	}
	if !gtx.Focused(&cb.Editor) && cb.open && !cb.focusNext {
		cb.Hide() // tabbed away
	}
	return t.Popover(gtx, &cb.Popup, Part(id, "list"), func(gtx C) D {
		return t.field(gtx, &cb.Editor, id, nil, placeholder, 0)
	}, func(gtx C) D {
		w := 0 // the field's width, or the widest match if wider
		for _, m := range cb.matches {
			w = max(w, t.menuRowWidth(gtx, false, m, "", false))
		}
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, max(gtx.Constraints.Min.X, w+gtx.Dp(8)))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(4).Layout(gtx, func(gtx C) D {
			var rows []layout.FlexChild
			for i, m := range cb.matches {
				rows = append(rows, Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return t.menuRowHi(gtx, cb.items.Get(itoa(i)), Part(id, "opt", itoa(i)), nil, m, "", false, false, i == cb.hi)
				}))
			}
			return Col(gtx, rows...)
		})
	})
}

// filter recomputes the matches and opens or closes the list. all shows
// every option (Down on an empty or exact field).
func (cb *Combobox) filter(options []string, all bool) {
	q := strings.ToLower(strings.TrimSpace(cb.Editor.Text()))
	cb.matches = cb.matches[:0]
	for _, o := range options {
		lo := strings.ToLower(o)
		if all || (q != "" && strings.Contains(lo, q) && lo != q) {
			cb.matches = append(cb.matches, o)
		}
		if len(cb.matches) == 8 {
			break
		}
	}
	cb.MatchWidth = true
	if len(cb.matches) > 0 {
		cb.Show()
	} else {
		cb.Hide()
	}
}

func (cb *Combobox) take(gtx C, s string) {
	cb.Editor.SetText(s)
	cb.Editor.SetCaret(cb.Editor.Len(), cb.Editor.Len())
	cb.Hide()
	cb.matches = cb.matches[:0]
	gtx.Execute(key.FocusCmd{Tag: &cb.Editor})
	Notify(gtx)
}
