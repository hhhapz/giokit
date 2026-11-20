package kit

import (
	"cmp"
	"image"

	"github.com/hhhapz/giokit/gio/font"
	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/io/pointer"
	"github.com/hhhapz/giokit/gio/io/semantic"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/unit"
	"github.com/hhhapz/giokit/gio/widget"
)

// WithLabel lays out a control followed by an optional text label.
func (t *Theme) WithLabel(gtx layout.Context, label string, ctl layout.Widget) layout.Dimensions {
	if label == "" {
		return ctl(gtx)
	}
	return Row(gtx,
		Rigid(ctl),
		Rigid(Gap(t.Space(Space2), 0)),
		Rigid(func(gtx layout.Context) layout.Dimensions { return t.Text(gtx, label, t.Type.Body, t.Fg, Regular) }),
	)
}

// FocusRing draws the theme's focus ring just inside a rounded rect of size sz
// with corner radius r in pixels.
func (t *Theme) FocusRing(gtx layout.Context, sz image.Point, r int) {
	Border(gtx, image.Rectangle{Max: sz}, r, max(1, gtx.Dp(t.Ring)), t.Focus)
}

// fieldPadY is the vertical padding, before density and control size, of
// fields and field-like triggers. Buttons use the same value so a button
// and a field of one size share a height.
const fieldPadY unit.Dp = 7

// FieldPad is the vertical padding of a field, or of a field-like control
// (a select's trigger, a button, a navigation row), at size: fieldPadY
// scaled by density and size as ControlPad does. Controls of one size
// that use it share a height.
func (t *Theme) FieldPad(size ControlSize) unit.Dp { return t.ControlPad(size, fieldPadY) }

// Field draws field chrome around w: full width, the control padding for
// the theme's control size, surface and border, a hover tint from
// st.Hover, and the focus ring while st.Focused. Field-like triggers
// (select, date) are built on it; TextField draws the same chrome.
func (t *Theme) Field(gtx layout.Context, st State, w layout.Widget) layout.Dimensions {
	return t.SizedField(gtx, ControlDefault, st, w)
}

// SizedField is Field at a control size (ControlDefault: the theme's), for
// components with a per-instance Size.
func (t *Theme) SizedField(gtx layout.Context, size ControlSize, st State, w layout.Widget) layout.Dimensions {
	s := t.Styled(Surface{Bg: t.StateColor(t.Surface, State{Hover: st.Hover}), Border: t.Border, Radius: t.Radius})
	if st.Focused {
		s.Focus = t.Focus
	}
	// Full width, natural height: a caller's minimum height (a window's)
	// would stretch the field and leave its popup no room.
	gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, 0)
	return s.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		v := t.FieldPad(size)
		return layout.Inset{Top: v, Bottom: v, Left: t.Space(Space2p5), Right: t.Space(Space2)}.Layout(gtx, w)
	})
}

// Editor text defaults.
const (
	// bodyLineHeight is the line height of wrapping body text, as a multiple
	// of the body size: paragraphs and multi-line editors.
	bodyLineHeight = 1.35
	// selectionAlpha is the alpha of the accent tint behind selected text.
	selectionAlpha = 80
)

// EditorField is how an editor field looks: the placeholder shown while
// empty, an optional leading icon, and a minimum height in lines.
type EditorField struct {
	Placeholder string      // dim hint drawn while the editor is empty
	Label       string      // spoken name; default Placeholder
	Size        ControlSize // height class; ControlDefault is the theme's
	Icon        Icon        // optional, before the text
	// Lines is the minimum height in lines; 0 is one line. It does not make
	// the editor multi-line: set Editor.SingleLine yourself.
	Lines int
	// Role is the field's role: RoleTextbox by default; a combobox's
	// field is RoleCombobox.
	Role Role
}

