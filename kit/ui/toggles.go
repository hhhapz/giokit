package ui

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/hhhapz/giokit/kit"
)

// Checkbox is a check box with an optional label; the label is part of the
// click target. Flip your bool when the Clickable reports a click.
//
//	ui.Checkbox{ID: "agree", Label: "I agree", Checked: v.agreed}.Layout(gtx, th, &v.agree)
type Checkbox struct {
	ID      string
	Label   string
	Checked bool
}

func (x Checkbox) Layout(gtx C, th *kit.Theme, c *kit.Clickable) D {
	return th.Press(gtx, c, x.ID, func(gtx C, st kit.State) D {
		semantic.CheckBox.Add(gtx.Ops)
		kit.SetRole(gtx, x.ID, kit.RoleCheckbox)
		semantic.LabelOp(x.Label).Add(gtx.Ops)
		semantic.SelectedOp(x.Checked).Add(gtx.Ops)
		s := th.Styled(kit.Surface{Bg: kit.WithAlpha(th.Surface, uint8(255*st.Hover)), Border: kit.LerpColor(th.Dim, th.Muted, st.Hover), Radius: min(th.RadiusSm(), 5)})
		if x.Checked {
			s.Bg, s.Border = th.Good, th.Good
		}
		if st.Focused {
			s.Focus = th.Focus
		}
		return th.WithLabel(gtx, x.Label, func(gtx C) D {
			return kit.Box(gtx, th.CheckSize, s, func(gtx C) D {
				if !x.Checked {
					return D{}
				}
				return kit.IconCheck(gtx, th.IconSize, th.Bg)
			})
		})
	})
}

// Switch is an on/off toggle whose thumb slides. Flip your bool when the
// Clickable reports a click. The label is part of the click target.
type Switch struct {
	ID    string
	Label string
	On    bool
}

func (x Switch) Layout(gtx C, th *kit.Theme, c *kit.Clickable) D {
	return th.Press(gtx, c, x.ID, func(gtx C, st kit.State) D {
		semantic.Switch.Add(gtx.Ops)
		kit.SetRole(gtx, x.ID, kit.RoleSwitch)
		semantic.LabelOp(x.Label).Add(gtx.Ops)
		semantic.SelectedOp(x.On).Add(gtx.Ops)
		target := float32(0)
		if x.On {
			target = 1
		}
		p := c.Anim().ToEase(gtx, target, th.Dur(kit.DurNormal), kit.EaseInOut)
		return th.WithLabel(gtx, x.Label, func(gtx C) D {
			w, h := gtx.Dp(th.SwitchWidth), gtx.Dp(th.SwitchHeight)
			sz := image.Pt(w, h)
			track := kit.LerpColor(th.Tint(th.Surface, 0.06*st.Hover), th.Accent, p)
			paint.FillShape(gtx.Ops, track, clip.UniformRRect(image.Rectangle{Max: sz}, h/2).Op(gtx.Ops))
			if p < 1 {
				border := kit.LerpColor(th.Border, th.Muted, st.Hover)
				kit.Border(gtx, image.Rectangle{Max: sz}, h/2, max(1, gtx.Dp(th.Stroke)), kit.WithAlpha(border, uint8(255*(1-p))))
			}
			if st.Focused {
				th.FocusRing(gtx, sz, h/2)
			}
			pad := gtx.Dp(kit.Space1 - 1) // 3dp between track and thumb
			d := h - 2*pad
			x := pad + int(float32(w-2*pad-d)*p)
			thumb := kit.LerpColor(th.Dim, th.OnAccent, p) // Dim: the off thumb is decoration, not text
			paint.FillShape(gtx.Ops, thumb, clip.Ellipse(image.Rect(x, pad, x+d, pad+d)).Op(gtx.Ops))
			return D{Size: sz}
		})
	})
}

// Radio is one option of a single-choice group: give each option its own
// Clickable (a kit.Pool keyed by value works) and set your value when one
// is clicked.
type Radio struct {
	ID       string
	Label    string
	Selected bool
}

