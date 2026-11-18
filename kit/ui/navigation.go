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
	"gioui.org/unit"

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
// Enter/Space re-pick the focused tab. Ids are ID/<item ID>.
//
//	if id, ok := v.tabs.Update(gtx); ok { v.tab = id }        // handle pass
//	ui.Tabs{ID: "view", Items: views, Selected: v.tab}.Layout(gtx, th, &v.tabs)
type Tabs struct {
	ID string // the tab list's id; tabs are ID/<item ID>
	// Items are the tabs: Label is shown and spoken, ID addresses the tab
	// and is what Update reports. IDs must be unique and stable; labels may
	// repeat or be translated.
	Items    []Choice
	Selected string   // the selected item's ID; "" (or an unknown ID) selects none
	Style    TabStyle // TabsUnderline (the zero value) or TabsSegmented
}

// TabsState is a Tabs' interaction state and animated indicator. The zero
// value is ready to use; do not copy it after first use.
type TabsState struct {
	group   kit.FocusGroup
	clicks  kit.Pool[kit.Clickable]
	x, w    kit.Tween
	last    []string // the item ids laid out last
	pending string   // a tab picked by the keyboard during layout, for Update
	has     bool
}

// Update returns the ID of a tab picked (clicked, or moved to with the
// keyboard) since the last call. A click on the selected tab reports it
// too. Keys are handled while Tabs lays out, so a keyboard pick is
// reported by the next frame's Update (Tabs asks for that frame). It
// reports one pick per call, the keyboard's first, and leaves further
// clicks for the next call.
func (s *TabsState) Update(gtx layout.Context) (string, bool) {
	if s.has {
		s.has = false
		return s.pending, true
	}
	for _, id := range s.last {
		if s.clicks.Get(id).Clicked(gtx) {
			return id, true
		}
	}
	return "", false
}

// Layout draws the tabs with the animated indicator under Selected and
// makes them one focus group. It handles the arrow, Home and End keys
// while laying out and queues the keyboard pick for s.Update, which also
// reports clicks.
func (tb Tabs) Layout(gtx layout.Context, th *kit.Theme, s *TabsState) layout.Dimensions {
	s.last = s.last[:0]
	selected := -1
	for i, it := range tb.Items {
		s.last = append(s.last, it.ID)
		s.group.Join(s.clicks.Get(it.ID), it.ID == tb.Selected)
		if it.ID == tb.Selected {
			selected = i
		}
	}
	if selected >= 0 {
		s.group.Active = tb.Selected
	}
	// cur is the tab the keyboard has moved to: several keys in one frame
	// each move on from the last.
	n, cur := len(tb.Items), selected
	pick := func(i int) {
		if n == 0 {
			return
		}
		cur = max(0, min(n-1, i))
		s.pending, s.has = tb.Items[cur].ID, true
		s.group.Active = s.pending
	}
	// Enter and Space re-pick the focused (selected) tab.
	for {
		e, ok := gtx.Event(key.Filter{Focus: s.group.FocusHandle(), Name: key.NameReturn},
			key.Filter{Focus: s.group.FocusHandle(), Name: key.NameEnter}, key.Filter{Focus: s.group.FocusHandle(), Name: key.NameSpace})
		if !ok {
			break
		}
		if ke, ok := e.(key.Event); ok && ke.State == key.Press && gtx.Enabled() && cur >= 0 {
			pick(cur)
			kit.Notify(gtx)
		}
	}
	return kit.KeyContext(gtx, "tabs", func(gtx layout.Context) layout.Dimensions {
		kit.OnAction(gtx, kit.ActCollapse, func() { pick(cur - 1) })
		kit.OnAction(gtx, kit.ActExpand, func() { pick(cur + 1) })
		kit.OnAction(gtx, kit.ActFirst, func() { pick(0) })
		kit.OnAction(gtx, kit.ActLast, func() { pick(n - 1) })
		return s.group.Layout(gtx, tb.ID, func(gtx layout.Context) layout.Dimensions {
			// The group's own area takes no pointer gestures, so Gio would drop
			// a tag on it: name it on a nested area.
			kit.SetRole(gtx, tb.ID, kit.RoleTabList)
			return kit.TagArea(gtx, tb.ID, func(gtx layout.Context) layout.Dimensions { return tb.draw(gtx, th, s, selected) })
		})
	})
}

