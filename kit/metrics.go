package kit

import (
	"image/color"

	"gioui.org/unit"
)

// Metrics are the theme's fixed sizes: everything that is neither spacing
// (the SpaceN scale, through Theme.Space) nor a corner (Theme.Radius).
// Components read them from the theme instead of writing numbers.
type Metrics struct {
	IconSize       unit.Dp // icons in controls, rows and fields
	IconSizeLg     unit.Dp // icons in alerts and toasts
	CheckSize      unit.Dp // checkbox box
	IconButtonSize unit.Dp // square icon-only button
	SwitchWidth    unit.Dp
	SwitchHeight   unit.Dp
	HitSize        unit.Dp // smallest pointer target for thin things (split handles, scrollbars)
	PaneMinSize    unit.Dp // smallest split pane

	MenuMinWidth    unit.Dp // menus are at least this wide
	ListMaxHeight   unit.Dp // a select's list scrolls past this
	PopoverMaxWidth unit.Dp
	DialogMaxWidth  unit.Dp
	ToastWidth      unit.Dp
	ReadingWidth    unit.Dp // longest comfortable text block (empty states)
	LabelWidth      unit.Dp // form label column
	NarrowWidth     unit.Dp // below this, forms stack labels above fields
}

// DefaultMetrics are giokit's sizes at density 1.
var DefaultMetrics = Metrics{
	IconSize: 14, IconSizeLg: 16, CheckSize: 18, IconButtonSize: 22,
	SwitchWidth: 34, SwitchHeight: 20, HitSize: 16, PaneMinSize: 80,
	MenuMinWidth: 180, ListMaxHeight: 300, PopoverMaxWidth: 420, DialogMaxWidth: 480,
	ToastWidth: 340, ReadingWidth: 360, LabelWidth: 132, NarrowWidth: 440,
}

// Strings is the text kit itself shows or speaks. Fields with %s or %g are
// fmt formats (%[1]s reorders for languages that need it). Replace the
// theme's Strings to translate.
type Strings struct {
	Expand, Collapse   string // tree toggles: "Expand %s"
	SortedAsc          string // a sorted column header's spoken name: "%s (asc)"
	SortedDesc         string
	Increase, Decrease string // stepper buttons: "Increase %s"
	MoveUp, MoveDown   string // reorder buttons: "Move %s up"
	Reorder            string // a reorder drag handle: "Reorder %s; ..."
	Required           string // validation
	EnterNumber        string
	NumberRange        string // "Use %g to %g"
	Commands           string // the command palette title
	CommandPlaceholder string
	NoCommands         string
	NoLogLines         string
	NoLogMatches       string
	Latest             string // log view: jump to the newest line
	Accordion          string // spoken names of groups
	Breadcrumbs        string
	Toolbar            string
	Form               string
	PrevPage, NextPage string // calendar paging buttons
	Close              string // close buttons (toasts)
	DateFormat         string // a date field's text, as a Go time layout
	MonthFormat        string // a calendar's month title, as a Go time layout
	Video              string // a video's spoken name when the app gives none
}

// English is the default Strings.
var English = Strings{
	Video:  "Video",
	Expand: "Expand %s", Collapse: "Collapse %s",
	SortedAsc: "%s (asc)", SortedDesc: "%s (desc)",
	Increase: "Increase %s", Decrease: "Decrease %s",
	MoveUp: "Move %s up", MoveDown: "Move %s down",
	Reorder:  "Reorder %s; use up and down arrows",
	Required: "Required", EnterNumber: "Enter a number", NumberRange: "Use %g to %g",
	Commands: "Commands", CommandPlaceholder: "Type a command", NoCommands: "No matching commands",
	NoLogLines: "No log lines yet", NoLogMatches: "No lines match the filter", Latest: "Latest",
	Accordion: "Accordion", Breadcrumbs: "Breadcrumbs", Toolbar: "Toolbar", Form: "Form",
	PrevPage: "Previous", NextPage: "Next", Close: "Close",
	DateFormat: "Mon, Jan 2, 2006", MonthFormat: "January 2006",
}

// PressTint is how far a pressed control's fill moves toward Fg (hover goes
// half way).
const PressTint = 0.16

// StateColor is a control's fill base as drawn in st: hover moves it half
// way to a press, a press all the way. Buttons and fields use it.
func (t *Theme) StateColor(base color.NRGBA, st State) color.NRGBA {
	amt := PressTint / 2 * st.Hover
	if st.Pressed {
		amt = PressTint
	}
	return t.Tint(base, amt)
}

// Highlight is the background of a row or cell lit by lit (0..1): hover
// animates it, focus, press or a keyboard choice light it fully. Menu rows,
// calendar cells and list rows use it.
func (t *Theme) Highlight(lit float32) color.NRGBA {
	return WithAlpha(t.Tint(t.Overlay, PressTint), uint8(255*lit))
}

// Tint mixes c toward the foreground by amount (PressTint is a full press):
// a hover/press colour that works on light and dark themes alike. A
// transparent c gives a translucent foreground.
func (t *Theme) Tint(c color.NRGBA, amount float32) color.NRGBA {
	if c.A == 0 {
		return WithAlpha(t.Fg, uint8(amount*255))
	}
	return LerpColor(c, t.Fg, amount)
}
