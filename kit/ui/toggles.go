package ui

import (
	"image"

	"github.com/hhhapz/giokit/gio/gesture"
	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/io/pointer"
	"github.com/hhhapz/giokit/gio/io/semantic"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"
	"github.com/hhhapz/giokit/gio/unit"

	"github.com/hhhapz/giokit/kit"
)

// Checkbox is a check box with an optional label; the label is part of the
// click target. Flip your bool when the Clickable reports a click.
//
//	ui.Checkbox{ID: "agree", Label: "I agree", Checked: v.agreed}.Layout(gtx, th, &v.agree)
type Checkbox struct {
	ID      string // the control's semantic id
	Label   string // drawn beside the box and spoken; "" draws the box alone
	Checked bool   // the controlled value
}

// checkMaxRadius caps the box's corner radius so a round theme still
// draws a box, not a radio-like circle.
const checkMaxRadius unit.Dp = 5

// Layout draws the box, checked or not, and its label for c's state, and
// registers c as the press and focus target. It does not flip Checked:
// read c.Clicked in the handle pass and flip your value.
func (cb Checkbox) Layout(gtx layout.Context, th *kit.Theme, c *kit.Clickable) layout.Dimensions {
	return th.Press(gtx, c, cb.ID, func(gtx layout.Context, st kit.State) layout.Dimensions {
		describe(gtx, kit.RoleCheckbox, cb.Label)
		semantic.SelectedOp(cb.Checked).Add(gtx.Ops)
		s := th.Styled(kit.Surface{Bg: kit.WithAlpha(th.Surface, uint8(255*st.Hover)), Border: kit.LerpColor(th.Dim, th.Muted, st.Hover), Radius: min(th.RadiusSm(), checkMaxRadius)})
		if cb.Checked {
			s.Bg, s.Border = th.Good, th.Good
		}
		if st.Focused {
			s.Focus = th.Focus
		}
		return th.WithLabel(gtx, cb.Label, func(gtx layout.Context) layout.Dimensions {
			return kit.Box(gtx, th.CheckSize, s, func(gtx layout.Context) layout.Dimensions {
				if !cb.Checked {
					return layout.Dimensions{}
				}
				return kit.IconCheck(gtx, th.IconSize, th.Bg)
			})
		})
	})
}

// Switch geometry and feedback.
const (
	switchHoverTint          = 0.06 // how far hover lifts the off track toward Fg
	switchThumbInset unit.Dp = 3    // between the track's edge and the thumb
)

// Switch is an on/off toggle whose thumb slides. Flip your bool when the
// Clickable reports a click. The label is part of the click target.
type Switch struct {
	ID    string // the control's semantic id
	Label string // drawn beside the track and spoken
	On    bool   // the controlled value; the thumb animates toward it
}

// Layout draws the track, the thumb and the label for c's state, and
// registers c as the press and focus target. It does not flip On: read
// c.Clicked in the handle pass and flip your value.
func (sw Switch) Layout(gtx layout.Context, th *kit.Theme, c *kit.Clickable) layout.Dimensions {
	return th.Press(gtx, c, sw.ID, func(gtx layout.Context, st kit.State) layout.Dimensions {
		describe(gtx, kit.RoleSwitch, sw.Label)
		semantic.SelectedOp(sw.On).Add(gtx.Ops)
		target := float32(0)
		if sw.On {
			target = 1
		}
		p := c.Anim().ToEase(gtx, target, th.Dur(kit.DurNormal), kit.EaseInOut)
		return th.WithLabel(gtx, sw.Label, func(gtx layout.Context) layout.Dimensions {
			w, h := gtx.Dp(th.SwitchWidth), gtx.Dp(th.SwitchHeight)
			sz := image.Pt(w, h)
			track := kit.LerpColor(th.Tint(th.Surface, switchHoverTint*st.Hover), th.Accent, p)
			paint.FillShape(gtx.Ops, track, clip.UniformRRect(image.Rectangle{Max: sz}, h/2).Op(gtx.Ops))
			if p < 1 {
				border := kit.LerpColor(th.Border, th.Muted, st.Hover)
				kit.Border(gtx, image.Rectangle{Max: sz}, h/2, max(1, gtx.Dp(th.Stroke)), kit.WithAlpha(border, uint8(255*(1-p))))
			}
			if st.Focused {
				th.FocusRing(gtx, sz, h/2)
			}
			pad := gtx.Dp(switchThumbInset)
			d := h - 2*pad
			x := pad + int(float32(w-2*pad-d)*p)
			thumb := kit.LerpColor(th.Dim, th.OnAccent, p) // Dim: the off thumb is decoration, not text
			paint.FillShape(gtx.Ops, thumb, clip.Ellipse(image.Rect(x, pad, x+d, pad+d)).Op(gtx.Ops))
			return layout.Dimensions{Size: sz}
		})
	})
}

