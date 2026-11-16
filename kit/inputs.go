package kit

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// withLabel lays out a control followed by an optional text label.
func (t *Theme) withLabel(gtx C, label string, ctl W) D {
	if label == "" {
		return ctl(gtx)
	}
	return Row(gtx,
		Rigid(ctl),
		Rigid(Gap(t.Space(8), 0)),
		Rigid(func(gtx C) D { return t.Text(gtx, label, t.Type.Body, t.Fg, Regular) }),
	)
}

// Switch is an on/off toggle; the thumb slides. Toggle your bool when
// c.Clicked. The label (optional) is part of the click target.
func (t *Theme) Switch(gtx C, c *Clickable, id, label string, on bool) D {
	return t.press(gtx, c, id, func(gtx C, st State) D {
		semantic.Switch.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.SelectedOp(on).Add(gtx.Ops)
		target := float32(0)
		if on {
			target = 1
		}
		p := c.anim.ToEase(gtx, target, t.Dur(DurNormal), EaseInOut)
		return t.withLabel(gtx, label, func(gtx C) D {
			w, h := gtx.Dp(34), gtx.Dp(20)
			sz := image.Pt(w, h)
			track := LerpColor(t.tint(t.Surface, 0.06*st.Hover), t.Accent, p)
			paint.FillShape(gtx.Ops, track, clip.UniformRRect(image.Rectangle{Max: sz}, h/2).Op(gtx.Ops))
			if p < 1 {
				border := LerpColor(t.Border, t.Muted, st.Hover)
				Border(gtx, image.Rectangle{Max: sz}, h/2, max(1, gtx.Dp(t.Stroke)), WithAlpha(border, uint8(255*(1-p))))
			}
			if st.Focused {
				t.ring(gtx, sz, h/2)
			}
			pad := gtx.Dp(3)
			d := h - 2*pad
			x := pad + int(float32(w-2*pad-d)*p)
			thumb := LerpColor(t.Dim, t.OnAccent, p) // Dim: the off thumb is decoration, not text
			paint.FillShape(gtx.Ops, thumb, clip.Ellipse(image.Rect(x, pad, x+d, pad+d)).Op(gtx.Ops))
			return D{Size: sz}
		})
	})
}

// ring draws the theme's focus ring just inside a rounded rect of size sz.
func (t *Theme) ring(gtx C, sz image.Point, r int) {
	Border(gtx, image.Rectangle{Max: sz}, r, max(1, gtx.Dp(t.Ring)), t.Focus)
}

// Radio is one option of a single-choice group: give each option its own
// Clickable (a Pool keyed by value works) and set your value when clicked.
func (t *Theme) Radio(gtx C, c *Clickable, id, label string, selected bool) D {
	return t.press(gtx, c, id, func(gtx C, st State) D {
		semantic.RadioButton.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.SelectedOp(selected).Add(gtx.Ops)
		target := float32(0)
		if selected {
			target = 1
		}
		p := c.anim.ToEase(gtx, target, t.Dur(DurNormal), EaseOutBack)
		return t.withLabel(gtx, label, func(gtx C) D {
			s := gtx.Dp(18)
			sz := image.Pt(s, s)
			paint.FillShape(gtx.Ops, WithAlpha(t.Surface, uint8(255*st.Hover)), clip.Ellipse(image.Rectangle{Max: sz}).Op(gtx.Ops))
			border := LerpColor(LerpColor(t.Dim, t.Muted, st.Hover), t.Accent, min(1, p))
			if st.Focused {
				border = t.Focus
			}
			bw := max(1, gtx.Dp(t.Stroke))
			if selected {
				bw = max(bw, gtx.Dp(2))
			}
			if st.Focused {
				bw = max(1, gtx.Dp(t.Ring))
			}
			Border(gtx, image.Rectangle{Max: sz}, s/2, bw, border)
			if p > 0.01 {
				r := int(float32(gtx.Dp(4)) * p)
				ctr := s / 2
				paint.FillShape(gtx.Ops, t.Accent, clip.Ellipse(image.Rect(ctr-r, ctr-r, ctr+r, ctr+r)).Op(gtx.Ops))
			}
			return D{Size: sz}
		})
	})
}

