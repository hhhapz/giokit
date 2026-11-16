package kit

import (
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
)

// CommandPalette is a modal, searchable action picker. Call Open from a
// shortcut, then wrap the window with Theme.CommandPalette each frame.
type CommandPalette struct {
	Dialog    Dialog
	Editor    widget.Editor
	items     Pool[Clickable]
	matches   []Action
	selected  int
	focusNext bool
}

func (p *CommandPalette) Open() {
	p.Editor.SetText("")
	p.selected = 0
	p.focusNext = true
	p.Dialog.Show()
}
func (p *CommandPalette) Close() { p.Dialog.Hide() }

func (t *Theme) CommandPalette(gtx C, p *CommandPalette, id string, actions *Actions, page W) D {
	if p.Dialog.Opened() {
		for _, act := range p.matches {
			if p.items.Get(act.ID).Clicked(gtx) {
				actions.Run(act.ID)
				p.Close()
			}
		}
		for {
			e, ok := gtx.Event(key.Filter{Focus: &p.Editor, Name: key.NameDownArrow},
				key.Filter{Focus: &p.Editor, Name: key.NameUpArrow}, key.Filter{Focus: &p.Editor, Name: key.NameReturn})
			if !ok {
				break
			}
			ke, ok := e.(key.Event)
			if !ok || ke.State != key.Press {
				continue
			}
			switch ke.Name {
			case key.NameDownArrow:
				if len(p.matches) > 0 {
					p.selected = (p.selected + 1) % len(p.matches)
					Notify(gtx)
				}
			case key.NameUpArrow:
				if len(p.matches) > 0 {
					p.selected = (p.selected - 1 + len(p.matches)) % len(p.matches)
					Notify(gtx)
				}
			default:
				if len(p.matches) > 0 {
					actions.Run(p.matches[p.selected].ID)
					p.Close()
					Notify(gtx)
				}
			}
		}
		for {
			e, ok := p.Editor.Update(gtx)
			if !ok {
				break
			}
			if _, ok := e.(widget.ChangeEvent); ok {
				p.selected = 0
				Notify(gtx)
			}
		}
		p.matches = actions.Search(p.Editor.Text())
		if len(p.matches) > 8 {
			p.matches = p.matches[:8]
		}
	}
	return t.Modal(gtx, &p.Dialog, id, page, func(gtx C) D {
		if p.focusNext {
			p.focusNext = false
			gtx.Execute(key.FocusCmd{Tag: &p.Editor})
		}
		return Col(gtx,
			Rigid(func(gtx C) D { return t.Text(gtx, "Commands", t.Type.Title, t.Fg, SemiBold) }),
			Rigid(Gap(0, t.Space(12))),
			Rigid(func(gtx C) D { return t.TextFieldIcon(gtx, &p.Editor, Part(id, "query"), IconSearch, "Type a command") }),
			Rigid(Gap(0, t.Space(8))),
			Rigid(func(gtx C) D {
				var rows []layout.FlexChild
				for i, act := range p.matches {
					i, act := i, act
					rows = append(rows, Rigid(func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return t.menuRowHi(gtx, p.items.Get(act.ID), Part(id, act.ID), nil, act.Title, "", false, false, i == p.selected)
					}))
				}
				if len(rows) == 0 {
					rows = append(rows, Rigid(func(gtx C) D { return t.Paragraph(gtx, "No matching commands", t.Muted) }))
				}
				return Col(gtx, rows...)
			}),
		)
	})
}