func (x Radio) Layout(gtx C, th *kit.Theme, c *kit.Clickable) D {
	return th.Press(gtx, c, x.ID, func(gtx C, st kit.State) D {
		semantic.RadioButton.Add(gtx.Ops)
		kit.SetRole(gtx, x.ID, kit.RoleRadio)
		semantic.LabelOp(x.Label).Add(gtx.Ops)
		semantic.SelectedOp(x.Selected).Add(gtx.Ops)
		target := float32(0)
		if x.Selected {
			target = 1
		}
		p := c.Anim().ToEase(gtx, target, th.Dur(kit.DurNormal), kit.EaseOutBack)
		return th.WithLabel(gtx, x.Label, func(gtx C) D {
			s := gtx.Dp(th.CheckSize)
			sz := image.Pt(s, s)
			paint.FillShape(gtx.Ops, kit.WithAlpha(th.Surface, uint8(255*st.Hover)), clip.Ellipse(image.Rectangle{Max: sz}).Op(gtx.Ops))
			border := kit.LerpColor(kit.LerpColor(th.Dim, th.Muted, st.Hover), th.Accent, min(1, p))
			if st.Focused {
				border = th.Focus
			}
			bw := max(1, gtx.Dp(th.Stroke))
			if x.Selected {
				bw = max(bw, gtx.Dp(th.Ring))
			}
			if st.Focused {
				bw = max(1, gtx.Dp(th.Ring))
			}
			kit.Border(gtx, image.Rectangle{Max: sz}, s/2, bw, border)
			if p > 0.01 {
				r := int(float32(gtx.Dp(kit.Space1)) * p)
				ctr := s / 2
				paint.FillShape(gtx.Ops, th.Accent, clip.Ellipse(image.Rect(ctr-r, ctr-r, ctr+r, ctr+r)).Op(gtx.Ops))
			}
			return D{Size: sz}
		})
	})
}

// Slider is a horizontal 0..1 slider. It is controlled: set Value from what
// SliderState.Update returns.
//
// Press or drag anywhere on it to set the value. It is focusable:
// Left/Right (or Down/Up) step by 5%, Home/End jump to the ends, and the
// wheel adjusts it while focused. Unfocused, or already at the end it is
// pushed toward, it leaves the wheel to its parent (the page scrolls).
type Slider struct {
	ID    string
	Label string // spoken name
	Value float32
}

// SliderState is a Slider's interaction state and last frame's geometry,
// so Update can resolve input before layout.
type SliderState struct {
	drag  gesture.Drag
	wheel gesture.Scroll
	quiet bool // focused by a click: no ring until the keyboard is used

	value     float32 // last laid out
	width, kr int     // last frame's width and knob radius
	disabled  bool    // laid out disabled: Update ignores input
}

// Dragging reports whether the slider is being dragged.
func (s *SliderState) Dragging() bool { return s.drag.Dragging() }

// sliderSteps is the keyboard and wheel resolution: 5% a key, 1/40 a notch.
const (
	sliderKeyStep = 0.05
	sliderWheel   = 40
	pxPerStep     = 60 // one wheel notch ≈ 150px ≈ 2.5 steps of 1/40
)