// TextField is a single-line input around a widget.Editor (set e.SingleLine
// and e.Submit yourself if you want Enter to submit). The border turns into a
// focus ring while the editor is focused; placeholder shows while empty.
func (t *Theme) TextField(gtx C, e *widget.Editor, id, placeholder string) D {
	return t.TextFieldIcon(gtx, e, id, nil, placeholder)
}

// TextFieldIcon is TextField with a leading icon (e.g. IconSearch).
func (t *Theme) TextFieldIcon(gtx C, e *widget.Editor, id string, icon Icon, placeholder string) D {
	return t.field(gtx, e, id, icon, placeholder, 0)
}

// TextArea is a multi-line input (Enter inserts a newline) at least lines
// lines tall; it grows with its text. e.SingleLine must be false.
func (t *Theme) TextArea(gtx C, e *widget.Editor, id, placeholder string, lines int) D {
	return t.field(gtx, e, id, nil, placeholder, lines)
}

func (t *Theme) field(gtx C, e *widget.Editor, id string, icon Icon, placeholder string, lines int) D {
	trackFocus(gtx, e) // keys the editor leaves resolve from this context
	// A press anywhere in the field, padding included, focuses the editor:
	// the target is the whole field, not just the text line. Read it before
	// drawing (rule 1). The tag is an address inside the editor, so it is
	// stable and unique without state of our own.
	pad := &e.Submit
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: pad, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pe, ok := ev.(pointer.Event); ok && pe.Kind == pointer.Press && gtx.Enabled() && !gtx.Focused(e) {
			gtx.Execute(key.FocusCmd{Tag: e})
			Notify(gtx)
		}
	}
	// Lay out the editor first: it handles this frame's input (a click that
	// focuses it), so the ring below reflects it (rule 1).
	fm := op.Record(gtx.Ops)
	fdims := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		// The id goes on the whole field so tests can click anywhere in it.
		m := op.Record(gtx.Ops)
		dims := layout.Inset{Top: t.control(7), Bottom: t.control(7), Left: t.Space(10), Right: t.Space(10)}.Layout(gtx, func(gtx C) D {
			return Row(gtx,
				Rigid(func(gtx C) D {
					if icon == nil {
						return D{}
					}
					return layout.Inset{Right: 8}.Layout(gtx, func(gtx C) D { return icon(gtx, 14, t.Dim) })
				}),
				Flexed(1, func(gtx C) D {
					if lines > 0 { // line height ≈ 1.35 × the body size
						gtx.Constraints.Min.Y = max(gtx.Constraints.Min.Y, int(float32(gtx.Sp(t.Type.Body))*1.35*float32(lines)))
					}
					ed := material.Editor(t.mat, e, placeholder)
					ed.TextSize = t.Type.Body
					ed.Color = t.Fg
					ed.HintColor = t.Dim
					ed.SelectionColor = WithAlpha(t.Accent, 80)
					return ed.Layout(gtx)
				}),
			)
		})
		call := m.Stop()
		defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
		if gtx.Enabled() {
			// Only while enabled: Gio drops the semantics (the id) of an area
			// whose handler asks for no events, as a disabled one doesn't.
			event.Op(gtx.Ops, pad)
			pointer.CursorText.Add(gtx.Ops)
		}
		semantic.EnabledOp(gtx.Enabled()).Add(gtx.Ops)
		Tag(gtx, id)
		call.Add(gtx.Ops)
		return dims
	}(gtx)
	fcall := fm.Stop()
	r := t.Radius
	if lines > 1 {
		r = min(r, t.RadiusLg()) // a multi-line field is a box, never a capsule
	}
	s := t.Styled(Surface{Bg: t.Surface, Border: t.Border, Radius: r})
	if gtx.Focused(e) {
		s.Focus = t.Focus
	}
	return s.Layout(gtx, func(gtx C) D { fcall.Add(gtx.Ops); return fdims })
}

