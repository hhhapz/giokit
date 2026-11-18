package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
)

// TextField is a single-line input around a widget.Editor. Read and set the
// text on the editor; set e.Submit yourself if Enter should submit. The
// border becomes a focus ring while the editor is focused.
//
//	ui.TextField{ID: "name", Placeholder: "Name"}.Layout(gtx, th, &v.name)
type TextField struct {
	ID          string          // the field's semantic id; the editor's area carries it
	Placeholder string          // dim hint text drawn while the editor is empty
	Label       string          // spoken name; default Placeholder
	Icon        kit.Icon        // optional leading icon (IconSearch for a search box)
	Password    bool            // mask the text (the value is unchanged); it sets the editor's Mask
	Size        kit.ControlSize // ControlDefault is the theme's
}

// Layout draws the field chrome and the editor e inside it. It sets
// e.SingleLine and e.Mask from the props every frame, so do not set those
// on e yourself. Read text changes and submits from e's own events.
func (f TextField) Layout(gtx layout.Context, th *kit.Theme, e *widget.Editor) layout.Dimensions {
	e.SingleLine = true
	e.Mask = 0
	if f.Password {
		e.Mask = passwordMask
	}
	return th.EditorField(gtx, e, f.ID, kit.EditorField{Placeholder: f.Placeholder, Label: f.Label, Icon: f.Icon, Size: f.Size})
}

// passwordMask is the rune a Password field draws for each character.
const passwordMask = '•'

// TextArea is a multi-line input (Enter inserts a newline) at least Lines
// lines tall; it grows with its text.
type TextArea struct {
	ID          string // the field's semantic id; the editor's area carries it
	Placeholder string // dim hint text drawn while the editor is empty
	Label       string // spoken name; default Placeholder
	Lines       int    // minimum height in text lines; below 1 is 1
}

// Layout draws the field chrome and the multi-line editor e inside it. It
// clears e.SingleLine every frame. Read text changes from e's own events.
func (f TextArea) Layout(gtx layout.Context, th *kit.Theme, e *widget.Editor) layout.Dimensions {
	e.SingleLine = false
	return th.EditorField(gtx, e, f.ID, kit.EditorField{Placeholder: f.Placeholder, Label: f.Label, Lines: max(1, f.Lines)})
}

// NumberField is a numeric editor with − and + buttons (ID/minus,
// ID/plus); Up/Down in the field step too.
//
// Its value is state-owned (see the package doc): the text being edited
// and the last valid number live in NumberFieldState. Set the value with
// SetValue, read changes from Update.
type NumberField struct {
	ID          string // the editor's id; the buttons are ID/minus and ID/plus
	Placeholder string // dim hint text drawn while the editor is empty
	// Label is the spoken name of the field and names the value in the
	// buttons' spoken labels ("Increase Quantity"). Default Placeholder; one
	// of the two is required.
	Label string
	// Min and Max bound the value when Max > Min: steps clamp to them, and
	// typed text outside them is an error. Use math.Inf for a one-sided
	// bound (Min: 0, Max: math.Inf(1)). Otherwise the value is unbounded.
	Min, Max float64
	Step     float64         // the buttons' and arrow keys' step; 0 is 1
	Size     kit.ControlSize // ControlDefault is the theme's
}

// numberBounds are a NumberField's bounds and step, kept from its last
// layout for the next Update.
type numberBounds struct {
	min, max, step float64
}

// bounded reports whether min and max bound the value.
func (b numberBounds) bounded() bool { return b.max > b.min }

// numberProblem is why a NumberField's text is not a valid value.
type numberProblem int

const (
	numberValid      numberProblem = iota
	numberNotANumber               // the text does not parse as a finite number
	numberOutOfRange               // the number is outside Min..Max
)

// NumberFieldState holds the editor, the buttons and the parsed value.
// The zero value is ready to use and starts at 0; like other states it
// must not be copied after first use.
type NumberFieldState struct {
	// Editor holds the text being edited. Prefer SetValue to writing it.
	Editor      widget.Editor
	Minus, Plus kit.Clickable // the step buttons

	value   float64
	problem numberProblem
	init    bool
	bounds  numberBounds // from the last layout
}

// Value is the last valid value: set by SetValue, a step, or text that
// parses within the bounds. Invalid text leaves it unchanged.
func (s *NumberFieldState) Value() float64 { return s.value }

// Valid reports whether the editor's text is a valid value.
func (s *NumberFieldState) Valid() bool { return s.problem == numberValid }

