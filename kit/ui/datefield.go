package ui

import (
	"fmt"
	"image"
	"strconv"
	"time"

	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/io/semantic"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/unit"

	"github.com/hhhapz/giokit/kit"
)

// DateField is a field showing Value (zero shows Placeholder) that opens a
// calendar. It is controlled: set Value from what Update returns.
//
// The calendar works in civil dates. Value's calendar day in its own
// location is authoritative (its time of day is ignored), and Update
// returns the picked day at midnight in that location: time.Local while
// Value is zero. Today is the day of gtx.Now in time.Local.
//
// The calendar has three views. The title (ID/view) zooms out from days to
// months to years; picking a year zooms into its months, a month into its
// days. Prev/next (ID/prev, ID/next) step by a month, a year or a decade;
// a day that does not exist in the target month (January 31 to February)
// becomes that month's last day. Ids: days ID/YYYY-MM-DD, months
// ID/YYYY-MM, years ID/YYYY; the panel is ID, the field ID/field.
// Keyboard: arrows move within the view (by day and week, or across the
// 3-column month and year grids), PageUp/PageDown step like prev/next,
// Enter picks, Escape closes. Month and weekday names come from
// Theme.Strings.
type DateField struct {
	ID          string          // the calendar panel's id; see the type doc for its parts
	Value       time.Time       // the controlled value; zero shows Placeholder
	Placeholder string          // the field's text while Value is zero
	Format      string          // field text layout; default Theme.Strings.DateFormat
	SundayFirst bool            // weeks start on Sunday instead of Monday
	Size        kit.ControlSize // ControlDefault is the theme's
}

// DateFieldState is a DateField's interaction state. The zero value is
// closed; do not copy it after first use.
type DateFieldState struct {
	// Trigger is the field's press target; Update toggles the calendar
	// when it is clicked.
	Trigger kit.Clickable

	popup kit.Popup
	view  calView
	// shown is the first of the month on display (its year in the year
	// views); cursor is the active cell's date. Both are civil dates: UTC
	// midnight.
	shown, cursor time.Time
	// cells holds the visible cells' clickables, keyed by calendar key (the
	// keys of days, months and years differ in form). Entries are kept only
	// while visible.
	cells      kit.Pool[kit.Clickable]
	head       kit.Clickable
	prev, next kit.Clickable
	grid       kit.FocusGroup // the cells: one Tab stop, arrows move the cursor
	refocus    bool           // focus the grid after this layout (a view changed)

	// From the last layout, for Update.
	loc       *time.Location // the zone picked dates are returned in
	weekStart time.Weekday
}

// Opened reports whether the calendar is open.
func (s *DateFieldState) Opened() bool { return s.popup.Opened() }

// calView is a calendar view: days of a month, months of a year, or years
// of a decade.
type calView int

const (
	viewDays calView = iota
	viewMonths
	viewYears
)

// Calendar keys and geometry.
const (
	dayKey   = "2006-01-02"
	monthKey = "2006-01"
	gridCols = 3 // month and year grids: 3 × 4
	gridRows = 4
	// The day grid: 7 columns of weekdays, 6 rows of weeks (every month
	// fits in 6 weeks from its first week's start).
	weekDays  = 7
	monthRows = 6
	yearCells = gridCols * gridRows // a decade and the years either side
)

// Day cell geometry.
const (
	dayCellWidth  unit.Dp = 34
	dayCellHeight unit.Dp = 30
	weekdayHeight unit.Dp = 20 // the weekday names row
)