// radioDotMin is the animation progress below which the selected dot is
// not drawn: smaller, it would be a speck.
const radioDotMin = 0.01

// Radio is one option of a single-choice group: give each option its own
// Clickable (a kit.Pool keyed by value works) and set your value when one
// is clicked.
type Radio struct {
	ID       string // the option's semantic id
	Label    string // drawn beside the circle and spoken
	Selected bool   // whether this option is the group's value
}

// Layout draws the circle, with a dot when Selected, and the label for c's
// state, and registers c as the press and focus target. Read c.Clicked and
// set your value to this option.
func (r Radio) Layout(gtx layout.Context, th *kit.Theme, c *kit.Clickable) layout.Dimensions {
	return th.Press(gtx, c, r.ID, func(gtx layout.Context, st kit.State) layout.Dimensions {
		describe(gtx, kit.RoleRadio, r.Label)
		semantic.SelectedOp(r.Selected).Add(gtx.Ops)
		target := float32(0)
		if r.Selected {
			target = 1
		}
		p := c.Anim().ToEase(gtx, target, th.Dur(kit.DurNormal), kit.EaseOutBack)
		return th.WithLabel(gtx, r.Label, func(gtx layout.Context) layout.Dimensions {
			s := gtx.Dp(th.CheckSize)
			sz := image.Pt(s, s)
			paint.FillShape(gtx.Ops, kit.WithAlpha(th.Surface, uint8(255*st.Hover)), clip.Ellipse(image.Rectangle{Max: sz}).Op(gtx.Ops))
			// The border thickens to the ring width when selected or focused.
			border := kit.LerpColor(kit.LerpColor(th.Dim, th.Muted, st.Hover), th.Accent, min(1, p))
			bw := max(1, gtx.Dp(th.Stroke))
			if r.Selected || st.Focused {
				bw = max(bw, gtx.Dp(th.Ring))
			}
			if st.Focused {
				border = th.Focus
			}
			kit.Border(gtx, image.Rectangle{Max: sz}, s/2, bw, border)
			if p > radioDotMin {
				r := int(float32(gtx.Dp(kit.Space1)) * p)
				ctr := s / 2
				paint.FillShape(gtx.Ops, th.Accent, clip.Ellipse(image.Rect(ctr-r, ctr-r, ctr+r, ctr+r)).Op(gtx.Ops))
			}
			return layout.Dimensions{Size: sz}
		})
	})
}

// Slider is a horizontal slider over 0..1. It is controlled: set Value
// from what SliderState.Update returns. Values outside 0..1 are drawn
// clamped.
//
// Press or drag anywhere on it to set the value. It is focusable:
// Left/Right (or Down/Up) step by 5%, Home/End jump to the ends, and the
// wheel adjusts it while focused. Unfocused, or already at the end it is
// pushed toward, it leaves the wheel to its parent (the page scrolls).
// Keys it does not use (Enter, shortcuts) reach the enclosing KeyContext.
type Slider struct {
	ID    string  // the slider's semantic id
	Label string  // spoken name
	Value float32 // the controlled value, 0..1
}