// Error is why the editor's text is not a valid value, in th's Strings,
// or "" when it is valid. Show it with FieldError.
func (s *NumberFieldState) Error(th *kit.Theme) string {
	switch s.problem {
	case numberNotANumber:
		return th.Strings.EnterNumber
	case numberOutOfRange:
		return fmt.Sprintf(th.Strings.NumberRange, s.bounds.min, s.bounds.max)
	}
	return ""
}

// SetValue sets the value and the editor's text, clearing any error.
func (s *NumberFieldState) SetValue(v float64) {
	s.init = true
	s.set(v)
}

// Update returns the value when buttons, keys or typing changed it since
// the last call. Call it in the handle pass. All of a frame's input is
// applied in order and the final value returned once.
func (s *NumberFieldState) Update(gtx layout.Context) (float64, bool) {
	before := s.value
	step := s.bounds.step
	if step == 0 {
		step = 1
	}
	if s.Minus.Clicked(gtx) {
		s.add(-step)
	}
	if s.Plus.Clicked(gtx) {
		s.add(step)
	}
	for {
		e, ok := gtx.Event(key.Filter{Focus: &s.Editor, Name: key.NameUpArrow}, key.Filter{Focus: &s.Editor, Name: key.NameDownArrow})
		if !ok {
			break
		}
		if ke, ok := e.(key.Event); ok && ke.State == key.Press {
			if ke.Name == key.NameUpArrow {
				s.add(step)
			} else {
				s.add(-step)
			}
			kit.Notify(gtx)
		}
	}
	for {
		e, ok := s.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := e.(widget.ChangeEvent); ok {
			s.parse()
			kit.Notify(gtx)
		}
	}
	return s.value, s.value != before
}

// add adds d to the value. The sum is rounded to the decimal places of
// the value and of d, so steps of 0.1 give 0.3, not the binary sum
// 0.30000000000000004.
func (s *NumberFieldState) add(d float64) {
	places := max(decimalPlaces(s.value), decimalPlaces(d))
	scale := math.Pow10(places)
	s.set(math.Round((s.value+d)*scale) / scale)
}

// maxDecimalPlaces caps decimalPlaces: beyond it float64 has no digits left.
const maxDecimalPlaces = 15

// decimalPlaces is the number of digits after the decimal point in v's
// shortest decimal form.
func decimalPlaces(v float64) int {
	str := strconv.FormatFloat(v, 'f', -1, 64)
	i := strings.IndexByte(str, '.')
	if i < 0 {
		return 0
	}
	return min(len(str)-i-1, maxDecimalPlaces)
}

// set clamps v to the bounds and makes it the value and the text.
func (s *NumberFieldState) set(v float64) {
	if s.bounds.bounded() {
		v = max(s.bounds.min, min(s.bounds.max, v))
	}
	s.value = v
	s.problem = numberValid
	s.Editor.SetText(strconv.FormatFloat(v, 'f', -1, 64))
}

// parse takes the typed text as the value when it is valid, and records
// the problem when not.
func (s *NumberFieldState) parse() {
	v, err := strconv.ParseFloat(strings.TrimSpace(s.Editor.Text()), 64)
	switch {
	case err != nil || math.IsNaN(v) || math.IsInf(v, 0):
		s.problem = numberNotANumber
	case s.bounds.bounded() && (v < s.bounds.min || v > s.bounds.max):
		s.problem = numberOutOfRange
	default:
		s.problem = numberValid
		s.value = v
	}
}

// Layout draws the editor and its − and + buttons from s. On first layout
// it writes s's value (0 unless SetValue ran) into the editor. It keeps
// the bounds and step for the next Update, which consumes the typing,
// the button clicks and the Up/Down keys.
func (f NumberField) Layout(gtx layout.Context, th *kit.Theme, s *NumberFieldState) layout.Dimensions {
	s.bounds = numberBounds{min: f.Min, max: f.Max, step: f.Step}
	if !s.init {
		s.SetValue(s.value)
	}
	s.Editor.SingleLine = true
	label := f.label()
	return kit.Flex{Align: layout.Middle, Gap: kit.Space1}.Layout(gtx, th,
		kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return th.EditorField(gtx, &s.Editor, f.ID, kit.EditorField{Placeholder: f.Placeholder, Label: f.Label, Size: f.Size})
		}),
		kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Row(gtx,
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return IconButton{ID: kit.Part(f.ID, "minus"), Icon: kit.IconMinus, Label: fmt.Sprintf(th.Strings.Decrease, label), Active: true}.Layout(gtx, th, &s.Minus)
				}),
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return IconButton{ID: kit.Part(f.ID, "plus"), Icon: kit.IconPlus, Label: fmt.Sprintf(th.Strings.Increase, label), Active: true}.Layout(gtx, th, &s.Plus)
				}),
			)
		}),
	)
}

