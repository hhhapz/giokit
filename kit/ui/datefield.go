package ui

import (
	"image"
	"strconv"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
)

// DateField is a field showing Value (zero shows Placeholder) that opens a
// calendar. It is controlled: set Value from what Update returns.
//
// The calendar has three views. The title (ID/view) zooms out from days to
// months to years; picking a year zooms into its months, a month into its
// days. Prev/next (ID/prev, ID/next) step by a month, a year or a decade.
// Ids: days ID/YYYY-MM-DD, months ID/YYYY-MM, years ID/YYYY; the panel is
// ID, the field ID/field. Keyboard: arrows move within the view (by day and
// week, or across the 3-column month and year grids), PageUp/PageDown step
// like prev/next, Enter picks, Escape closes.
type DateField struct {
	ID          string
	Value       time.Time
	Placeholder string
	Format      string // field text layout; default "Mon, Jan 2, 2006"
	SundayFirst bool   // weeks start on Sunday instead of Monday
}

// DateFieldState is a DateField's interaction state.
type DateFieldState struct {
	kit.Popup
	Trigger kit.Clickable

	view       calView
	shown      time.Time // first of the month on display (its year in the year views)
	days       kit.Pool[kit.Clickable]
	months     kit.Pool[kit.Clickable]
	years      kit.Pool[kit.Clickable]
	head       kit.Clickable
	prev, next kit.Clickable
	focus      *kit.Clickable // focused after this layout (cells exist by then)
}

type calView int

const (
	viewDays calView = iota
	viewMonths
	viewYears
)

const (
	dayKey   = "2006-01-02"
	monthKey = "2006-01"
	gridCols = 3 // month and year grids: 3 × 4
)

// Update returns a date picked since the last call, and handles the
// calendar's navigation. Call it in the handle pass. A pick closes the
// calendar and returns focus to the field.
func (s *DateFieldState) Update(gtx C) (time.Time, bool) {
	if s.Trigger.Clicked(gtx) {
		s.Toggle()
	}
	if s.head.Clicked(gtx) && s.view < viewYears {
		s.view++
		s.focus = s.cell(s.shown)
	}
	years, months := 0, 1 // prev/next step: a month, a year or a decade
	switch s.view {
	case viewMonths:
		years, months = 1, 0
	case viewYears:
		years, months = 10, 0
	}
	if s.prev.Clicked(gtx) {
		s.shown = s.shown.AddDate(-years, -months, 0)
	}
	if s.next.Clicked(gtx) {
		s.shown = s.shown.AddDate(years, months, 0)
	}
	s.years.Each(func(k string, c *kit.Clickable) {
		if !c.Clicked(gtx) {
			return
		}
		if y, err := strconv.Atoi(k); err == nil {
			s.shown = time.Date(y, s.shown.Month(), 1, 0, 0, 0, 0, time.Local)
			s.view = viewMonths
			s.focus = s.cell(s.shown)
		}
	})
	s.months.Each(func(k string, c *kit.Clickable) {
		if !c.Clicked(gtx) {
			return
		}
		if m, err := time.ParseInLocation(monthKey, k, time.Local); err == nil {
			s.shown = m
			s.view = viewDays
			s.focus = s.days.Get(m.Format(dayKey))
		}
	})
	var picked time.Time
	ok := false
	s.days.Each(func(k string, c *kit.Clickable) {
		if c.Clicked(gtx) {
			if t, err := time.ParseInLocation(dayKey, k, time.Local); err == nil {
				picked, ok = t, true
			}
		}
	})
	if ok {
		s.Hide()
		s.Trigger.FocusQuiet(gtx)
	}
	return picked, ok
}

// cell is the month or year cell for t in the current view.
func (s *DateFieldState) cell(t time.Time) *kit.Clickable {
	if s.view == viewYears {
		return s.years.Get(strconv.Itoa(t.Year()))
	}
	return s.months.Get(t.Format(monthKey))
}

