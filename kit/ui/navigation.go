package ui

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/hhhapz/giokit/kit"
)

// TabStyle picks how Tabs draws its selection.
type TabStyle int

const (
	TabsUnderline TabStyle = iota // text tabs with a sliding underline
	TabsSegmented                 // a segmented control: a sliding pill on a track
)

// Tabs is a row of tabs with one selected. It is one Tab stop (WAI-ARIA
// tabs): Left/Right move the selection, Home/End jump to the ends,
// Enter/Space re-pick the focused tab. Ids are ID/<label>.
//
//	if i, ok := v.tabs.Update(gtx); ok { v.tab = i }        // handle pass
//	ui.Tabs{ID: "view", Labels: names, Selected: v.tab}.Layout(gtx, th, &v.tabs)
type Tabs struct {
	ID       string
	Labels   []string
	Selected int
	Style    TabStyle
}

// TabsState is a Tabs' interaction state and animated indicator.
type TabsState struct {
	group   kit.FocusGroup
	clicks  kit.Pool[kit.Clickable]
	x, w    kit.Tween
	last    []string
	pending int
	has     bool
}

// Update returns the index of a tab picked (clicked, or moved to with the
// keyboard) since the last call. A click on the selected tab reports it too.
func (s *TabsState) Update(gtx C) (int, bool) {
	for i, l := range s.last {
		if s.clicks.Get(l).Clicked(gtx) {
			s.pending, s.has = i, true
		}
	}
	if !s.has {
		return 0, false
	}
	s.has = false
	return s.pending, true
}

func (tb Tabs) Layout(gtx C, th *kit.Theme, s *TabsState) D {
	s.last = append(s.last[:0], tb.Labels...)
	n := len(tb.Labels)
	pick := func(i int) {
		if n == 0 {
			return
		}
		i = max(0, min(n-1, i))
		s.pending, s.has = i, true
		s.group.Active = tb.Labels[i]
	}
	for i, l := range tb.Labels {
		s.group.Join(s.clicks.Get(l), i == tb.Selected)
	}
	if tb.Selected >= 0 && tb.Selected < n {
		s.group.Active = tb.Labels[tb.Selected]
	}
	// Enter and Space re-pick the focused (selected) tab.
	for {
		e, ok := gtx.Event(key.Filter{Focus: s.group.FocusHandle(), Name: key.NameReturn},
			key.Filter{Focus: s.group.FocusHandle(), Name: key.NameEnter}, key.Filter{Focus: s.group.FocusHandle(), Name: key.NameSpace})
		if !ok {
			break
		}
		if ke, ok := e.(key.Event); ok && ke.State == key.Press && gtx.Enabled() {
			pick(tb.Selected)
			kit.Notify(gtx)
		}
	}
	return kit.KeyContext(gtx, "tabs", func(gtx C) D {
		kit.OnAction(gtx, kit.ActCollapse, func() { pick(tb.Selected - 1) })
		kit.OnAction(gtx, kit.ActExpand, func() { pick(tb.Selected + 1) })
		kit.OnAction(gtx, kit.ActFirst, func() { pick(0) })
		kit.OnAction(gtx, kit.ActLast, func() { pick(n - 1) })
		return s.group.Layout(gtx, tb.ID, func(gtx C) D {
			// The group's own area takes no pointer gestures, so Gio would drop
			// a tag on it: name it on a nested area.
			kit.SetRole(gtx, tb.ID, kit.RoleTabList)
			return kit.TagArea(gtx, tb.ID, func(gtx C) D { return tb.draw(gtx, th, s) })
		})
	})
}