// Slider is the state of a horizontal 0..1 slider.
type Slider struct {
	Value float32
	drag  gesture.Drag
	wheel gesture.Scroll
	quiet bool // focused by a click: no ring until the keyboard is used
}

// Dragging reports whether the slider is being dragged.
func (s *Slider) Dragging() bool { return s.drag.Dragging() }

// Slider lays out a slider. Press or drag anywhere on it to set the value.
// It is focusable: Left/Right (or Down/Up) step by 5%, Home/End jump to the
// ends, and the wheel adjusts it while focused. Unfocused, or already at the
// end it is being pushed toward, it leaves the wheel to its parent (the page
// scrolls): it only asks for input it will use.
// Read s.Value after layout, or compare it with last frame's value.
func (t *Theme) Slider(gtx C, s *Slider, id string) D {
	h := gtx.Dp(20)
	w := gtx.Constraints.Max.X
	kr := gtx.Dp(8)
	sz := image.Pt(w, h)
	set := func(v float32) {
		v = max(0, min(1, v))
		if v != s.Value {
			s.Value = v
			Notify(gtx)
		}
	}
	// ---- input ----
	for {
		e, ok := s.drag.Update(gtx.Metric, gtx.Source, gesture.Horizontal)
		if !ok {
			break
		}
		if e.Kind == pointer.Press && !gtx.Focused(s) {
			gtx.Execute(key.FocusCmd{Tag: s}) // so the wheel and arrows work next
			s.quiet = true
		}
		if (e.Kind == pointer.Press || e.Kind == pointer.Drag) && w > 2*kr {
			set((e.Position.X - float32(kr)) / float32(w-2*kr))
		}
	}
	if gtx.Focused(s) {
		// Wheel up raises the value. Scroll ranges are in pixels; allow as
		// much as can still be used in each direction, so a slider at its end
		// passes the wheel on (scroll chaining).
		const pxPerStep = 60 // one wheel notch ≈ 150px ≈ 2.5 steps of 1/40
		up, down := int((1-s.Value)*40*pxPerStep), int(s.Value*40*pxPerStep)
		if d := s.wheel.Update(gtx.Metric, gtx.Source, gtx.Now, gesture.Vertical,
			pointer.ScrollRange{}, pointer.ScrollRange{Min: -up, Max: down}); d != 0 {
			set(s.Value - float32(d)/(40*pxPerStep))
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
			set(s.Value - 0.05)
		case key.NameRightArrow, key.NameUpArrow:
			set(s.Value + 0.05)
		case key.NameHome:
			set(0)
		case key.NameEnd:
			set(1)
		}
	}
	// ---- draw ----
	th := gtx.Dp(4)
	y := h / 2
	x := kr + int(float32(w-2*kr)*s.Value)
	paint.FillShape(gtx.Ops, t.Surface, clip.UniformRRect(image.Rect(kr, y-th/2, w-kr, y+th/2), th/2).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, t.Accent, clip.UniformRRect(image.Rect(kr, y-th/2, x, y+th/2), th/2).Op(gtx.Ops))
	knob := image.Rect(x-kr, y-kr, x+kr, y+kr)
	kc := t.Fg
	if s.drag.Dragging() {
		kc = t.Accent
	}
	paint.FillShape(gtx.Ops, kc, clip.Ellipse(knob).Op(gtx.Ops))
	if !gtx.Focused(s) {
		s.quiet = false
	}
	if gtx.Focused(s) && !s.quiet {
		ring := knob.Inset(-gtx.Dp(3))
		Border(gtx, ring, ring.Dx()/2, max(1, gtx.Dp(t.Ring)), t.Focus)
	}
	defer clip.Rect(image.Rectangle{Max: sz}).Push(gtx.Ops).Pop()
	s.drag.Add(gtx.Ops)
	if gtx.Focused(s) {
		s.wheel.Add(gtx.Ops) // only claim the wheel while we use it
	}
	event.Op(gtx.Ops, s)
	if !gtx.Enabled() {
		return disabledArea(gtx, id, func(C) D { return D{Size: sz} })
	}
	pointer.CursorPointer.Add(gtx.Ops)
	Tag(gtx, id)
	return D{Size: sz}
}