// Layout lays out the field and, while open, the calendar.
func (f DateField) Layout(gtx C, th *kit.Theme, s *DateFieldState) D {
	sel := day(f.Value)
	if s.Opening() || s.shown.IsZero() {
		base := sel
		if base.IsZero() {
			base = day(gtx.Now)
		}
		s.shown = time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, time.Local)
		s.view = viewDays
		if s.Opening() {
			s.focus = s.days.Get(base.Format(dayKey))
		}
	}
	if s.focus != nil {
		defer func(c *kit.Clickable) { c.FocusQuiet(gtx) }(s.focus)
		s.focus = nil
	}
	if dm, byKey := s.DidDismiss(); dm {
		if byKey {
			s.Trigger.Focus(gtx)
		} else {
			s.Trigger.FocusQuiet(gtx)
		}
	}
	label, lc := f.Placeholder, th.Dim
	if !sel.IsZero() {
		layoutStr := f.Format
		if layoutStr == "" {
			layoutStr = th.Strings.DateFormat
		}
		label, lc = sel.Format(layoutStr), th.Fg
	}
	return th.Popover(gtx, &s.Popup, f.ID, func(gtx C) D {
		return th.Press(gtx, &s.Trigger, kit.Part(f.ID, "field"), func(gtx C, st kit.State) D {
			semantic.Button.Add(gtx.Ops)
			st.Focused = st.Focused || s.Opened()
			return th.Field(gtx, st, func(gtx C) D {
				return kit.Row(gtx,
					kit.Flexed(1, func(gtx C) D { return th.Text(gtx, label, th.Type.Body, lc, kit.Regular) }),
					kit.Rigid(func(gtx C) D { return kit.IconChevronDown(gtx, th.IconSize, th.Muted) }),
				)
			})
		})
	}, func(gtx C) D { return f.calendar(gtx, th, s, sel) })
}

func day(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	t = t.Local()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// decade is the first year of the decade holding y.
func decade(y int) int { return y - ((y%10)+10)%10 }

// calendar handles the current view's keys (before drawing, so a page
// lands this frame), then lays out the header and the view's grid.
func (f DateField) calendar(gtx C, th *kit.Theme, s *DateFieldState, sel time.Time) D {
	switch s.view {
	case viewDays:
		f.dayKeys(gtx, s)
	case viewMonths:
		gridKeys(gtx, 12, func(i int) *kit.Clickable {
			return s.months.Get(time.Date(s.shown.Year(), time.Month(i+1), 1, 0, 0, 0, 0, time.Local).Format(monthKey))
		}, func(i, to int) {
			t := time.Date(s.shown.Year(), time.Month(to+1), 1, 0, 0, 0, 0, time.Local)
			s.shown = t
			s.months.Get(t.Format(monthKey)).Focus(gtx)
		})
	case viewYears:
		first := decade(s.shown.Year()) - 1
		gridKeys(gtx, 12, func(i int) *kit.Clickable {
			return s.years.Get(strconv.Itoa(first + i))
		}, func(i, to int) {
			y := first + to
			s.shown = time.Date(y, s.shown.Month(), 1, 0, 0, 0, 0, time.Local)
			s.years.Get(strconv.Itoa(y)).Focus(gtx)
		})
	}

	cell := image.Pt(gtx.Dp(34), gtx.Dp(30))
	weekdays := gtx.Dp(20)                      // the weekday names row
	gridW, gridH := cell.X*7, weekdays+cell.Y*6 // the day grid's size: every view keeps it
	var title string
	switch s.view {
	case viewDays:
		title = s.shown.Format(th.Strings.MonthFormat)
	case viewMonths:
		title = strconv.Itoa(s.shown.Year())
	default:
		d := decade(s.shown.Year())
		title = strconv.Itoa(d) + "–" + strconv.Itoa(d+9)
	}
	header := func(gtx C) D {
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = gridW, gridW
		return kit.Row(gtx,
			kit.Rigid(func(gtx C) D {
				return IconButton{ID: kit.Part(f.ID, "prev"), Icon: kit.IconChevronLeft, Label: th.Strings.PrevPage, Active: true}.Layout(gtx, th, &s.prev)
			}),
			kit.Flexed(1, func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D { return f.viewButton(gtx, th, s, title) })
			}),
			kit.Rigid(func(gtx C) D {
				return IconButton{ID: kit.Part(f.ID, "next"), Icon: kit.IconChevronRight, Label: th.Strings.NextPage, Active: true}.Layout(gtx, th, &s.next)
			}),
		)
	}
	body := func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(gridW, gridH))
		switch s.view {
		case viewMonths:
			return f.grid(gtx, th, 12, func(gtx C, i int) D {
				m := time.Date(s.shown.Year(), time.Month(i+1), 1, 0, 0, 0, 0, time.Local)
				key := m.Format(monthKey)
				return f.gridCell(gtx, th, s.months.Get(key), kit.Part(f.ID, key), m.Month().String()[:3],
					sameMonth(m, sel), sameMonth(m, gtx.Now), false)
			})
		case viewYears:
			first := decade(s.shown.Year()) - 1
			return f.grid(gtx, th, 12, func(gtx C, i int) D {
				y := first + i
				key := strconv.Itoa(y)
				return f.gridCell(gtx, th, s.years.Get(key), kit.Part(f.ID, key), key,
					!sel.IsZero() && sel.Year() == y, gtx.Now.Year() == y, i == 0 || i == 11)
			})
		}
		return f.dayGrid(gtx, th, s, sel, cell, weekdays)
	}
	return kit.Flex{Axis: layout.Vertical, Pad: kit.Space2, Gap: kit.Space1p5}.Layout(gtx, th, kit.Rigid(header), kit.Rigid(body))
}

