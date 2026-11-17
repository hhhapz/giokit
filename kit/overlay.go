package kit

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// Overlays: anything that floats above the page. Popup is the shared state;
// Popover the primitive the rest (Menu, Select, ContextArea) build on; Modal
// the dialog host; ToastHost the notification stack.
//
// A floating panel is drawn with op.Defer, so it lands above everything,
// positioned against its anchor's window position. kit learns that from
// pointer events: the window area (kit.Window) and the anchor see the same
// hover event in their own coordinates, and the difference is the anchor's
// offset (stock Gio has no way to ask). It opens below its anchor and flips
// above it, or shifts sideways, to stay inside the window; an anchor the
// pointer has never crossed (opened from the keyboard first) opens below.
//
// While a panel is open, a backdrop covers the window: a press outside the
// panel closes it and goes nowhere else (as native menus do). The backdrop's
// id is "<id>/backdrop"; kittest's audit treats widgets under a backdrop as
// intentionally unreachable, not occluded.

// Popup is the state of a floating panel. The zero value is closed.
type Popup struct {
	// MatchWidth makes the panel at least as wide as its anchor (selects).
	MatchWidth bool
	// KeepFocus leaves keyboard focus where it is when the panel opens (a
	// combobox keeps typing in its field).
	KeepFocus bool

	open      bool
	focusNext bool // opened since the last layout: focus the panel
	dismissed bool // closed by Escape or a press outside since DidDismiss
	byKey     bool // ...by Escape (return focus with a ring)
	point     image.Point
	atPoint   bool
	entering  bool // just opened: replay the fade-in
	backdrop  gesture.Click
	anchor    int         // tag: its address identifies the anchor area
	origin    image.Point // the anchor's window position, from pointer events
	located   bool        // origin is known
	focus     Focus       // the panel's focus
	focusTo   *Focus      // focused on open instead of the panel (a menu's group)
	fade      Tween
}

// Opened reports whether the panel is open.
func (p *Popup) Opened() bool { return p.open }

// Opening reports that the popup opened since its last layout, which moves
// focus into it: a component picks what to focus (a calendar's day), or
// keeps a just-opened list from closing.
func (p *Popup) Opening() bool { return p.focusNext }

// FocusOnOpen makes the popup focus f when it opens, instead of its panel
// (a menu focuses its item group). Set it before laying out the Popover.
func (p *Popup) FocusOnOpen(f *Focus) { p.focusTo = f }

// PanelFocus is the panel's own focus, for reading keys (Tab) while the
// panel holds it.
func (p *Popup) PanelFocus() *Focus { return &p.focus }

// Show opens the panel below its anchor.
func (p *Popup) Show() {
	if !p.open {
		p.open, p.focusNext, p.atPoint = true, true, false
	}
}

// ShowAt opens the panel at pt, in the anchor's coordinates (context menus).
func (p *Popup) ShowAt(pt image.Point) {
	p.open, p.focusNext, p.atPoint, p.point = true, true, true, pt
}

// Hide closes the panel.
func (p *Popup) Hide() { p.open = false }

// Toggle opens a closed panel and closes an open one.
func (p *Popup) Toggle() {
	if p.open {
		p.Hide()
	} else {
		p.Show()
	}
}

// DidDismiss reports whether the user closed the panel (Escape, or a press
// outside it) since the last call, and whether it was by keyboard: return
// focus to the trigger then, with a ring if by keyboard.
func (p *Popup) DidDismiss() (dismissed, byKey bool) {
	d, k := p.dismissed, p.byKey
	p.dismissed, p.byKey = false, false
	return d, k
}

// update handles outside presses and focuses the panel on open. It runs
// before anything of the popup is drawn (rule 1), and asks for events only
// while open. Escape is an action (ActCancel) declared by Popover.
func (p *Popup) update(gtx C) {
	if !p.open {
		return
	}
	for {
		e, ok := p.backdrop.Update(gtx.Source)
		if !ok {
			break
		}
		if e.Kind == gesture.KindPress {
			p.close(gtx, false)
		}
	}
	if p.open && p.focusNext {
		p.focusNext = false
		if !p.KeepFocus {
			target := &p.focus
			if p.focusTo != nil {
				target = p.focusTo
			}
			target.ask(gtx) // focusable before the command runs
			gtx.Execute(key.FocusCmd{Tag: target})
		}
		p.entering = true
	}
}

func (p *Popup) close(gtx C, byKey bool) {
	p.open, p.dismissed, p.byKey = false, true, byKey
	Notify(gtx)
}

