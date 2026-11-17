package kit

import (
	"cmp"
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// WithLabel lays out a control followed by an optional text label.
func (t *Theme) WithLabel(gtx C, label string, ctl W) D {
	if label == "" {
		return ctl(gtx)
	}
	return Row(gtx,
		Rigid(ctl),
		Rigid(Gap(t.Space(Space2), 0)),
		Rigid(func(gtx C) D { return t.Text(gtx, label, t.Type.Body, t.Fg, Regular) }),
	)
}

// FocusRing draws the theme's focus ring just inside a rounded rect of size sz
// with corner radius r in pixels.
func (t *Theme) FocusRing(gtx C, sz image.Point, r int) {
	Border(gtx, image.Rectangle{Max: sz}, r, max(1, gtx.Dp(t.Ring)), t.Focus)
}

// EditorField is how an editor field looks: the placeholder shown while
// empty, an optional leading icon, and a minimum height in lines (0 is one
// line; set Editor.SingleLine yourself).
type EditorField struct {
	Placeholder string
	Label       string // spoken name; default Placeholder
	Icon        Icon
	Lines       int
}

// EditorField lays out e as a field: theme chrome, a ring while focused,
// and the whole field (padding included) as the click target that focuses
// the editor. ui.TextField, ui.TextArea and every editor-based component
// are built on it.
func (t *Theme) EditorField(gtx C, e *widget.Editor, id string, f EditorField) D {
	icon, placeholder, lines := f.Icon, f.Placeholder, f.Lines
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
		dims := layout.Inset{Top: t.control(7), Bottom: t.control(7), Left: t.Space(Space2p5), Right: t.Space(Space2p5)}.Layout(gtx, func(gtx C) D {
			return Row(gtx,
				Rigid(func(gtx C) D {
					if icon == nil {
						return D{}
					}
					return layout.Inset{Right: t.Space(Space2)}.Layout(gtx, func(gtx C) D { return icon(gtx, t.IconSize, t.Dim) })
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
		// The tagged area is the editor as screen readers and tests see it:
		// role and spoken name on the node that carries the id.
		semantic.Editor.Add(gtx.Ops)
		if label := cmp.Or(f.Label, placeholder); label != "" {
			semantic.LabelOp(label).Add(gtx.Ops)
		}
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