// tabSpan is a laid-out tab's horizontal extent, where the indicator goes.
type tabSpan struct{ x, w int }

// tabHoverAlpha is the opacity of the Hover token behind a fully hovered,
// unselected text tab.
const tabHoverAlpha = 160

// tabPadY is a tab's vertical padding before density (see ControlPad).
const tabPadY unit.Dp = 5

// draw lays out the tabs and the indicator under the selected index (-1
// for none), animated toward it by s. Segmented tabs also get the track.
func (tb Tabs) draw(gtx layout.Context, th *kit.Theme, s *TabsState, selected int) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	seg := tb.Style == TabsSegmented
	pad := th.Space(kit.Space1p5)
	if seg {
		pad = th.Space(kit.Space1)
	}
	// Lay tabs out first (recorded) to learn their positions, then draw the
	// indicator under them and replay.
	m := op.Record(gtx.Ops)
	spans := make([]tabSpan, len(tb.Items))
	x, h := gtx.Dp(pad), 0
	for i, it := range tb.Items {
		id := kit.Part(tb.ID, it.ID)
		st := op.Offset(image.Pt(x, gtx.Dp(pad))).Push(gtx.Ops)
		d := th.Press(gtx, s.clicks.Get(it.ID), id, func(gtx layout.Context, st kit.State) layout.Dimensions {
			describe(gtx, id, kit.RoleTab, semantic.Button, it.Label)
			semantic.SelectedOp(i == selected).Add(gtx.Ops)
			fg := kit.LerpColor(th.Muted, th.Fg, st.Hover)
			if i == selected {
				fg = th.Fg
			}
			sf := th.Styled(kit.Surface{Radius: th.RadiusSm()})
			if st.Focused {
				sf.Focus = th.Focus
			}
			if !seg && i != selected {
				sf.Bg = kit.WithAlpha(th.Hover, uint8(tabHoverAlpha*st.Hover))
			}
			return sf.Layout(gtx, kit.Pad(th.ControlPad(kit.ControlDefault, tabPadY), th.Space(kit.Space3), func(gtx layout.Context) layout.Dimensions {
				return th.Text(gtx, it.Label, th.Type.Body, fg, kit.Medium)
			}))
		})
		st.Pop()
		spans[i] = tabSpan{x: x, w: d.Size.X}
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
	if selected >= 0 {
		sp := spans[selected]
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
	return layout.Dimensions{Size: size}
}

// Disclosure is a section with a clickable header (chevron, title, an
// optional trailing count) that shows or hides Body. Flip Open when the
// Clickable reports a click (Enter/Space on the header too). The header id
// is ID.
//
//	if v.more.Clicked(gtx) { v.moreOpen = !v.moreOpen }
//	ui.Disclosure{ID: "more", Title: "More", Open: v.moreOpen, Body: body}.Layout(gtx, th, &v.more)
type Disclosure struct {
	ID       string        // the header's id
	Title    string        // the header's visible and spoken text
	Trailing string        // a count or summary pill after the title
	Open     bool          // the controlled value: whether Body shows
	Body     layout.Widget // laid out only while Open
}

// Layout draws the header for c's state and, while Open, Body below it. It
// registers c as the header's press and focus target, with its semantic
// selected state following Open. It does not flip Open: read c.Clicked.
func (dc Disclosure) Layout(gtx layout.Context, th *kit.Theme, c *kit.Clickable) layout.Dimensions {
	return kit.Col(gtx,
		kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			return th.Press(gtx, c, dc.ID, func(gtx layout.Context, st kit.State) layout.Dimensions {
				describe(gtx, dc.ID, kit.RoleButton, semantic.Button, dc.Title)
				semantic.SelectedOp(dc.Open).Add(gtx.Ops)
				s := th.Styled(kit.Surface{Bg: kit.WithAlpha(th.Hover, uint8(255*st.Hover)), Radius: th.Radius})
				if st.Focused {
					s.Focus = th.Focus
				}
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return s.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: th.Space(kit.Space1p5), Bottom: th.Space(kit.Space1p5), Left: th.Space(kit.Space1), Right: th.Space(kit.Space2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						chev := kit.IconChevronRight
						if dc.Open {
							chev = kit.IconChevronDown
						}
						return kit.Row(gtx,
							kit.Rigid(func(gtx layout.Context) layout.Dimensions { return chev(gtx, th.IconSize, th.Muted) }),
							kit.Rigid(kit.Gap(th.Space(kit.Space1p5), 0)),
							kit.Rigid(func(gtx layout.Context) layout.Dimensions { return th.Caption(gtx, dc.Title) }),
							kit.Rigid(kit.Gap(th.Space(kit.Space2), 0)),
							kit.Rigid(func(gtx layout.Context) layout.Dimensions {
								if dc.Trailing == "" {
									return layout.Dimensions{}
								}
								return th.Pill(gtx, dc.Trailing, th.Muted, th.Surface)
							}),
						)
					})
				})
			})
		}),
		kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !dc.Open || dc.Body == nil {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: th.Space(kit.Space2)}.Layout(gtx, dc.Body)
		}),
	)
}