// Popover lays out anchor, and while p is open a floating panel holding
// content below it. Open it from your handle pass (p.Show, or Toggle on a
// trigger's click); Escape or a press outside closes it.
//
// Anchor and panel share one KeyContext nested in the opener's, so Escape
// reaches the innermost open popup first (also with focus on its trigger or,
// for KeepFocus, its field), and anything it does not handle bubbles on.
func (t *Theme) Popover(gtx C, p *Popup, id string, anchor W, content W) D {
	return KeyContext(gtx, "popover", func(gtx C) D {
		p.update(gtx)
		if p.open {
			OnAction(gtx, ActCancel, func() { p.close(gtx, true) })
		}
		if off, ok := anchorOffset(gtx, &p.anchor); ok {
			p.origin, p.located = off, true
		}
		m := op.Record(gtx.Ops)
		dims := anchor(gtx)
		call := m.Stop()
		area := clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops)
		event.Op(gtx.Ops, &p.anchor)
		call.Add(gtx.Ops)
		area.Pop()
		if p.open {
			t.float(gtx, p, id, dims.Size, func(gtx C) D {
				return t.FloatingPanel(gtx, content)
			})
		}
		return dims
	})
}

// FloatingPanel is the floating surface of menus, popovers, dialogs and
// toasts: overlay fill, hairline border, a shadow scaled by Elevation.
func (t *Theme) FloatingPanel(gtx C, content W) D {
	m := op.Record(gtx.Ops)
	dims := t.Styled(Surface{Bg: t.Overlay, Border: t.Border, Radius: t.RadiusLg()}).Layout(gtx, content)
	call := m.Stop()
	t.shadow(gtx, dims.Size, gtx.Dp(t.RadiusLg()))
	call.Add(gtx.Ops)
	return dims
}

// shadow draws a drop shadow under a rounded rect of size sz: a few
// widening, fading layers (Gio has no blur). Theme.Elevation scales its
// reach and strength; 0 draws none.
func (t *Theme) shadow(gtx C, sz image.Point, r int) {
	if t.Elevation <= 0 {
		return
	}
	c := t.Scrim
	r = min(r, min(sz.X, sz.Y)/2)
	for i := 3; i >= 1; i-- {
		d := int(float32(gtx.Dp(2)*i) * t.Elevation)
		rect := image.Rectangle{Max: sz}.Inset(-d).Add(image.Pt(0, d))
		a := min(255, int(float32(int(c.A)/(3+3*i))*min(t.Elevation, 1.6)))
		paint.FillShape(gtx.Ops, WithAlpha(c, uint8(a)), clip.UniformRRect(rect, r+d).Op(gtx.Ops))
	}
}

// float lays out a panel in a deferred layer above the page, placed against
// the anchor (size anchorSz at the current origin), with the backdrop.
func (t *Theme) float(gtx C, p *Popup, id string, anchorSz image.Point, panel W) {
	// The anchor in window space: this frame's size at the position its
	// pointer events last showed. Not yet located (opened by keyboard before
	// the pointer ever crossed it): no edges, so the panel opens below it.
	win := windowBounds(gtx)
	ab := image.Rectangle{Max: anchorSz}
	if p.located {
		ab = ab.Add(p.origin)
	} else {
		win = noEdges
	}
	if p.atPoint {
		ab = image.Rectangle{Min: ab.Min.Add(p.point), Max: ab.Min.Add(p.point)}
	}
	margin, gap := gtx.Dp(8), gtx.Dp(4)
	if p.atPoint {
		gap = 0
	}

	// Measure the panel.
	pgtx := gtx
	pgtx.Constraints = layout.Constraints{Max: image.Pt(
		max(0, min(win.Dx()-2*margin, gtx.Dp(t.PopoverMaxWidth))),
		max(0, max(ab.Min.Y-win.Min.Y, win.Max.Y-ab.Max.Y)-gap-margin),
	)}
	if p.MatchWidth {
		pgtx.Constraints.Min.X = min(ab.Dx(), pgtx.Constraints.Max.X)
	}
	m := op.Record(gtx.Ops)
	if p.entering {
		p.entering = false
		p.fade.From(gtx, 0, t.Dur(DurFast))
	}
	o := paint.PushOpacity(gtx.Ops, p.fade.To(gtx, 1, t.Dur(DurFast)))
	p.focus.track(gtx)
	pm := op.Record(gtx.Ops)
	pd := panel(pgtx)
	pcall := pm.Stop()

	// Place it: below, else above if there's more room there; then shift
	// inside the window.
	sz := pd.Size
	pos := image.Pt(ab.Min.X, ab.Max.Y+gap)
	if pos.Y+sz.Y > win.Max.Y-margin && ab.Min.Y-win.Min.Y > win.Max.Y-ab.Max.Y {
		pos.Y = ab.Min.Y - gap - sz.Y
	}
	pos.X = max(win.Min.X+margin, min(pos.X, win.Max.X-margin-sz.X))
	pos.Y = max(win.Min.Y+margin, min(pos.Y, win.Max.Y-margin-sz.Y))
	// Backdrop: catches presses outside the panel. Huge, so it covers the
	// window wherever this layer's origin is.
	huge := image.Rect(-1<<16, -1<<16, 1<<16, 1<<16)
	bd := clip.Rect(huge).Push(gtx.Ops)
	p.backdrop.Add(gtx.Ops)
	Tag(gtx, Part(id, "backdrop"))
	bd.Pop()

	// The panel, in window space translated to this layer's origin (the
	// anchor's origin this frame, i.e. ab.Min less any point offset).
	anchorOrigin := ab.Min
	if p.atPoint {
		anchorOrigin = ab.Min.Sub(p.point)
	}
	off := op.Offset(pos.Sub(anchorOrigin)).Push(gtx.Ops)
	pa := clip.Rect(image.Rectangle{Max: sz}).Push(gtx.Ops)
	event.Op(gtx.Ops, &p.focus)
	// The id goes on a nested, handler-free area: Gio drops semantics from
	// an area whose handler asks for no pointer gestures.
	pt := clip.Rect(image.Rectangle{Max: sz}).Push(gtx.Ops)
	Tag(gtx, id)
	FocusID(gtx, &p.focus, id)
	pcall.Add(gtx.Ops)
	pt.Pop()
	pa.Pop()
	off.Pop()
	o.Pop()
	op.Defer(gtx.Ops, m.Stop())
}