func (tb Tabs) draw(gtx C, th *kit.Theme, s *TabsState) D {
	gtx.Constraints.Min = image.Point{}
	seg := tb.Style == TabsSegmented
	pad := th.Space(kit.Space1p5)
	if seg {
		pad = th.Space(kit.Space1)
	}
	// Lay tabs out first (recorded) to learn their positions, then draw the
	// indicator under them and replay.
	m := op.Record(gtx.Ops)
	type span struct{ x, w int }
	spans := make([]span, len(tb.Labels))
	x, h := gtx.Dp(pad), 0
	for i, l := range tb.Labels {
		st := op.Offset(image.Pt(x, gtx.Dp(pad))).Push(gtx.Ops)
		d := th.Press(gtx, s.clicks.Get(l), kit.Part(tb.ID, l), func(gtx C, st kit.State) D {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(l).Add(gtx.Ops)
			semantic.SelectedOp(i == tb.Selected).Add(gtx.Ops)
			kit.SetRole(gtx, kit.Part(tb.ID, l), kit.RoleTab)
			fg := kit.LerpColor(th.Muted, th.Fg, st.Hover)
			if i == tb.Selected {
				fg = th.Fg
			}
			sf := th.Styled(kit.Surface{Radius: th.RadiusSm()})
			if st.Focused {
				sf.Focus = th.Focus
			}
			if !seg && i != tb.Selected {
				sf.Bg = kit.WithAlpha(th.Hover, uint8(160*st.Hover))
			}
			return sf.Layout(gtx, kit.Pad(th.ControlPad(kit.ControlDefault, 5), th.Space(kit.Space3), func(gtx C) D {
				return th.Text(gtx, l, th.Type.Body, fg, kit.Medium)
			}))
		})
		st.Pop()
		spans[i] = span{x, d.Size.X}
		x += d.Size.X
		if !seg {
			x += gtx.Dp(kit.Space0p5) // a sliver between text tabs
		}
		h = max(h, d.Size.Y)
	}
	call := m.Stop()
	size := image.Pt(x+gtx.Dp(pad), h+2*gtx.Dp(pad))
	stroke := max(1, gtx.Dp(th.Stroke))
	if seg {
		tr := min(gtx.Dp(th.Radius)+gtx.Dp(pad), size.Y/2)
		paint.FillShape(gtx.Ops, th.Panel, clip.UniformRRect(image.Rectangle{Max: size}, tr).Op(gtx.Ops))
		kit.Border(gtx, image.Rectangle{Max: size}, tr, stroke, th.Border)
	}
	if tb.Selected >= 0 && tb.Selected < len(spans) {
		sp := spans[tb.Selected]
		ix := int(s.x.ToEase(gtx, float32(sp.x), th.Dur(kit.DurNormal), kit.EaseInOut))
		iw := int(s.w.ToEase(gtx, float32(sp.w), th.Dur(kit.DurNormal), kit.EaseInOut))
		if seg {
			r := image.Rect(ix, gtx.Dp(pad), ix+iw, gtx.Dp(pad)+h)
			ir := min(gtx.Dp(th.Radius), r.Dy()/2)
			paint.FillShape(gtx.Ops, th.Surface, clip.UniformRRect(r, ir).Op(gtx.Ops))
			kit.Border(gtx, r, ir, stroke, th.Border)
		} else {
			bh := gtx.Dp(th.Ring)
			inset := gtx.Dp(kit.Space2)
			r := image.Rect(ix+inset, size.Y-bh, ix+iw-inset, size.Y)
			paint.FillShape(gtx.Ops, th.Accent, clip.UniformRRect(r, bh/2).Op(gtx.Ops))
		}
	}
	call.Add(gtx.Ops)
	return D{Size: size}
}

// Disclosure is a section with a clickable header (chevron, title, an
// optional trailing count) that shows or hides Body. Flip Open when the
// Clickable reports a click (Enter/Space on the header too). The header id
// is ID.
//
//	if v.more.Clicked(gtx) { v.moreOpen = !v.moreOpen }
//	ui.Disclosure{ID: "more", Title: "More", Open: v.moreOpen, Body: body}.Layout(gtx, th, &v.more)
type Disclosure struct {
	ID       string
	Title    string
	Trailing string // a count or summary pill after the title
	Open     bool
	Body     kit.W
}

func (dc Disclosure) Layout(gtx C, th *kit.Theme, c *kit.Clickable) D {
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return th.Press(gtx, c, dc.ID, func(gtx C, st kit.State) D {
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(dc.Title).Add(gtx.Ops)
				kit.SetRole(gtx, dc.ID, kit.RoleButton)
				semantic.SelectedOp(dc.Open).Add(gtx.Ops)
				s := th.Styled(kit.Surface{Bg: kit.WithAlpha(th.Hover, uint8(255*st.Hover)), Radius: th.Radius})
				if st.Focused {
					s.Focus = th.Focus
				}
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return s.Layout(gtx, func(gtx C) D {
					return layout.Inset{Top: th.Space(kit.Space1p5), Bottom: th.Space(kit.Space1p5), Left: th.Space(kit.Space1), Right: th.Space(kit.Space2)}.Layout(gtx, func(gtx C) D {
						chev := kit.IconChevronRight
						if dc.Open {
							chev = kit.IconChevronDown
						}
						return kit.Row(gtx,
							kit.Rigid(func(gtx C) D { return chev(gtx, th.IconSize, th.Muted) }),
							kit.Rigid(kit.Gap(th.Space(kit.Space1p5), 0)),
							kit.Rigid(func(gtx C) D { return th.Caption(gtx, dc.Title) }),
							kit.Rigid(kit.Gap(th.Space(kit.Space2), 0)),
							kit.Rigid(func(gtx C) D {
								if dc.Trailing == "" {
									return D{}
								}
								return th.Pill(gtx, dc.Trailing, th.Muted, th.Surface)
							}),
						)
					})
				})
			})
		}),
		kit.Rigid(func(gtx C) D {
			if !dc.Open || dc.Body == nil {
				return D{}
			}
			return layout.Inset{Top: th.Space(kit.Space2)}.Layout(gtx, dc.Body)
		}),
	)
}