// Update returns the value input set since the last call (drag, click,
// wheel, keys). Call it in the handle pass.
func (s *SliderState) Update(gtx C) (float32, bool) {
	if s.disabled {
		return s.value, false // a disabled slider registers no handlers; nothing to read
	}
	v, changed := s.value, false
	set := func(nv float32) {
		nv = max(0, min(1, nv))
		if nv != v {
			v, changed = nv, true
		}
	}
	for {
		e, ok := s.drag.Update(gtx.Metric, gtx.Source, gesture.Horizontal)
		if !ok {
			break
		}
		if e.Kind == pointer.Press && !gtx.Focused(s) {
			gtx.Execute(key.FocusCmd{Tag: s}) // so the wheel and arrows work next
			s.quiet = true
		}
		if (e.Kind == pointer.Press || e.Kind == pointer.Drag) && s.width > 2*s.kr {
			set((e.Position.X - float32(s.kr)) / float32(s.width-2*s.kr))
		}
	}
	if gtx.Focused(s) {
		// Wheel up raises the value. Allow as much scroll as can still be
		// used in each direction, so a slider at its end passes the wheel on.
		up, down := int((1-v)*sliderWheel*pxPerStep), int(v*sliderWheel*pxPerStep)
		if d := s.wheel.Update(gtx.Metric, gtx.Source, gtx.Now, gesture.Vertical,
			pointer.ScrollRange{}, pointer.ScrollRange{Min: -up, Max: down}); d != 0 {
			set(v - float32(d)/(sliderWheel*pxPerStep))
		}
	}
	for {
		e, ok := gtx.Event(
			key.FocusFilter{Target: s},
			key.Filter{Focus: s, Name: key.NameLeftArrow}, key.Filter{Focus: s, Name: key.NameRightArrow},
			key.Filter{Focus: s, Name: key.NameUpArrow}, key.Filter{Focus: s, Name: key.NameDownArrow},
			key.Filter{Focus: s, Name: key.NameHome}, key.Filter{Focus: s, Name: key.NameEnd},
		)
		if !ok {
			break
		}
		ke, ok := e.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		s.quiet = false
		switch ke.Name {
		case key.NameLeftArrow, key.NameDownArrow:
			set(v - sliderKeyStep)
		case key.NameRightArrow, key.NameUpArrow:
			set(v + sliderKeyStep)
		case key.NameHome:
			set(0)
		case key.NameEnd:
			set(1)
		}
	}
	if changed {
		s.value = v
		kit.Notify(gtx)
	}
	return v, changed
}

func (x Slider) Layout(gtx C, th *kit.Theme, s *SliderState) D {
	h := gtx.Dp(th.SwitchHeight)
	w := gtx.Constraints.Max.X
	kr := gtx.Dp(kit.Space2)
	s.value, s.width, s.kr = max(0, min(1, x.Value)), w, kr
	s.disabled = !gtx.Enabled()
	sz := image.Pt(w, h)

	track := gtx.Dp(kit.Space1)
	y := h / 2
	cx := kr + int(float32(w-2*kr)*s.value)
	paint.FillShape(gtx.Ops, th.Surface, clip.UniformRRect(image.Rect(kr, y-track/2, w-kr, y+track/2), track/2).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, th.Accent, clip.UniformRRect(image.Rect(kr, y-track/2, cx, y+track/2), track/2).Op(gtx.Ops))
	knob := image.Rect(cx-kr, y-kr, cx+kr, y+kr)
	kc := th.Fg
	if s.drag.Dragging() {
		kc = th.Accent
	}
	paint.FillShape(gtx.Ops, kc, clip.Ellipse(knob).Op(gtx.Ops))
	if !gtx.Focused(s) {
		s.quiet = false
	}
	if gtx.Focused(s) && !s.quiet {
		ring := knob.Inset(-gtx.Dp(kit.Space1 - 1))
		kit.Border(gtx, ring, ring.Dx()/2, max(1, gtx.Dp(th.Ring)), th.Focus)
	}
	defer clip.Rect(image.Rectangle{Max: sz}).Push(gtx.Ops).Pop()
	if !s.disabled { // disabled: no handlers, so no drag, wheel, keys or Tab stop
		s.drag.Add(gtx.Ops)
		if gtx.Focused(s) {
			s.wheel.Add(gtx.Ops) // only claim the wheel while we use it
		}
		event.Op(gtx.Ops, s)
	}
	if x.Label != "" {
		semantic.LabelOp(x.Label).Add(gtx.Ops)
	}
	if !gtx.Enabled() {
		// A nested area: Gio drops the semantics of an area whose handler
		// gets no events, as a disabled one doesn't.
		defer clip.Rect(image.Rectangle{Max: sz}).Push(gtx.Ops).Pop()
		kit.Tag(gtx, x.ID)
		kit.SetRole(gtx, x.ID, kit.RoleSlider)
		semantic.EnabledOp(false).Add(gtx.Ops)
		return D{Size: sz}
	}
	pointer.CursorPointer.Add(gtx.Ops)
	kit.Tag(gtx, x.ID)
	kit.SetRole(gtx, x.ID, kit.RoleSlider)
	kit.FocusID(gtx, s, x.ID)
	return D{Size: sz}
}