// Dialog is the state of a modal dialog (th.Modal). Open it with Show from
// your handle pass.
type Dialog struct {
	// Persistent dialogs ignore Escape and backdrop clicks: only your
	// buttons close them (unsaved-changes prompts).
	Persistent bool

	open      bool
	focusNext bool
	dismissed bool
	backdrop  gesture.Click
	focus     Focus
	fade      Tween
}

func (d *Dialog) Opened() bool { return d.open }
func (d *Dialog) Show() {
	if !d.open {
		d.open, d.focusNext = true, true
	}
}
func (d *Dialog) Hide() { d.open = false }

// DidDismiss reports whether the user closed the dialog with Escape or a
// backdrop click since the last call (your buttons call Hide themselves).
func (d *Dialog) DidDismiss() bool {
	v := d.dismissed
	d.dismissed = false
	return v
}

// Modal lays out page, and while d is open a dialog above it. The page is
// laid out disabled meanwhile, without dimming it twice (the scrim does):
// nothing behind the dialog takes input or focus, so Tab cycles inside the
// dialog for free. Call it near the root, around everything the dialog
// should cover. Escape or a click on the backdrop closes the dialog unless
// it is Persistent.
func (t *Theme) Modal(gtx C, d *Dialog, id string, page W, dialog W) D {
	if d.open {
		for {
			e, ok := d.backdrop.Update(gtx.Source)
			if !ok {
				break
			}
			if e.Kind == gesture.KindClick && !d.Persistent {
				d.open, d.dismissed = false, true
				Notify(gtx)
			}
		}
		d.focus.ask(gtx)
	}
	if !d.open {
		return page(gtx)
	}
	if d.focusNext {
		d.focusNext = false
		gtx.Execute(key.FocusCmd{Tag: &d.focus})
		d.fade.From(gtx, 0, t.Dur(DurNormal))
	}
	pgtx := gtx
	pgtx.Source = gtx.Source.Disabled()
	dims := disabledArea(pgtx, "", page) // inert to screen readers too

	m := op.Record(gtx.Ops)
	a := d.fade.To(gtx, 1, t.Dur(DurNormal))
	o := paint.PushOpacity(gtx.Ops, a)
	full := image.Rectangle{Max: gtx.Constraints.Max}
	bd := clip.Rect(full).Push(gtx.Ops)
	paint.ColorOp{Color: t.Scrim}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	d.backdrop.Add(gtx.Ops)
	Tag(gtx, Part(id, "backdrop"))
	bd.Pop()
	// The card: centered, at most 480dp wide, sliding up a little as it fades in.
	cgtx := gtx
	cgtx.Constraints = layout.Constraints{Max: image.Pt(
		min(gtx.Constraints.Max.X-2*gtx.Dp(24), gtx.Dp(t.DialogMaxWidth)),
		gtx.Constraints.Max.Y-2*gtx.Dp(24))}
	cgtx.Constraints.Min.X = cgtx.Constraints.Max.X
	cm := op.Record(gtx.Ops)
	// The dialog is a modal key context: Escape closes it unless Persistent,
	// and with focus lost, keys still resolve here rather than behind it.
	cd := KeyContext(cgtx, "dialog", func(gtx C) D {
		markModal(gtx)
		trackFocus(gtx, &d.focus)
		if !d.Persistent {
			OnAction(gtx, ActCancel, func() { d.open, d.dismissed = false, true })
		}
		return t.FloatingPanel(gtx, func(gtx C) D {
			return layout.UniformInset(t.Space(Space5)).Layout(gtx, dialog)
		})
	})
	call := cm.Stop()
	pos := full.Max.Sub(cd.Size).Div(2)
	pos.Y -= int(float32(gtx.Dp(12)) * (1 - a))
	off := op.Offset(pos).Push(gtx.Ops)
	pa := clip.Rect(image.Rectangle{Max: cd.Size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &d.focus)
	pt := clip.Rect(image.Rectangle{Max: cd.Size}).Push(gtx.Ops) // id on a handler-free area
	Tag(gtx, id)
	FocusID(gtx, &d.focus, id)
	SetRole(gtx, id, RoleDialog)
	call.Add(gtx.Ops)
	pt.Pop()
	pa.Pop()
	off.Pop()
	o.Pop()
	op.Defer(gtx.Ops, m.Stop())
	return dims
}