// SliderState is a Slider's interaction state and last frame's geometry,
// so Update can resolve input before layout. The zero value is ready to
// use. It is its own event tag: do not copy it after first use.
type SliderState struct {
	drag  gesture.Drag
	wheel gesture.Scroll
	ring  kit.FocusVisible // a click focuses without a ring until a key is used

	value     float32 // last laid out
	width, kr int     // last frame's width and knob radius
	disabled  bool    // laid out disabled: Update ignores input
}

// Dragging reports whether the slider is being dragged.
func (s *SliderState) Dragging() bool { return s.drag.Dragging() }

// The slider's keyboard and wheel resolution.
const (
	sliderKeyStep    = 0.05 // the value change of one arrow key
	sliderWheelSteps = 40   // wheel steps across the whole range
	// sliderWheelStepPx is the scroll distance of one wheel step: a wheel
	// notch (about 150px) moves the value about 2.5 steps.
	sliderWheelStepPx = 60
)

// sliderRingGap is the space between the knob and its focus ring.
const sliderRingGap unit.Dp = 3

// Update returns the value input set since the last call (drag, click,
// wheel, keys). Call it in the handle pass. All of a frame's events are
// applied in order, and the last value is returned.
func (s *SliderState) Update(gtx layout.Context) (float32, bool) {
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
			s.ring.Hide()
		}
		if (e.Kind == pointer.Press || e.Kind == pointer.Drag) && s.width > 2*s.kr {
			set((e.Position.X - float32(s.kr)) / float32(s.width-2*s.kr))
		}
	}
	if gtx.Focused(s) {
		// Wheel up raises the value. Allow as much scroll as can still be
		// used in each direction, so a slider at its end passes the wheel on.
		const rangePx = sliderWheelSteps * sliderWheelStepPx
		up, down := int((1-v)*rangePx), int(v*rangePx)
		if d := s.wheel.Update(gtx.Metric, gtx.Source, gtx.Now, gesture.Vertical,
			pointer.ScrollRange{}, pointer.ScrollRange{Min: -up, Max: down}); d != 0 {
			set(v - float32(d)/rangePx)
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
		s.ring.Show()
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

// Layout draws the track and knob at Value across the full available width
// and registers s for drags, focus, keys and (while focused) the wheel. It
// keeps the geometry and Value for s.Update, which consumes that input.
// Inside th.Disable it registers no handlers.
func (sl Slider) Layout(gtx layout.Context, th *kit.Theme, s *SliderState) layout.Dimensions {
	h := gtx.Dp(th.SwitchHeight)
	w := gtx.Constraints.Max.X
	kr := gtx.Dp(kit.Space2)
	s.value, s.width, s.kr = max(0, min(1, sl.Value)), w, kr
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
	if s.ring.Visible(gtx.Focused(s)) {
		ring := knob.Inset(-gtx.Dp(sliderRingGap))
		kit.Border(gtx, ring, ring.Dx()/2, max(1, gtx.Dp(th.Ring)), th.Focus)
	}
	defer clip.Rect(image.Rectangle{Max: sz}).Push(gtx.Ops).Pop()
	if s.disabled {
		// No handlers, so no drag, wheel, keys or Tab stop. The semantics go
		// on a nested area: Gio drops those of an area whose handler gets no
		// events, as a disabled one doesn't.
		defer clip.Rect(image.Rectangle{Max: sz}).Push(gtx.Ops).Pop()
		semantic.EnabledOp(false).Add(gtx.Ops)
	} else {
		s.drag.Add(gtx.Ops)
		if gtx.Focused(s) {
			s.wheel.Add(gtx.Ops) // only claim the wheel while we use it
		}
		event.Op(gtx.Ops, s)
		pointer.CursorPointer.Add(gtx.Ops)
		kit.FocusID(gtx, s, sl.ID)
	}
	if sl.Label != "" {
		semantic.LabelOp(sl.Label).Add(gtx.Ops)
	}
	kit.Tag(gtx, sl.ID)
	kit.RoleSlider.Add(gtx.Ops)
	return layout.Dimensions{Size: sz}
}