// Accordion is a stack of Disclosures with at most one open. Headers are
// ID/<section ID>.
type Accordion struct {
	ID       string
	Sections []AccordionSection
	Open     string // the open section's ID; "" for none
}

// AccordionSection is one section. IDs must stay stable when sections are
// reordered. Body is laid out only while the section is open.
type AccordionSection struct {
	ID, Title, Summary string
	Body               kit.W
}

// AccordionState holds the section headers.
type AccordionState struct {
	headers kit.Pool[kit.Clickable]
	last    []string
	open    string
}

// Update returns the section to show open (or "" to close all) after a
// header was clicked since the last call.
func (s *AccordionState) Update(gtx C) (string, bool) {
	for _, id := range s.last {
		if s.headers.Get(id).Clicked(gtx) {
			if s.open == id {
				return "", true
			}
			return id, true
		}
	}
	return "", false
}

func (a Accordion) Layout(gtx C, th *kit.Theme, s *AccordionState) D {
	s.open = a.Open
	s.last = s.last[:0]
	children := make([]layout.FlexChild, 0, len(a.Sections))
	for _, sec := range a.Sections {
		s.last = append(s.last, sec.ID)
		children = append(children, kit.Rigid(func(gtx C) D {
			return Disclosure{ID: kit.Part(a.ID, sec.ID), Title: sec.Title, Trailing: sec.Summary, Open: a.Open == sec.ID, Body: sec.Body}.Layout(gtx, th, s.headers.Get(sec.ID))
		}))
	}
	kit.SetRole(gtx, a.ID, kit.RoleGroup)
	return kit.Group(gtx, a.ID, th.Strings.Accordion, func(gtx C) D {
		return kit.Flex{Axis: layout.Vertical, Gap: kit.Space1}.Layout(gtx, th, children...)
	})
}

// Breadcrumbs is a location path that wraps whole segments when the width
// is short. Segments are ID/<crumb ID>.
type Breadcrumbs struct {
	ID   string
	Path []Breadcrumb
}

// Breadcrumb is one location. A nil Clickable marks the current location
// (plain text); earlier ones are Links.
type Breadcrumb struct {
	ID, Label string
	Clickable *kit.Clickable
}

func (b Breadcrumbs) Layout(gtx C, th *kit.Theme) D {
	items := make([]kit.W, 0, len(b.Path))
	for i, part := range b.Path {
		items = append(items, func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(func(gtx C) D {
					if i == 0 {
						return D{}
					}
					return layout.Inset{Right: th.Space(kit.Space1p5)}.Layout(gtx, func(gtx C) D {
						return kit.IconChevronRight(gtx, 12, th.Dim)
					})
				}),
				kit.Rigid(func(gtx C) D {
					if part.Clickable != nil {
						return Link{ID: kit.Part(b.ID, part.ID), Text: part.Label}.Layout(gtx, th, part.Clickable)
					}
					return kit.Group(gtx, kit.Part(b.ID, part.ID), part.Label, func(gtx C) D {
						// Match Link's inset and reserved underline so Flow's top
						// alignment keeps every crumb on the same text line.
						d := layout.UniformInset(th.Space(kit.Space1)).Layout(gtx, func(gtx C) D {
							gtx.Constraints.Min.X = 0
							return th.Text(gtx, part.Label, th.Type.Body, th.Fg, kit.SemiBold)
						})
						underline := max(1, gtx.Dp(th.Stroke))
						d.Size.Y += underline
						d.Baseline += underline
						return d
					})
				}),
			)
		})
	}
	kit.SetRole(gtx, b.ID, kit.RoleGroup)
	return kit.Group(gtx, b.ID, th.Strings.Breadcrumbs, func(gtx C) D {
		return kit.Flow(gtx, th.Space(kit.Space1p5), th.Space(kit.Space1), items...)
	})
}