// Accordion is a stack of Disclosures with at most one open. Headers are
// ID/<section ID>.
type Accordion struct {
	ID       string             // the group's id; headers are ID/<section ID>
	Sections []AccordionSection // top to bottom
	Open     string             // the open section's ID; "" for none
}

// AccordionSection is one section. IDs must stay stable when sections are
// reordered. Body is laid out only while the section is open.
type AccordionSection struct {
	// ID is the section's stable id, Title its header text and Summary an
	// optional pill after the title (Disclosure.Trailing).
	ID, Title, Summary string
	Body               layout.Widget // the section's content
}

// AccordionState holds the section headers, one per section ID. The zero
// value is ready to use; do not copy it after first use.
type AccordionState struct {
	headers kit.Pool[kit.Clickable]
	last    []string
	open    string
}

// Update returns the section to show open (or "" to close all) after a
// header was clicked since the last call.
func (s *AccordionState) Update(gtx layout.Context) (string, bool) {
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

// Layout draws one Disclosure per section, Open's section expanded, inside
// a named group. It remembers the sections and Open for s.Update, which
// reports header clicks.
func (a Accordion) Layout(gtx layout.Context, th *kit.Theme, s *AccordionState) layout.Dimensions {
	s.open = a.Open
	s.last = s.last[:0]
	children := make([]layout.FlexChild, 0, len(a.Sections))
	for _, sec := range a.Sections {
		s.last = append(s.last, sec.ID)
		children = append(children, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			return Disclosure{ID: kit.Part(a.ID, sec.ID), Title: sec.Title, Trailing: sec.Summary, Open: a.Open == sec.ID, Body: sec.Body}.Layout(gtx, th, s.headers.Get(sec.ID))
		}))
	}
	kit.SetRole(gtx, a.ID, kit.RoleGroup)
	return kit.Group(gtx, a.ID, th.Strings.Accordion, func(gtx layout.Context) layout.Dimensions {
		return kit.Flex{Axis: layout.Vertical, Gap: kit.Space1}.Layout(gtx, th, children...)
	})
}

// Breadcrumbs is a location path that wraps whole segments when the width
// is short. The last crumb is the current location, drawn as plain text;
// the earlier ones are Links, reported by BreadcrumbsState.Update.
// Segments are ID/<crumb ID>.
type Breadcrumbs struct {
	ID   string       // the group's id; segments are ID/<crumb ID>
	Path []Breadcrumb // from the root to the current location
}

