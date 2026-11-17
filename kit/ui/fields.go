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
	ID          string
	Placeholder string
	Label       string   // spoken name; default Placeholder
	Icon        kit.Icon // optional leading icon (IconSearch for a search box)
	Password    bool     // mask the text (the value is unchanged)
}

func (f TextField) Layout(gtx C, th *kit.Theme, e *widget.Editor) D {
	e.SingleLine = true
	if f.Password {
		e.Mask = '•'
	}
	return th.EditorField(gtx, e, f.ID, kit.EditorField{Placeholder: f.Placeholder, Label: f.Label, Icon: f.Icon})
}

// TextArea is a multi-line input (Enter inserts a newline) at least Lines
// lines tall; it grows with its text.
type TextArea struct {
	ID          string
	Placeholder string
	Label       string // spoken name; default Placeholder
	Lines       int
}

func (f TextArea) Layout(gtx C, th *kit.Theme, e *widget.Editor) D {
	e.SingleLine = false
	return th.EditorField(gtx, e, f.ID, kit.EditorField{Placeholder: f.Placeholder, Label: f.Label, Lines: max(1, f.Lines)})
}

// NumberField is a numeric editor with − and + buttons (ID/minus,
// ID/plus); Up/Down in the field step too. Min and Max bound the value
// when either is non-zero; Step defaults to 1.
type NumberField struct {
	ID          string
	Placeholder string
	Min, Max    float64
	Step        float64
}

// NumberFieldState holds the editor and the parsed value. Value and Error
// are read-only between frames: set the value with SetValue.
type NumberFieldState struct {
	Editor      widget.Editor
	Minus, Plus kit.Clickable
	Value       float64
	Error       string // why the text is not a valid value; "" when valid

	init           bool
	min, max, step float64 // from the last layout
	strings        *kit.Strings
}

// SetValue sets the value and the editor's text.
func (s *NumberFieldState) SetValue(v float64) {
	s.Value, s.Error, s.init = v, "", true
	s.Editor.SetText(strconv.FormatFloat(v, 'f', -1, 64))
}

// Update returns the value when buttons, keys or typing changed it since
// the last call. Call it in the handle pass.
func (s *NumberFieldState) Update(gtx C) (float64, bool) {
	before := s.Value
	step := s.step
	if step == 0 {
		step = 1
	}
	if s.Minus.Clicked(gtx) {
		s.set(s.Value - step)
	}
	if s.Plus.Clicked(gtx) {
		s.set(s.Value + step)
	}
	for {
		e, ok := gtx.Event(key.Filter{Focus: &s.Editor, Name: key.NameUpArrow}, key.Filter{Focus: &s.Editor, Name: key.NameDownArrow})
		if !ok {
			break
		}
		if ke, ok := e.(key.Event); ok && ke.State == key.Press {
			if ke.Name == key.NameUpArrow {
				s.set(s.Value + step)
			} else {
				s.set(s.Value - step)
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
	return s.Value, s.Value != before
}

func (s *NumberFieldState) bounded() bool { return s.min != 0 || s.max != 0 }

func (s *NumberFieldState) set(v float64) {
	if s.bounded() {
		v = math.Max(s.min, math.Min(s.max, v))
	}
	s.Value = v
	s.Editor.SetText(strconv.FormatFloat(v, 'f', -1, 64))
	s.Error = ""
}

func (s *NumberFieldState) parse() {
	str := &kit.English
	if s.strings != nil {
		str = s.strings
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s.Editor.Text()), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		s.Error = str.EnterNumber
		return
	}
	if s.bounded() && (v < s.min || v > s.max) {
		s.Error = fmt.Sprintf(str.NumberRange, s.min, s.max)
		return
	}
	s.Error = ""
	s.Value = v
}

func (f NumberField) Layout(gtx C, th *kit.Theme, s *NumberFieldState) D {
	s.min, s.max, s.step, s.strings = f.Min, f.Max, f.Step, &th.Strings
	if !s.init {
		s.SetValue(s.Value)
	}
	s.Editor.SingleLine = true
	return kit.Flex{Align: layout.Middle, Gap: kit.Space1}.Layout(gtx, th,
		kit.Flexed(1, func(gtx C) D {
			return th.EditorField(gtx, &s.Editor, f.ID, kit.EditorField{Placeholder: f.Placeholder})
		}),
		kit.Rigid(func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(func(gtx C) D {
					return IconButton{ID: kit.Part(f.ID, "minus"), Icon: kit.IconMinus, Label: fmt.Sprintf(th.Strings.Decrease, f.label()), Active: true}.Layout(gtx, th, &s.Minus)
				}),
				kit.Rigid(func(gtx C) D {
					return IconButton{ID: kit.Part(f.ID, "plus"), Icon: kit.IconPlus, Label: fmt.Sprintf(th.Strings.Increase, f.label()), Active: true}.Layout(gtx, th, &s.Plus)
				}),
			)
		}),
	)
}