// NavItem is a full-width sidebar or list entry with an optional trailing
// value. A click focuses it quietly (no ring); inside a FocusGroup the
// group takes focus, so arrows work right after.
type NavItem struct {
	ID         string
	Label      string
	Trailing   string
	TrailColor color.NRGBA // zero is Dim
	Active     bool        // the current page
}

func (n NavItem) Layout(gtx C, th *kit.Theme, c *kit.Clickable) D {
	c.FocusOnClick = true
	tc := n.TrailColor
	if tc.A == 0 {
		tc = th.Dim
	}
	return th.Press(gtx, c, n.ID, func(gtx C, st kit.State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(n.Label).Add(gtx.Ops)
		kit.SetRole(gtx, n.ID, kit.RoleLink)
		semantic.SelectedOp(n.Active).Add(gtx.Ops)
		bg, fg := kit.WithAlpha(th.Hover, uint8(255*st.Hover)), th.Muted
		if n.Active {
			bg, fg = th.Surface, th.Fg
		}
		s := th.Styled(kit.Surface{Bg: bg, Radius: th.Radius})
		if st.Focused {
			s.Focus = th.Focus
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return s.Layout(gtx, kit.Pad(th.ControlPad(kit.ControlDefault, 7), th.Space(kit.Space2p5), func(gtx C) D {
			return kit.Row(gtx,
				kit.Flexed(1, func(gtx C) D { return th.Text(gtx, n.Label, th.Type.Body, fg, kit.Regular) }),
				kit.Rigid(func(gtx C) D { return th.Text(gtx, n.Trailing, th.Type.Small, tc, kit.Medium) }),
			)
		}))
	})
}

// Stepper is "− value +" around a value you own; its buttons are ID/minus
// and ID/plus. Change the value when their Clickables report clicks.
type Stepper struct {
	ID         string
	Label      string // names the value in the buttons' spoken labels; default ID
	Value      string
	ValueColor color.NRGBA // zero is Fg
	Active     bool        // show the buttons at rest (its row is hovered)
}

// StepperState holds the two buttons.
type StepperState struct{ Minus, Plus kit.Clickable }

func (sp Stepper) Layout(gtx C, th *kit.Theme, s *StepperState) D {
	name, vc := sp.Label, sp.ValueColor
	if name == "" {
		name = sp.ID
	}
	if vc.A == 0 {
		vc = th.Fg
	}
	return kit.Row(gtx,
		kit.Rigid(func(gtx C) D {
			return IconButton{ID: kit.Part(sp.ID, "minus"), Icon: kit.IconMinus, Label: fmt.Sprintf(th.Strings.Decrease, name), Active: sp.Active}.Layout(gtx, th, &s.Minus)
		}),
		kit.Rigid(kit.Fixed(36, func(gtx C) D {
			return layout.Center.Layout(gtx, func(gtx C) D { return th.Text(gtx, sp.Value, th.Type.Text, vc, kit.SemiBold) })
		})),
		kit.Rigid(func(gtx C) D {
			return IconButton{ID: kit.Part(sp.ID, "plus"), Icon: kit.IconPlus, Label: fmt.Sprintf(th.Strings.Increase, name), Active: sp.Active}.Layout(gtx, th, &s.Plus)
		}),
	)
}

// EmptyState gives an empty view a title, explanation and optional action
// (a normal component, with its own focus and semantics).
type EmptyState struct {
	ID, Title, Body string
	Action          kit.W
}

func (es EmptyState) Layout(gtx C, th *kit.Theme) D {
	return kit.Group(gtx, es.ID, es.Title, func(gtx C) D {
		return layout.Center.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(th.ReadingWidth))
			return kit.Col(gtx,
				kit.Rigid(func(gtx C) D { return th.Text(gtx, es.Title, th.Type.Text, th.Fg, kit.SemiBold) }),
				kit.Rigid(kit.Gap(0, th.Space(kit.Space1p5))),
				kit.Rigid(func(gtx C) D { return th.Paragraph(gtx, es.Body, th.Muted) }),
				kit.Rigid(func(gtx C) D {
					if es.Action == nil {
						return D{}
					}
					return layout.Inset{Top: th.Space(kit.Space3)}.Layout(gtx, es.Action)
				}),
			)
		})
	})
}

// Collapsible is ready-made state for a Disclosure that keeps its own Open:
// call Update in the handle pass, lay out with Open and &Clickable.
type Collapsible struct {
	kit.Clickable
	Open bool
}

// Update flips Open when the header was clicked, and reports it.
func (c *Collapsible) Update(gtx C) bool {
	if c.Clicked(gtx) {
		c.Open = !c.Open
		return true
	}
	return false
}
