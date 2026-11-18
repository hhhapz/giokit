package kit

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Strings is the text kit itself shows or speaks. Fields with %s, %g or %d
// verbs are fmt formats (%[1]s reorders for languages that need it), and
// Validate checks that a translation keeps their verbs; DateFormat and
// MonthFormat are Go time layouts. Replace the theme's Strings to
// translate, starting from English.
type Strings struct {
	Expand, Collapse   string     // tree toggles: "Expand %s"
	SortedAsc          string     // a sorted column header's spoken name: "%s (asc)"
	SortedDesc         string     // the same, sorted descending: "%s (desc)"
	Increase, Decrease string     // stepper buttons: "Increase %s"
	MoveUp, MoveDown   string     // reorder buttons: "Move %s up"
	Reorder            string     // a reorder drag handle: "Reorder %s; ..."
	Required           string     // validation
	EnterNumber        string     // a number field's text does not parse
	NumberRange        string     // "Use %g to %g"
	Commands           string     // the command palette title
	CommandPlaceholder string     // the command palette's query hint
	NoCommands         string     // the command palette found nothing
	NoLogLines         string     // log view: the buffer is empty
	NoLogMatches       string     // log view: the filters hide every line
	Latest             string     // log view: jump to the newest line
	Accordion          string     // spoken names of groups
	Breadcrumbs        string     // the Breadcrumbs group
	Toolbar            string     // the Toolbar group
	Form               string     // the Form group
	PrevPage, NextPage string     // calendar paging buttons
	Close              string     // close buttons
	Dismiss            string     // a toast's dismiss button
	DateFormat         string     // a date field's text, as a Go time layout
	MonthFormat        string     // a calendar's month title, as a Go time layout
	DecadeFormat       string     // a calendar's decade title: "%d–%d", first and last year
	Months             [12]string // short month names, January first: a calendar's month grid
	Weekdays           [7]string  // short weekday names, Sunday first: a calendar's column heads
	Video              string     // a video's spoken name when the app gives none
}

// English returns the default Strings. Each call returns a fresh value:
// start a translation from it and replace the fields that differ.
func English() Strings {
	return Strings{
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
		PrevPage: "Previous", NextPage: "Next", Close: "Close", Dismiss: "Dismiss",
		DateFormat: "Mon, Jan 2, 2006", MonthFormat: "January 2006", DecadeFormat: "%d–%d",
		Months:   [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
		Weekdays: [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"},
	}
}

// stringFormat is one fmt-format field of Strings and the arguments kit
// formats it with.
type stringFormat struct {
	name  string
	value string
	verbs string // one verb letter per argument, in argument order
}

// formats lists the fields of s that are fmt formats.
func (s *Strings) formats() []stringFormat {
	return []stringFormat{
		{name: "Expand", value: s.Expand, verbs: "s"},
		{name: "Collapse", value: s.Collapse, verbs: "s"},
		{name: "SortedAsc", value: s.SortedAsc, verbs: "s"},
		{name: "SortedDesc", value: s.SortedDesc, verbs: "s"},
		{name: "Increase", value: s.Increase, verbs: "s"},
		{name: "Decrease", value: s.Decrease, verbs: "s"},
		{name: "MoveUp", value: s.MoveUp, verbs: "s"},
		{name: "MoveDown", value: s.MoveDown, verbs: "s"},
		{name: "Reorder", value: s.Reorder, verbs: "s"},
		{name: "NumberRange", value: s.NumberRange, verbs: "gg"},
		{name: "DecadeFormat", value: s.DecadeFormat, verbs: "dd"},
	}
}

// Validate checks the fmt formats of a translation, which go vet cannot:
// each format field must use every argument kit passes it, with the same
// verb as English (%s for names, %g for numbers, %d for years), and no
// others. Arguments may be reordered with explicit indexes
// ("%[2]g … %[1]g"). It returns an error naming the first field that does
// not. Time layouts (DateFormat,
// MonthFormat) are not checked.
func (s Strings) Validate() error {
	for _, f := range s.formats() {
		if err := checkFormat(f.value, f.verbs); err != nil {
			return fmt.Errorf("kit: Strings.%s %q: %w", f.name, f.value, err)
		}
	}
	return nil
}

// checkFormat reports whether format uses exactly the arguments described
// by verbs, following fmt's rules for implicit and explicit argument
// indexes.
func checkFormat(format, verbs string) error {
	used := make([]bool, len(verbs))
	arg := 0 // index of the next implicit argument
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		i++
		if i < len(format) && format[i] == '%' {
			continue
		}
		i = skipFormatFlags(format, i)
		if i < len(format) && format[i] == '[' {
			end := strings.IndexByte(format[i:], ']')
			if end < 0 {
				return errors.New("unclosed argument index")
			}
			n, err := strconv.Atoi(format[i+1 : i+end])
			if err != nil || n < 1 {
				return fmt.Errorf("bad argument index %s", format[i:i+end+1])
			}
			arg = n - 1
			i = skipFormatFlags(format, i+end+1)
		}
		if i >= len(format) {
			return errors.New("ends with an incomplete verb")
		}
		if arg >= len(verbs) {
			return fmt.Errorf("verb %%%c has no argument (kit passes %d)", format[i], len(verbs))
		}
		if format[i] != verbs[arg] {
			return fmt.Errorf("argument %d needs %%%c, not %%%c", arg+1, verbs[arg], format[i])
		}
		used[arg] = true
		arg++
	}
	for i, ok := range used {
		if !ok {
			return fmt.Errorf("argument %d (%%%c) is not used", i+1, verbs[i])
		}
	}
	return nil
}

// skipFormatFlags skips the flags, width and precision of a verb.
func skipFormatFlags(format string, i int) int {
	for i < len(format) && strings.IndexByte("+-# 0123456789.", format[i]) >= 0 {
		i++
	}
	return i
}
