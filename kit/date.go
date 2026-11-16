package kit

import (
	"image"
	"strconv"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
)

// DatePicker is the state of a date field with a calendar popup
// (th.DateField). Weeks start on Monday unless SundayFirst.
type DatePicker struct {
	Popup
	Trigger     Clickable
	SundayFirst bool   // weeks start on Sunday instead of Monday
	Format      string // field label layout; default "Mon, Jan 2, 2006"

	shown      time.Time // first of the month on display
	days       Pool[Clickable]
	prev, next Clickable
}

const dayKey = "2006-01-02"

// Changed returns a date picked since the last call, and toggles the
// calendar when the field is clicked. Call it in your handle pass. A pick
// closes the calendar and returns focus to the field.
func (d *DatePicker) Changed(gtx C) (time.Time, bool) {
	if d.Trigger.Clicked(gtx) {
		d.Toggle()
	}
	if d.prev.Clicked(gtx) {
		d.shown = d.shown.AddDate(0, -1, 0)
	}
	if d.next.Clicked(gtx) {
		d.shown = d.shown.AddDate(0, 1, 0)
	}
	var picked time.Time
	ok := false
	d.days.Each(func(k string, c *Clickable) {
		if c.Clicked(gtx) {
			if t, err := time.ParseInLocation(dayKey, k, time.Local); err == nil {
				picked, ok = t, true
			}
		}
	})
	if ok {
		d.Hide()
		d.Trigger.FocusQuiet(gtx)
	}
	return picked, ok
}

// DateField is a field showing value (a zero value shows placeholder) that
// opens a month calendar. Ids: <id>/field, days <id>/YYYY-MM-DD,
// <id>/prev, <id>/next; the calendar panel is id. Keyboard: arrows move
// by day and week, PageUp/PageDown by month, Enter picks, Escape closes.
func (t *Theme) DateField(gtx C, d *DatePicker, id string, value time.Time, placeholder string) D {
	sel := day(value)
	if d.focusNext || d.shown.IsZero() {
		base := sel
		if base.IsZero() {
			base = day(gtx.Now)
		}
		d.shown = time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, time.Local)
		if d.focusNext {
			defer d.days.Get(base.Format(dayKey)).FocusQuiet(gtx)
		}
	}
	if dm, byKey := d.DidDismiss(); dm {
		if byKey {
			d.Trigger.Focus(gtx)
		} else {
			d.Trigger.FocusQuiet(gtx)
		}
	}
	label, lc := placeholder, t.Dim
	if !sel.IsZero() {
		f := d.Format
		if f == "" {
			f = "Mon, Jan 2, 2006"
		}
		label, lc = sel.Format(f), t.Fg
	}
	return t.Popover(gtx, &d.Popup, id, func(gtx C) D {
		return t.press(gtx, &d.Trigger, Part(id, "field"), func(gtx C, st State) D {
			semantic.Button.Add(gtx.Ops)
			s := t.Styled(Surface{Bg: t.tint(t.Surface, PressTint/2*st.Hover), Border: t.Border, Radius: t.Radius})
			if st.Focused || d.open {
				s.Focus = t.Focus
			}
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return s.Layout(gtx, func(gtx C) D {
				return layout.Inset{Top: t.control(7), Bottom: t.control(7), Left: t.Space(10), Right: t.Space(8)}.Layout(gtx, func(gtx C) D {
					return Row(gtx,
						Flexed(1, func(gtx C) D { return t.Text(gtx, label, t.Type.Body, lc, Regular) }),
						Rigid(func(gtx C) D { return IconChevronDown(gtx, 14, t.Muted) }),
					)
				})
			})
		})
	}, func(gtx C) D { return t.calendar(gtx, d, id, sel) })
}