func sameMonth(a, b time.Time) bool {
	return !b.IsZero() && a.Year() == b.Year() && a.Month() == b.Month()
}

// viewButton is the title: it zooms out to the next coarser view.
func (f DateField) viewButton(gtx C, th *kit.Theme, s *DateFieldState, title string) D {
	return th.Press(gtx, &s.head, kit.Part(f.ID, "view"), func(gtx C, st kit.State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(title).Add(gtx.Ops)
		sf := th.Styled(kit.Surface{Bg: th.Highlight(st.Hover), Radius: th.RadiusSm()})
		if st.Focused {
			sf.Focus = th.Focus
		}
		return sf.Layout(gtx, func(gtx C) D {
			return kit.Flex{Gap: kit.Space1, Align: layout.Middle, PadX: kit.Space2, PadY: kit.Space1}.Layout(gtx, th,
				kit.Rigid(func(gtx C) D { return th.Text(gtx, title, th.Type.Body, th.Fg, kit.SemiBold) }),
				kit.Rigid(func(gtx C) D {
					if s.view == viewYears {
						return D{}
					}
					return kit.IconChevronDown(gtx, 12, th.Muted)
				}),
			)
		})
	})
}

// dayKeys moves focus between days, paging months as needed.
func (f DateField) dayKeys(gtx C, s *DateFieldState) {
	for _, c := range f.cells(s) {
		cl := s.days.Get(c.Format(dayKey))
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
			if !sameMonth(to, s.shown) {
				s.shown = time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.Local)
			}
			s.days.Get(to.Format(dayKey)).Focus(gtx)
			kit.Notify(gtx)
			return // the grid changed under the loop: finish next frame
		}
	}
}

// gridKeys moves focus in a 3-column grid of n cells: arrows by one or by
// a row, PageUp/PageDown by a whole grid. to may fall outside 0..n-1; move
// pages the view so it exists.
func gridKeys(gtx C, n int, cell func(i int) *kit.Clickable, move func(i, to int)) {
	for i := range n {
		cl := cell(i)
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
			d := map[key.Name]int{
				key.NameLeftArrow: -1, key.NameRightArrow: 1,
				key.NameUpArrow: -gridCols, key.NameDownArrow: gridCols,
				key.NamePageUp: -n, key.NamePageDown: n,
			}[ke.Name]
			move(i, i+d)
			kit.Notify(gtx)
			return
		}
	}
}