// civil is t's calendar date in t's location, as UTC midnight: the form
// the calendar computes in, free of zone offsets and daylight saving.
func civil(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// firstOfMonth is the first day of d's month.
func firstOfMonth(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// addMonths moves d by n months, keeping its day of the month or, when the
// target month is shorter, taking its last day (unlike time.AddDate, which
// rolls January 31 over into March).
func addMonths(d time.Time, n int) time.Time {
	first := time.Date(d.Year(), d.Month()+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(d.Day(), last)-1)
}

// decade is the first year of the decade holding y.
func decade(y int) int { return y - ((y%10)+10)%10 }

// sameMonth reports whether a and b fall in the same month of the same
// year. A zero b (no date) matches no month.
func sameMonth(a, b time.Time) bool {
	return !b.IsZero() && a.Year() == b.Year() && a.Month() == b.Month()
}

// key is the view's calendar key for d: a day, a month or a year.
func (v calView) key(d time.Time) string {
	switch v {
	case viewMonths:
		return d.Format(monthKey)
	case viewYears:
		return strconv.Itoa(d.Year())
	}
	return d.Format(dayKey)
}

// visible are the dates of the view's cells: the six weeks around the
// shown month, its year's twelve months, or the decade with a year either
// side.
func (s *DateFieldState) visible() []time.Time {
	switch s.view {
	case viewMonths:
		out := make([]time.Time, 12)
		for i := range out {
			out[i] = time.Date(s.shown.Year(), time.Month(i+1), 1, 0, 0, 0, 0, time.UTC)
		}
		return out
	case viewYears:
		first := decade(s.shown.Year()) - 1
		out := make([]time.Time, yearCells)
		for i := range out {
			out[i] = time.Date(first+i, time.January, 1, 0, 0, 0, 0, time.UTC)
		}
		return out
	}
	lead := (int(s.shown.Weekday()) - int(s.weekStart) + weekDays) % weekDays
	start := s.shown.AddDate(0, 0, -lead)
	out := make([]time.Time, weekDays*monthRows)
	for i := range out {
		out[i] = start.AddDate(0, 0, i)
	}
	return out
}

// Update returns a date picked since the last call, and handles the
// calendar's navigation. Call it in the handle pass. A pick closes the
// calendar and returns focus to the field.
func (s *DateFieldState) Update(gtx layout.Context) (time.Time, bool) {
	if s.Trigger.Clicked(gtx) {
		s.popup.Toggle()
	}
	// Cells first: a click lands on a cell of the view as last drawn.
	var picked time.Time
	ok := false
	if s.popup.Opened() {
		for _, d := range s.visible() {
			if !s.cells.Get(s.view.key(d)).Clicked(gtx) {
				continue
			}
			switch s.view {
			case viewYears:
				s.shown = time.Date(d.Year(), s.shown.Month(), 1, 0, 0, 0, 0, time.UTC)
				s.cursor, s.view, s.refocus = s.shown, viewMonths, true
			case viewMonths:
				s.shown, s.cursor, s.view, s.refocus = d, d, viewDays, true
			default:
				picked, ok = d, true
			}
			break
		}
	}
	if s.head.Clicked(gtx) && s.view < viewYears {
		s.view++
		s.refocus = true
	}
	if s.prev.Clicked(gtx) {
		s.page(-1)
	}
	if s.next.Clicked(gtx) {
		s.page(1)
	}
	if !ok {
		return time.Time{}, false
	}
	s.popup.Hide()
	s.Trigger.FocusQuiet(gtx)
	loc := s.loc
	if loc == nil {
		loc = time.Local
	}
	return time.Date(picked.Year(), picked.Month(), picked.Day(), 0, 0, 0, 0, loc), true
}

// page steps the view by n pages (a month, a year or a decade each),
// moving the cursor with it.
func (s *DateFieldState) page(n int) {
	months := n
	switch s.view {
	case viewMonths:
		months = 12 * n
	case viewYears:
		months = 120 * n
	}
	s.shown = addMonths(s.shown, months)
	s.cursor = addMonths(s.cursor, months)
}

// move moves the cursor by n steps of the view's unit (a day, a month or
// a year) and pages the view to show it.
func (s *DateFieldState) move(n int) {
	switch s.view {
	case viewDays:
		s.cursor = s.cursor.AddDate(0, 0, n)
		if !sameMonth(s.cursor, s.shown) {
			s.shown = firstOfMonth(s.cursor)
		}
	case viewMonths:
		s.cursor = addMonths(s.cursor, n)
		s.shown = time.Date(s.cursor.Year(), s.shown.Month(), 1, 0, 0, 0, 0, time.UTC)
	case viewYears:
		s.cursor = addMonths(s.cursor, 12*n)
		if decade(s.cursor.Year()) != decade(s.shown.Year()) {
			s.shown = time.Date(s.cursor.Year(), s.shown.Month(), 1, 0, 0, 0, 0, time.UTC)
		}
	}
}

// Layout draws the field showing Value in Format and, while s is open, the
// calendar in a popover. It handles the calendar's view, paging and arrow
// keys while laying out, and keeps Value's zone and the week start for
// s.Update, which reports a picked day.
func (f DateField) Layout(gtx layout.Context, th *kit.Theme, s *DateFieldState) layout.Dimensions {
	s.loc = time.Local
	if !f.Value.IsZero() {
		s.loc = f.Value.Location()
	}
	s.weekStart = time.Monday
	if f.SundayFirst {
		s.weekStart = time.Sunday
	}
	sel := civil(f.Value)
	if s.popup.Opening() || s.shown.IsZero() {
		base := sel
		if base.IsZero() {
			base = civil(gtx.Now.In(time.Local))
		}
		s.shown, s.cursor, s.view = firstOfMonth(base), base, viewDays
	}
	s.popup.FocusOnOpen(s.grid.FocusHandle()) // an open calendar focuses its grid
	if s.refocus {
		s.refocus = false
		defer s.grid.FocusQuiet(gtx)
	}
	if dm, byKey := s.popup.DidDismiss(); dm {
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
		label, lc = f.Value.Format(layoutStr), th.Fg
	}
	return th.Popover(gtx, &s.popup, f.ID, func(gtx layout.Context) layout.Dimensions {
		kit.SetRole(gtx, f.ID, kit.RoleDialog)
		return th.Press(gtx, &s.Trigger, kit.Part(f.ID, "field"), func(gtx layout.Context, st kit.State) layout.Dimensions {
			describe(gtx, kit.Part(f.ID, "field"), kit.RoleCombobox, semantic.Button, "") // spoken by its text
			st.Focused = st.Focused || s.popup.Opened()
			return th.SizedField(gtx, f.Size, st, func(gtx layout.Context) layout.Dimensions {
				return kit.Row(gtx,
					kit.Flexed(1, func(gtx layout.Context) layout.Dimensions { return th.Text(gtx, label, th.Type.Body, lc, kit.Regular) }),
					kit.Rigid(func(gtx layout.Context) layout.Dimensions { return kit.IconChevronDown(gtx, th.IconSize, th.Muted) }),
				)
			})
		})
	}, func(gtx layout.Context) layout.Dimensions { return f.calendar(gtx, th, s, sel) })
}

// calCell is one cell of a calendar view, ready to draw.
type calCell struct {
	date  time.Time // the day, or the first of the month or year
	label string
	// selected is Value's day, month or year; current is today's; outside
	// is a day of another month or a year of a neighbouring decade.
	selected, current, outside bool
}

// cellsOf describes the view's visible cells for drawing.
func (s *DateFieldState) cellsOf(th *kit.Theme, dates []time.Time, sel, today time.Time) []calCell {
	out := make([]calCell, len(dates))
	for i, d := range dates {
		c := calCell{date: d}
		switch s.view {
		case viewMonths:
			c.label = th.Strings.Months[d.Month()-1]
			c.selected, c.current = sameMonth(d, sel), sameMonth(d, today)
		case viewYears:
			c.label = strconv.Itoa(d.Year())
			c.selected = !sel.IsZero() && sel.Year() == d.Year()
			c.current = today.Year() == d.Year()
			c.outside = i == 0 || i == len(dates)-1
		default:
			c.label = strconv.Itoa(d.Day())
			c.selected, c.current = d.Equal(sel), d.Equal(today)
			c.outside = d.Month() != s.shown.Month()
		}
		out[i] = c
	}
	return out
}

// calendar handles the current view's keys (before drawing, so a page
// lands this frame), then lays out the header and the view's grid.
func (f DateField) calendar(gtx layout.Context, th *kit.Theme, s *DateFieldState, sel time.Time) layout.Dimensions {
	// Every visible cell joins the grid: members, not Tab stops. The
	// cursor's cell is the active member: the one Tab stop.
	s.grid.Active = s.view.key(s.cursor)
	for _, d := range s.visible() {
		k := s.view.key(d)
		s.grid.Join(s.cells.Get(k), k == s.grid.Active)
	}
	f.pageKeys(gtx, s)
	row := weekDays // the arrow keys' vertical step
	if s.view != viewDays {
		row = gridCols
	}
	keys := func(gtx layout.Context) {
		step := func(n int) func() {
			return func() {
				s.move(n)
				s.grid.Active = s.view.key(s.cursor)
			}
		}
		kit.OnAction(gtx, kit.ActCollapse, step(-1))
		kit.OnAction(gtx, kit.ActExpand, step(1))
		kit.OnAction(gtx, kit.ActMoveUp, step(-row))
		kit.OnAction(gtx, kit.ActMoveDown, step(row))
		kit.OnAction(gtx, kit.ActConfirm, func() { s.cells.Get(s.view.key(s.cursor)).Click() }) // reported by Update
	}

	cell := image.Pt(gtx.Dp(dayCellWidth), gtx.Dp(dayCellHeight))
	weekdays := gtx.Dp(weekdayHeight)
	// The day grid's size: every view keeps it, so paging never resizes.
	gridW, gridH := cell.X*weekDays, weekdays+cell.Y*monthRows
	var title string
	switch s.view {
	case viewDays:
		title = s.shown.Format(th.Strings.MonthFormat)
	case viewMonths:
		title = strconv.Itoa(s.shown.Year())
	default:
		d := decade(s.shown.Year())
		title = fmt.Sprintf(th.Strings.DecadeFormat, d, d+9)
	}
	header := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = gridW, gridW
		return kit.Row(gtx,
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return IconButton{ID: kit.Part(f.ID, "prev"), Icon: kit.IconChevronLeft, Label: th.Strings.PrevPage, Active: true}.Layout(gtx, th, &s.prev)
			}),
			kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return f.viewButton(gtx, th, s, title) })
			}),
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return IconButton{ID: kit.Part(f.ID, "next"), Icon: kit.IconChevronRight, Label: th.Strings.NextPage, Active: true}.Layout(gtx, th, &s.next)
			}),
		)
	}
	cells := s.cellsOf(th, s.visible(), sel, civil(gtx.Now.In(time.Local)))
	body := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(image.Pt(gridW, gridH))
		if s.view == viewDays {
			return f.dayGrid(gtx, th, s, cells, cell, weekdays)
		}
		return f.grid(gtx, th, len(cells), func(gtx layout.Context, i int) layout.Dimensions { return f.gridCell(gtx, th, s, cells[i]) })
	}
	grid := func(gtx layout.Context) layout.Dimensions {
		return kit.KeyContext(gtx, "calendar", func(gtx layout.Context) layout.Dimensions {
			keys(gtx)
			kit.SetRole(gtx, kit.Part(f.ID, "grid"), kit.RoleGrid)
			return s.grid.Layout(gtx, kit.Part(f.ID, "grid"), func(gtx layout.Context) layout.Dimensions {
				// The group's own area takes no pointer gestures: name it on a nested one.
				return kit.TagArea(gtx, kit.Part(f.ID, "grid"), body)
			})
		})
	}
	d := kit.Flex{Axis: layout.Vertical, Pad: kit.Space2, Gap: kit.Space1p5}.Layout(gtx, th, kit.Rigid(header), kit.Rigid(grid))
	s.cells.Sweep() // every visible cell was touched: drop the ones paged away
	return d
}