func day(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	t = t.Local()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// calendar lays out the month grid and moves focus with the keyboard.
func (t *Theme) calendar(gtx C, d *DatePicker, id string, sel time.Time) D {
	first := d.shown
	weekStart := time.Monday
	if d.SundayFirst {
		weekStart = time.Sunday
	}
	lead := (int(first.Weekday()) - int(weekStart) + 7) % 7
	start := first.AddDate(0, 0, -lead)
	cells := make([]time.Time, 42)
	for i := range cells {
		cells[i] = start.AddDate(0, 0, i)
	}
	// Keyboard: move focus between days, paging months as needed (before
	// drawing, so the new month draws this frame).
	for _, c := range cells {
		cl := d.days.Get(c.Format(dayKey))
		for {
			e, ok := gtx.Event(
				key.Filter{Focus: cl, Name: key.NameLeftArrow}, key.Filter{Focus: cl, Name: key.NameRightArrow},
				key.Filter{Focus: cl, Name: key.NameUpArrow}, key.Filter{Focus: cl, Name: key.NameDownArrow},
				key.Filter{Focus: cl, Name: key.NamePageUp}, key.Filter{Focus: cl, Name: key.NamePageDown},
			)
			if !ok {
				break
			}
			ke, ok := e.(key.Event)
			if !ok || ke.State != key.Press {
				continue
			}
			to := c
			switch ke.Name {
			case key.NameLeftArrow:
				to = c.AddDate(0, 0, -1)
			case key.NameRightArrow:
				to = c.AddDate(0, 0, 1)
			case key.NameUpArrow:
				to = c.AddDate(0, 0, -7)
			case key.NameDownArrow:
				to = c.AddDate(0, 0, 7)
			case key.NamePageUp:
				to = c.AddDate(0, -1, 0)
			case key.NamePageDown:
				to = c.AddDate(0, 1, 0)
			}
			if to.Month() != d.shown.Month() || to.Year() != d.shown.Year() {
				d.shown = time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.Local)
			}
			d.days.Get(to.Format(dayKey)).Focus(gtx)
			Notify(gtx)
		}
	}
	if d.shown != first { // paged: lay out the new month
		return t.calendar(gtx, d, id, sel)
	}

	today := day(gtx.Now)
	cell := image.Pt(gtx.Dp(34), gtx.Dp(30))
	return layout.UniformInset(t.Space(8)).Layout(gtx, func(gtx C) D {
		rows := []layout.FlexChild{
			Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = cell.X * 7
				gtx.Constraints.Max.X = cell.X * 7
				return Row(gtx,
					Rigid(func(gtx C) D { return t.IconButton(gtx, &d.prev, Part(id, "prev"), IconChevronLeft, true) }),
					Flexed(1, func(gtx C) D {
						return layout.Center.Layout(gtx, func(gtx C) D {
							return t.Text(gtx, first.Format("January 2006"), t.Type.Body, t.Fg, SemiBold)
						})
					}),
					Rigid(func(gtx C) D { return t.IconButton(gtx, &d.next, Part(id, "next"), IconChevronRight, true) }),
				)
			}),
			Rigid(Gap(0, 6)),
			Rigid(func(gtx C) D {
				var hs []layout.FlexChild
				for i := range 7 {
					name := time.Weekday((int(weekStart) + i) % 7).String()[:2]
					hs = append(hs, Rigid(func(gtx C) D {
						gtx.Constraints = layout.Exact(image.Pt(cell.X, gtx.Dp(20)))
						return layout.Center.Layout(gtx, func(gtx C) D { return t.Text(gtx, name, t.Type.Small, t.Muted, Medium) })
					}))
				}
				return layout.Flex{}.Layout(gtx, hs...)
			}),
		}
		for w := range 6 {
			week := cells[w*7 : w*7+7]
			rows = append(rows, Rigid(func(gtx C) D {
				var ds []layout.FlexChild
				for _, c := range week {
					ds = append(ds, Rigid(func(gtx C) D { return t.dayCell(gtx, d, id, c, cell, c.Equal(sel), c.Equal(today)) }))
				}
				return layout.Flex{}.Layout(gtx, ds...)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

func (t *Theme) dayCell(gtx C, d *DatePicker, id string, c time.Time, sz image.Point, selected, today bool) D {
	gtx.Constraints = layout.Exact(sz)
	return t.press(gtx, d.days.Get(c.Format(dayKey)), Part(id, c.Format(dayKey)), func(gtx C, st State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.SelectedOp(selected).Add(gtx.Ops)
		fg := t.Fg
		if c.Month() != d.shown.Month() {
			fg = t.Dim
		}
		s := t.Styled(Surface{Bg: WithAlpha(t.tint(t.Overlay, PressTint), uint8(255*st.Hover)), Radius: t.RadiusSm()})
		switch {
		case selected:
			s.Bg, fg = t.Accent, t.OnAccent
		case today:
			s.Border = t.Border
		}
		if st.Focused {
			s.Focus = t.Focus
		}
		return s.Layout(gtx, func(gtx C) D {
			return layout.Center.Layout(gtx, func(gtx C) D {
				w := Regular
				if today || selected {
					w = SemiBold
				}
				return t.Text(gtx, strconv.Itoa(c.Day()), t.Type.Body, fg, w)
			})
		})
	})
}