// cells are the 42 days the month grid shows, from the week start.
func (f DateField) cells(s *DateFieldState) []time.Time {
	weekStart := time.Monday
	if f.SundayFirst {
		weekStart = time.Sunday
	}
	lead := (int(s.shown.Weekday()) - int(weekStart) + 7) % 7
	start := s.shown.AddDate(0, 0, -lead)
	cells := make([]time.Time, 42)
	for i := range cells {
		cells[i] = start.AddDate(0, 0, i)
	}
	return cells
}

func (f DateField) dayGrid(gtx C, th *kit.Theme, s *DateFieldState, sel time.Time, cell image.Point, headH int) D {
	weekStart := time.Monday
	if f.SundayFirst {
		weekStart = time.Sunday
	}
	cells := f.cells(s)
	today := day(gtx.Now)
	rows := []layout.FlexChild{kit.Rigid(func(gtx C) D {
		var hs []layout.FlexChild
		for i := range 7 {
			name := time.Weekday((int(weekStart) + i) % 7).String()[:2]
			hs = append(hs, kit.Rigid(func(gtx C) D {
				gtx.Constraints = layout.Exact(image.Pt(cell.X, headH))
				return layout.Center.Layout(gtx, func(gtx C) D { return th.Text(gtx, name, th.Type.Small, th.Muted, kit.Medium) })
			}))
		}
		return layout.Flex{}.Layout(gtx, hs...)
	})}
	for w := range 6 {
		week := cells[w*7 : w*7+7]
		rows = append(rows, kit.Rigid(func(gtx C) D {
			var ds []layout.FlexChild
			for _, c := range week {
				ds = append(ds, kit.Rigid(func(gtx C) D {
					gtx.Constraints = layout.Exact(cell)
					key := c.Format(dayKey)
					return f.gridCell(gtx, th, s.days.Get(key), kit.Part(f.ID, key), strconv.Itoa(c.Day()),
						c.Equal(sel), c.Equal(today), c.Month() != s.shown.Month())
				}))
			}
			return layout.Flex{}.Layout(gtx, ds...)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

// grid lays out n cells in rows of gridCols, filling the constraints.
func (f DateField) grid(gtx C, th *kit.Theme, n int, cell func(gtx C, i int) D) D {
	rowsN := (n + gridCols - 1) / gridCols
	sz := image.Pt(gtx.Constraints.Max.X/gridCols, gtx.Constraints.Max.Y/rowsN)
	var rows []layout.FlexChild
	for r := range rowsN {
		rows = append(rows, kit.Rigid(func(gtx C) D {
			var cs []layout.FlexChild
			for c := range gridCols {
				i := r*gridCols + c
				if i >= n {
					break
				}
				cs = append(cs, kit.Rigid(func(gtx C) D {
					gtx.Constraints = layout.Exact(sz)
					return layout.UniformInset(th.Space(kit.Space0p5)).Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min = gtx.Constraints.Max
						return cell(gtx, i)
					})
				}))
			}
			return layout.Flex{}.Layout(gtx, cs...)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

// gridCell is one day, month or year: selected is accent-filled, current
// (today, this month, this year) outlined, outside (another month, a
// neighbouring decade) dimmed.
func (f DateField) gridCell(gtx C, th *kit.Theme, c *kit.Clickable, id, label string, selected, current, outside bool) D {
	return th.Press(gtx, c, id, func(gtx C, st kit.State) D {
		semantic.Button.Add(gtx.Ops)
		semantic.SelectedOp(selected).Add(gtx.Ops)
		fg := th.Fg
		if outside {
			fg = th.Dim
		}
		sf := th.Styled(kit.Surface{Bg: th.Highlight(st.Hover), Radius: th.RadiusSm()})
		switch {
		case selected:
			sf.Bg, fg = th.Accent, th.OnAccent
		case current:
			sf.Border = th.Border
		}
		if st.Focused {
			sf.Focus = th.Focus
		}
		return sf.Layout(gtx, func(gtx C) D {
			return layout.Center.Layout(gtx, func(gtx C) D {
				w := kit.Regular
				if current || selected {
					w = kit.SemiBold
				}
				return th.Text(gtx, label, th.Type.Body, fg, w)
			})
		})
	})
}