// pageKeys handles PageUp and PageDown on the grid: a page of the view.
func (f DateField) pageKeys(gtx layout.Context, s *DateFieldState) {
	focus := s.grid.FocusHandle()
	for {
		e, ok := gtx.Event(key.Filter{Focus: focus, Name: key.NamePageUp}, key.Filter{Focus: focus, Name: key.NamePageDown})
		if !ok {
			return
		}
		ke, ok := e.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		n := 1
		if ke.Name == key.NamePageUp {
			n = -1
		}
		switch s.view {
		case viewDays:
			s.cursor = addMonths(s.cursor, n)
			s.shown = firstOfMonth(s.cursor)
		case viewMonths:
			s.move(12 * n)
		case viewYears:
			s.move(10 * n)
		}
		s.grid.Active = s.view.key(s.cursor)
		kit.Notify(gtx)
	}
}

// viewButton is the title: it zooms out to the next coarser view.
func (f DateField) viewButton(gtx layout.Context, th *kit.Theme, s *DateFieldState, title string) layout.Dimensions {
	return th.Press(gtx, &s.head, kit.Part(f.ID, "view"), func(gtx layout.Context, st kit.State) layout.Dimensions {
		describe(gtx, kit.Part(f.ID, "view"), kit.RoleButton, semantic.Button, title)
		sf := th.Styled(kit.Surface{Bg: th.Highlight(st.Hover), Radius: th.RadiusSm()})
		if st.Focused {
			sf.Focus = th.Focus
		}
		return sf.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return kit.Flex{Gap: kit.Space1, Align: layout.Middle, PadX: kit.Space2, PadY: kit.Space1}.Layout(gtx, th,
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return th.Text(gtx, title, th.Type.Body, th.Fg, kit.SemiBold)
				}),
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.view == viewYears {
						return layout.Dimensions{}
					}
					return kit.IconChevronDown(gtx, th.IconSize, th.Muted)
				}),
			)
		})
	})
}