// label is the field's spoken name.
func (f NumberField) label() string {
	if f.Label != "" {
		return f.Label
	}
	return f.Placeholder
}

// FieldError draws an error message under a field; nothing when empty.
type FieldError struct {
	Message string // the error, in the user's language; "" draws nothing
}

// Layout draws Message as small text in the theme's Bad colour. It takes
// no input and has no semantic id.
func (f FieldError) Layout(gtx layout.Context, th *kit.Theme) layout.Dimensions {
	if f.Message == "" {
		return layout.Dimensions{}
	}
	return th.Text(gtx, f.Message, th.Type.Small, th.Bad, kit.Regular)
}

// Form lays out labeled fields. At narrow widths it stacks labels above
// controls; wider, it aligns them in two columns. Errors and help text
// stay with their field in either layout. Rows are ID/<field ID>.
type Form struct {
	ID     string      // the form's group id; rows are ID/<field ID>
	Fields []FormField // the rows, top to bottom
}

// FormField is one labeled control. Control draws the control with its own
// state and id (a TextField, a Select...); ID names the row.
type FormField struct {
	// ID names the row (ID/<field ID>) and Label is its visible and spoken
	// name. Help is a dim line under the control; Error, when set,
	// replaces it in the theme's Bad colour.
	ID, Label, Help, Error string
	// Control draws the control, stretched to the row's width.
	Control layout.Widget
}

// Layout draws the rows: two columns (labels at th.LabelWidth, capped at a
// third of the width) or, below th.NarrowWidth, labels stacked above their
// controls. It records the form and each row as named groups and handles
// no input itself; the controls handle theirs.
func (f Form) Layout(gtx layout.Context, th *kit.Theme) layout.Dimensions {
	narrow := gtx.Constraints.Max.X < gtx.Dp(th.NarrowWidth)
	labelWidth := min(gtx.Dp(th.LabelWidth), gtx.Constraints.Max.X/3)
	rows := make([]layout.FlexChild, 0, len(f.Fields))
	for _, ff := range f.Fields {
		rows = append(rows, kit.Rigid(func(gtx layout.Context) layout.Dimensions { return f.row(gtx, th, ff, narrow, labelWidth) }))
	}
	kit.SetRole(gtx, f.ID, kit.RoleGroup)
	return kit.Group(gtx, f.ID, th.Strings.Form, func(gtx layout.Context) layout.Dimensions {
		return kit.Flex{Axis: layout.Vertical, Gap: kit.Space3}.Layout(gtx, th, rows...)
	})
}

// row is one field's group. Its name comes from the label inside it
// (ID/<field>/label), so the row itself is not labelled too: a screen
// reader would announce the label twice.
func (f Form) row(gtx layout.Context, th *kit.Theme, ff FormField, narrow bool, labelWidth int) layout.Dimensions {
	id := kit.Part(f.ID, ff.ID)
	kit.SetRole(gtx, id, kit.RoleGroup)
	return kit.Group(gtx, id, "", func(gtx layout.Context) layout.Dimensions {
		label := func(gtx layout.Context) layout.Dimensions {
			return kit.Group(gtx, kit.Part(id, "label"), ff.Label, func(gtx layout.Context) layout.Dimensions {
				return th.Text(gtx, ff.Label, th.Type.Body, th.Muted, kit.Medium)
			})
		}
		control := func(gtx layout.Context) layout.Dimensions {
			return kit.Flex{Axis: layout.Vertical, Gap: kit.Space1}.Layout(gtx, th, f.parts(th, ff)...)
		}
		if narrow {
			return kit.Flex{Axis: layout.Vertical, Gap: kit.Space1}.Layout(gtx, th, kit.Rigid(label), kit.Rigid(control))
		}
		return kit.Flex{Gap: kit.Space3}.Layout(gtx, th,
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = labelWidth, labelWidth
				return label(gtx)
			}),
			kit.Flexed(1, control),
		)
	})
}

// parts are the control and, when there is one, its error or help line.
func (f Form) parts(th *kit.Theme, ff FormField) []layout.FlexChild {
	var parts []layout.FlexChild
	if ff.Control != nil {
		parts = append(parts, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return ff.Control(gtx)
		}))
	}
	if ff.Error != "" || ff.Help != "" {
		fg, message := th.Dim, ff.Help
		if ff.Error != "" {
			fg, message = th.Bad, ff.Error
		}
		parts = append(parts, kit.Rigid(func(gtx layout.Context) layout.Dimensions { return th.Paragraph(gtx, message, fg) }))
	}
	return parts
}