// Breadcrumb is one location. IDs must be unique within the path.
type Breadcrumb struct {
	ID, Label string // the crumb's stable id and visible text
}

// BreadcrumbsState holds the links, one per crumb ID. The zero value is
// ready to use; do not copy it after first use.
type BreadcrumbsState struct {
	links kit.Pool[kit.Clickable]
	last  []string // the linked crumb ids laid out last
}

// Update returns the ID of a crumb clicked since the last call. Call it
// in the handle pass.
func (s *BreadcrumbsState) Update(gtx layout.Context) (string, bool) {
	for _, id := range s.last {
		if s.links.Get(id).Clicked(gtx) {
			return id, true
		}
	}
	return "", false
}

// Layout draws the crumbs with separators, wrapping whole segments, the
// last as plain text and the others as Links from s. It remembers the
// linked crumbs for s.Update, which reports clicks.
func (b Breadcrumbs) Layout(gtx layout.Context, th *kit.Theme, s *BreadcrumbsState) layout.Dimensions {
	s.last = s.last[:0]
	items := make([]layout.Widget, 0, len(b.Path))
	for i, part := range b.Path {
		var link *kit.Clickable
		if i < len(b.Path)-1 {
			s.last = append(s.last, part.ID)
			link = s.links.Get(part.ID)
		}
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return kit.Row(gtx,
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					if i == 0 {
						return layout.Dimensions{}
					}
					return layout.Inset{Right: th.Space(kit.Space1p5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return kit.IconChevronRight(gtx, th.IconSize, th.Dim)
					})
				}),
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					if link != nil {
						return Link{ID: kit.Part(b.ID, part.ID), Text: part.Label}.Layout(gtx, th, link)
					}
					return kit.Group(gtx, kit.Part(b.ID, part.ID), part.Label, func(gtx layout.Context) layout.Dimensions {
						// Match Link's inset and reserved underline so Flow's top
						// alignment keeps every crumb on the same text line.
						d := layout.UniformInset(th.Space(kit.Space1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
	s.links.Sweep()
	kit.SetRole(gtx, b.ID, kit.RoleGroup)
	return kit.Group(gtx, b.ID, th.Strings.Breadcrumbs, func(gtx layout.Context) layout.Dimensions {
		return kit.Flow(gtx, th.Space(kit.Space1p5), th.Space(kit.Space1), items...)
	})
}

// NavItem is a full-width sidebar or list entry with an optional trailing
// value. A click focuses it quietly (no ring); inside a FocusGroup the
// group takes focus, so arrows work right after. Layout sets the
// Clickable's FocusOnClick: the Clickable belongs to this NavItem.
type NavItem struct {
	ID         string      // the entry's semantic id
	Label      string      // the visible and spoken text
	Trailing   string      // an optional value at the right edge (a count)
	TrailColor color.NRGBA // zero is Dim
	Active     bool        // the current page
}

// Layout draws the entry at the full available width, highlighted while
// Active or hovered, and registers c as its press and focus target. Read
// c.Clicked to navigate.
func (n NavItem) Layout(gtx layout.Context, th *kit.Theme, c *kit.Clickable) layout.Dimensions {
	c.FocusOnClick = true
	tc := n.TrailColor
	if tc.A == 0 {
		tc = th.Dim
	}
	return th.Press(gtx, c, n.ID, func(gtx layout.Context, st kit.State) layout.Dimensions {
		describe(gtx, n.ID, kit.RoleLink, semantic.Button, n.Label)
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
		return s.Layout(gtx, kit.Pad(th.FieldPad(kit.ControlDefault), th.Space(kit.Space2p5), func(gtx layout.Context) layout.Dimensions {
			return kit.Row(gtx,
				kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return th.Text(gtx, n.Label, th.Type.Body, fg, kit.Regular)
				}),
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return th.Text(gtx, n.Trailing, th.Type.Small, tc, kit.Medium)
				}),
			)
		}))
	})
}

