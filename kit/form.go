package kit

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
)

// PasswordField masks the editor without changing its value. Its text is
// available through e.Text(); callers own storage and clearing.
func (t *Theme) PasswordField(gtx C, e *widget.Editor, id, placeholder string) D {
	e.SingleLine = true
	e.Mask = '•'
	return t.TextField(gtx, e, id, placeholder)
}

// NumberField is a numeric editor with keyboard and button steppers.
// Min/Max are optional when both zero; Step defaults to one.
type NumberField struct {
	Editor                widget.Editor
	Value, Min, Max, Step float64
	Minus, Plus           Clickable
	Error                 string
	init, changed         bool
}

func (n *NumberField) SetValue(v float64) {
	n.Value = v
	n.Editor.SetText(strconv.FormatFloat(v, 'f', -1, 64))
	n.Error = ""
	n.init = true
}
func (n *NumberField) Changed() (float64, bool) {
	v, ok := n.Value, n.changed
	n.changed = false
	return v, ok
}
func (n *NumberField) set(v float64) {
	if n.Min != 0 || n.Max != 0 {
		v = math.Max(n.Min, math.Min(n.Max, v))
	}
	if v != n.Value {
		n.Value, n.changed = v, true
	}
	n.Editor.SetText(strconv.FormatFloat(v, 'f', -1, 64))
	n.Error = ""
}
func (n *NumberField) parse() {
	s := strings.TrimSpace(n.Editor.Text())
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		n.Error = "Enter a number"
		return
	}
	if n.Min != 0 || n.Max != 0 {
		if v < n.Min || v > n.Max {
			n.Error = fmt.Sprintf("Use %g to %g", n.Min, n.Max)
			return
		}
	}
	n.Error = ""
	if v != n.Value {
		n.Value, n.changed = v, true
	}
}

func (t *Theme) NumberField(gtx C, n *NumberField, id, placeholder string) D {
	if !n.init {
		n.SetValue(n.Value)
	}
	step := n.Step
	if step == 0 {
		step = 1
	}
	if n.Minus.Clicked(gtx) {
		n.set(n.Value - step)
	}
	if n.Plus.Clicked(gtx) {
		n.set(n.Value + step)
	}
	for {
		e, ok := gtx.Event(key.Filter{Focus: &n.Editor, Name: key.NameUpArrow}, key.Filter{Focus: &n.Editor, Name: key.NameDownArrow})
		if !ok {
			break
		}
		ke, ok := e.(key.Event)
		if ok && ke.State == key.Press {
			if ke.Name == key.NameUpArrow {
				n.set(n.Value + step)
			} else {
				n.set(n.Value - step)
			}
			Notify(gtx)
		}
	}
	for {
		e, ok := n.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := e.(widget.ChangeEvent); ok {
			n.parse()
			Notify(gtx)
		}
	}
	return Row(gtx,
		Flexed(1, func(gtx C) D { return t.TextField(gtx, &n.Editor, id, placeholder) }),
		Rigid(Gap(t.Space(4), 0)),
		Rigid(func(gtx C) D {
			return t.IconButtonLabel(gtx, &n.Minus, Part(id, "minus"), "Decrease "+id, IconMinus, true)
		}),
		Rigid(func(gtx C) D {
			return t.IconButtonLabel(gtx, &n.Plus, Part(id, "plus"), "Increase "+id, IconPlus, true)
		}),
	)
}

// Validation describes one field error. Empty Message means valid.
type Validation struct{ FieldID, Message string }

// Required validates trimmed text; callers can combine it with domain rules.
func Required(id, value string) Validation {
	if strings.TrimSpace(value) == "" {
		return Validation{FieldID: id, Message: "Required"}
	}
	return Validation{FieldID: id}
}

// FieldError draws an error under a field when Message is nonempty.
func (t *Theme) FieldError(gtx C, v Validation) D {
	if v.Message == "" {
		return layout.Dimensions{}
	}
	return t.Text(gtx, v.Message, t.Type.Small, t.Bad, Regular)
}