// dayGrid lays out the weekday names over six weeks of day cells.
func (f DateField) dayGrid(gtx layout.Context, th *kit.Theme, s *DateFieldState, cells []calCell, cell image.Point, headH int) layout.Dimensions {
	rows := []layout.FlexChild{kit.Rigid(func(gtx layout.Context) layout.Dimensions {
		var hs []layout.FlexChild
		for i := range weekDays {
			name := th.Strings.Weekdays[(int(s.weekStart)+i)%weekDays]
			hs = append(hs, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(image.Pt(cell.X, headH))
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return th.Text(gtx, name, th.Type.Small, th.Muted, kit.Medium)
				})
			}))
		}
		return layout.Flex{}.Layout(gtx, hs...)
	})}
	for w := range monthRows {
		week := cells[w*weekDays : (w+1)*weekDays]
		rows = append(rows, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			var ds []layout.FlexChild
			for _, c := range week {
				ds = append(ds, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints = layout.Exact(cell)
					return f.gridCell(gtx, th, s, c)
				}))
			}
			return layout.Flex{}.Layout(gtx, ds...)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

// grid lays out n cells in rows of gridCols, filling the constraints.
func (f DateField) grid(gtx layout.Context, th *kit.Theme, n int, cell func(gtx layout.Context, i int) layout.Dimensions) layout.Dimensions {
	rowsN := (n + gridCols - 1) / gridCols
	sz := image.Pt(gtx.Constraints.Max.X/gridCols, gtx.Constraints.Max.Y/rowsN)
	var rows []layout.FlexChild
	for r := range rowsN {
		rows = append(rows, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			var cs []layout.FlexChild
			for c := range gridCols {
				i := r*gridCols + c
				if i >= n {
					break
				}
				cs = append(cs, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints = layout.Exact(sz)
					return layout.UniformInset(th.Space(kit.Space0p5)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
// outlined, outside dimmed.
func (f DateField) gridCell(gtx layout.Context, th *kit.Theme, s *DateFieldState, c calCell) layout.Dimensions {
	k := s.view.key(c.date)
	id := kit.Part(f.ID, k)
	return th.Press(gtx, s.cells.Get(k), id, func(gtx layout.Context, st kit.State) layout.Dimensions {
		describe(gtx, id, kit.RoleGridCell, semantic.Button, "") // spoken by its text
		semantic.SelectedOp(c.selected).Add(gtx.Ops)
		fg := th.Fg
		if c.outside {
			fg = th.Dim
		}
		sf := th.Styled(kit.Surface{Bg: th.Highlight(st.Hover), Radius: th.RadiusSm()})
		switch {
		case c.selected:
			sf.Bg, fg = th.Accent, th.OnAccent
		case c.current:
			sf.Border = th.Border
		}
		if st.Focused {
			sf.Focus = th.Focus
		}
		return sf.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				w := kit.Regular
				if c.current || c.selected {
					w = kit.SemiBold
				}
				return th.Text(gtx, c.label, th.Type.Body, fg, w)
			})
		})
	})
}