// Stepper is "− value +" around a value you own; its buttons are ID/minus
// and ID/plus. Change the value when their Clickables report clicks.
type Stepper struct {
	ID         string      // the stepper's id; buttons are ID/minus and ID/plus
	Label      string      // names the value in the buttons' spoken labels ("Increase %s"); required
	Value      string      // the value, formatted by the caller
	ValueColor color.NRGBA // zero is Fg
	Active     bool        // show the buttons at rest (its row is hovered)
}

// stepperValueWidth is the room for a Stepper's value between its buttons.
const stepperValueWidth unit.Dp = 36

// StepperState holds the two buttons: read Minus.Clicked and Plus.Clicked
// in the handle pass. The zero value is ready to use; do not copy it after
// first use.
type StepperState struct {
	Minus, Plus kit.Clickable // the − and + buttons
}

// Layout draws "− value +" with the value centred in a fixed width. It
// registers the two buttons and changes nothing itself.
func (sp Stepper) Layout(gtx layout.Context, th *kit.Theme, s *StepperState) layout.Dimensions {
	name, vc := sp.Label, sp.ValueColor
	if vc.A == 0 {
		vc = th.Fg
	}
	return kit.Row(gtx,
		kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			return IconButton{ID: kit.Part(sp.ID, "minus"), Icon: kit.IconMinus, Label: fmt.Sprintf(th.Strings.Decrease, name), Active: sp.Active}.Layout(gtx, th, &s.Minus)
		}),
		kit.Rigid(kit.Fixed(stepperValueWidth, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return th.Text(gtx, sp.Value, th.Type.Text, vc, kit.SemiBold)
			})
		})),
		kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			return IconButton{ID: kit.Part(sp.ID, "plus"), Icon: kit.IconPlus, Label: fmt.Sprintf(th.Strings.Increase, name), Active: sp.Active}.Layout(gtx, th, &s.Plus)
		}),
	)
}

// EmptyState gives an empty view a title, explanation and optional action
// (a normal component, with its own focus and semantics).
type EmptyState struct {
	// ID is the group's id, Title its name and heading, Body the
	// explanation under it.
	ID, Title, Body string
	// Action, if set, draws a control below (a Button), with its own state.
	Action layout.Widget
}

// Layout draws the message centred, at most th.ReadingWidth wide, inside a
// group named by Title. It handles no input; Action handles its own.
func (es EmptyState) Layout(gtx layout.Context, th *kit.Theme) layout.Dimensions {
	kit.SetRole(gtx, es.ID, kit.RoleGroup)
	return kit.Group(gtx, es.ID, es.Title, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(th.ReadingWidth))
			return kit.Col(gtx,
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return th.Text(gtx, es.Title, th.Type.Text, th.Fg, kit.SemiBold)
				}),
				kit.Rigid(kit.Gap(0, th.Space(kit.Space1p5))),
				kit.Rigid(func(gtx layout.Context) layout.Dimensions { return th.Paragraph(gtx, es.Body, th.Muted) }),
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					if es.Action == nil {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: th.Space(kit.Space3)}.Layout(gtx, es.Action)
				}),
			)
		})
	})
}

// Collapsible is ready-made state for a Disclosure that keeps its own Open:
// call Update in the handle pass, lay out with Open and &Clickable. The
// zero value is closed; it embeds an event tag, so do not copy it after
// first use.
type Collapsible struct {
	kit.Clickable      // the header's press target
	Open          bool // whether the Disclosure shows its body
}

// Update flips Open when the header was clicked, and reports it.
func (c *Collapsible) Update(gtx layout.Context) bool {
	if c.Clicked(gtx) {
		c.Open = !c.Open
		return true
	}
	return false
}