// EditorField lays out e as a field: theme chrome, a ring while focused,
// and the whole field (padding included) as the click target that focuses
// the editor. ui.TextField, ui.TextArea and every editor-based component
// are built on it.
func (t *Theme) EditorField(gtx layout.Context, e *widget.Editor, id string, f EditorField) layout.Dimensions {
	trackFocus(gtx, e) // keys the editor leaves resolve from this context
	if gtx.Focused(e) {
		if f := frameOf(gtx); f != nil {
			f.typing = true // printable keys are text, not shortcuts
		}
	}
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
	m := op.Record(gtx.Ops)
	dims := t.editorBody(gtx, e, pad, id, f)
	call := m.Stop()
	r := t.Radius
	if f.Lines > 1 {
		r = min(r, t.RadiusLg()) // a multi-line field is a box, never a capsule
	}
	s := t.Styled(Surface{Bg: t.Surface, Border: t.Border, Radius: r})
	if gtx.Focused(e) {
		s.Focus = t.Focus
	}
	return s.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		call.Add(gtx.Ops)
		return dims
	})
}

// editorBody lays out the inside of an EditorField at full width: padding,
// icon and editor, on one area that carries the id, the editor's semantics
// and, while enabled, the press target pad that focuses it.
func (t *Theme) editorBody(gtx layout.Context, e *widget.Editor, pad event.Tag, id string, f EditorField) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	m := op.Record(gtx.Ops)
	v := t.FieldPad(f.Size)
	dims := layout.Inset{Top: v, Bottom: v, Left: t.Space(Space2p5), Right: t.Space(Space2p5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return Row(gtx,
			Rigid(func(gtx layout.Context) layout.Dimensions {
				if f.Icon == nil {
					return layout.Dimensions{}
				}
				return layout.Inset{Right: t.Space(Space2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return f.Icon(gtx, t.IconSize, t.Dim) })
			}),
			Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if f.Lines > 0 {
					lineH := float32(gtx.Sp(t.Type.Body)) * bodyLineHeight
					gtx.Constraints.Min.Y = max(gtx.Constraints.Min.Y, int(lineH*float32(f.Lines)))
				}
				return t.editor(gtx, e, f.Placeholder)
			}),
		)
	})
	call := m.Stop()
	// The id goes on the whole field so tests can click anywhere in it.
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	if gtx.Enabled() {
		// Only while enabled: a disabled field takes no clicks and shows no
		// text cursor.
		event.Op(gtx.Ops, pad)
		pointer.CursorText.Add(gtx.Ops)
	}
	semantic.EnabledOp(gtx.Enabled()).Add(gtx.Ops)
	// The tagged area is the editor as screen readers and tests see it:
	// role and spoken name on the node that carries the id.
	cmp.Or(f.Role, RoleTextbox).Add(gtx.Ops)
	if label := cmp.Or(f.Label, f.Placeholder); label != "" {
		semantic.LabelOp(label).Add(gtx.Ops)
	}
	Tag(gtx, id)
	FocusID(gtx, e, id)
	call.Add(gtx.Ops)
	return dims
}

// editor draws e in the UI face at body size, with hint drawn while it is
// empty, and the editor at least as large as the hint.
func (t *Theme) editor(gtx layout.Context, e *widget.Editor, hint string) layout.Dimensions {
	f := font.Font{Typeface: t.UI}
	m := op.Record(gtx.Ops)
	maxLines := 0
	if e.SingleLine {
		maxLines = 1
	}
	hd := widget.Label{Alignment: e.Alignment, MaxLines: maxLines}.Layout(gtx, t.Shaper, f, t.Type.Body, hint, colorOp(gtx, t.Dim))
	hintCall := m.Stop()
	gtx.Constraints.Min = image.Pt(max(gtx.Constraints.Min.X, hd.Size.X), max(gtx.Constraints.Min.Y, hd.Size.Y))
	d := e.Layout(gtx, t.Shaper, f, t.Type.Body, colorOp(gtx, t.Fg), colorOp(gtx, WithAlpha(t.Accent, selectionAlpha)))
	if e.Len() == 0 {
		hintCall.Add(gtx.Ops)
	}
	return d
}