// label names the value in the buttons' spoken labels.
func (f NumberField) label() string {
	if f.Placeholder != "" {
		return f.Placeholder
	}
	return f.ID
}

// FieldError draws an error message under a field; nothing when empty.
type FieldError struct{ Message string }

func (f FieldError) Layout(gtx C, th *kit.Theme) D {
	if f.Message == "" {
		return D{}
	}
	return th.Text(gtx, f.Message, th.Type.Small, th.Bad, kit.Regular)
}

// Form lays out labeled fields. At narrow widths it stacks labels above
// controls; wider, it aligns them in two columns. Errors and help text
// stay with their field in either layout. Rows are ID/<field ID>.
type Form struct {
	ID     string
	Fields []FormField
}

// FormField is one labeled control. Control draws the control with its own
// state and id (a TextField, a Select...); ID names the row.
type FormField struct {
	ID, Label, Help, Error string
	Control                kit.W
}

func (f Form) Layout(gtx C, th *kit.Theme) D {
	narrow := gtx.Constraints.Max.X < gtx.Dp(th.NarrowWidth)
	labelWidth := min(gtx.Dp(th.LabelWidth), gtx.Constraints.Max.X/3)
	rows := make([]layout.FlexChild, 0, len(f.Fields))
	for _, ff := range f.Fields {
		rows = append(rows, kit.Rigid(func(gtx C) D { return f.row(gtx, th, ff, narrow, labelWidth) }))
	}
	return kit.Group(gtx, f.ID, th.Strings.Form, func(gtx C) D {
		return kit.Flex{Axis: layout.Vertical, Gap: kit.Space3}.Layout(gtx, th, rows...)
	})
}

func (f Form) row(gtx C, th *kit.Theme, ff FormField, narrow bool, labelWidth int) D {
	return kit.Group(gtx, kit.Part(f.ID, ff.ID), ff.Label, func(gtx C) D {
		label := func(gtx C) D {
			return kit.Group(gtx, kit.Part(f.ID, ff.ID, "label"), ff.Label, func(gtx C) D {
				return th.Text(gtx, ff.Label, th.Type.Body, th.Muted, kit.Medium)
			})
		}
		control := func(gtx C) D {
			return kit.Flex{Axis: layout.Vertical, Gap: kit.Space1}.Layout(gtx, th, f.parts(th, ff)...)
		}
		if narrow {
			return kit.Flex{Axis: layout.Vertical, Gap: kit.Space1}.Layout(gtx, th, kit.Rigid(label), kit.Rigid(control))
		}
		return kit.Flex{Gap: kit.Space3}.Layout(gtx, th,
			kit.Rigid(func(gtx C) D {
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
		parts = append(parts, kit.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return ff.Control(gtx)
		}))
	}
	if ff.Error != "" || ff.Help != "" {
		color, message := th.Dim, ff.Help
		if ff.Error != "" {
			color, message = th.Bad, ff.Error
		}
		parts = append(parts, kit.Rigid(func(gtx C) D { return th.Paragraph(gtx, message, color) }))
	}
	return parts
}
