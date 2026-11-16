package kit

import (
	"gioui.org/layout"
)

// AccordionSection is one section of an Accordion. IDs must stay stable when
// sections are reordered. Body is only laid out while the section is open.
type AccordionSection struct {
	ID, Title, Summary string
	Body               W
}

// Accordion keeps at most one section open. Set Open to a section ID to open
// it programmatically. Each header is a keyboard-focusable Disclosure.
type Accordion struct {
	Open  string
	items Pool[Collapsible]
}

func (t *Theme) Accordion(gtx C, a *Accordion, id string, sections []AccordionSection) D {
	for _, section := range sections {
		if a.items.Get(section.ID).Clicked(gtx) {
			if a.Open == section.ID {
				a.Open = ""
			} else {
				a.Open = section.ID
			}
		}
	}
	children := make([]layout.FlexChild, 0, len(sections)*2)
	for i, section := range sections {
		section := section
		if i > 0 {
			children = append(children, Rigid(Gap(0, t.Space(4))))
		}
		children = append(children, Rigid(func(gtx C) D {
			state := a.items.Get(section.ID)
			state.Open = a.Open == section.ID
			if section.Body == nil {
				section.Body = func(C) D { return D{} }
			}
			return t.Disclosure(gtx, state, Part(id, section.ID), section.Title, section.Summary, section.Body)
		}))
	}
	return namedGroup(gtx, id, "Accordion", func(gtx C) D { return Col(gtx, children...) })
}

// Breadcrumb is a location in a path. A nil Clickable marks the current
// location as plain text; earlier locations use a Link and are actionable.
type Breadcrumb struct {
	ID, Label string
	Clickable *Clickable
}

// Breadcrumbs wraps whole path segments when the available width is short.
func (t *Theme) Breadcrumbs(gtx C, id string, path []Breadcrumb) D {
	items := make([]W, 0, len(path))
	for i, part := range path {
		i, part := i, part
		items = append(items, func(gtx C) D {
			return Row(gtx,
				Rigid(func(gtx C) D {
					if i == 0 {
						return D{}
					}
					return layout.Inset{Right: t.Space(6)}.Layout(gtx, func(gtx C) D {
						return IconChevronRight(gtx, 12, t.Dim)
					})
				}),
				Rigid(func(gtx C) D {
					if part.Clickable != nil {
						return t.Link(gtx, part.Clickable, Part(id, part.ID), part.Label)
					}
					return namedGroup(gtx, Part(id, part.ID), part.Label, func(gtx C) D {
						// Match Link's top inset and reserved underline space so Flow's
						// top alignment keeps every breadcrumb on the same text line.
						d := layout.Inset{Top: t.Space(3), Bottom: t.Space(3), Left: t.Space(3), Right: t.Space(3)}.Layout(gtx, func(gtx C) D {
							gtx.Constraints.Min.X = 0
							return t.Text(gtx, part.Label, t.Type.Body, t.Fg, SemiBold)
						})
						underline := max(1, gtx.Dp(1))
						d.Size.Y += underline
						d.Baseline += underline
						return d
					})
				}),
			)
		})
	}
	return namedGroup(gtx, id, "Breadcrumbs", func(gtx C) D {
		return Flow(gtx, t.Space(6), t.Space(4), items...)
	})
}
