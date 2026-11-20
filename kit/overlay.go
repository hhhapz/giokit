package kit

import (
	"image"

	"github.com/hhhapz/giokit/gio/gesture"
	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"
	"github.com/hhhapz/giokit/gio/unit"
)

// Floating panel placement, in dp.
const (
	popupMargin unit.Dp = 8 // kept between a panel and the window's edges
	popupGap    unit.Dp = 4 // between a panel and its anchor
	// dialogMargin is kept between a dialog and the window's edges.
	dialogMargin unit.Dp = 24
	// dialogSlide is how far a dialog slides up as it fades in.
	dialogSlide unit.Dp = 12
)

// Drop shadow: shadowLayers rounded rects, each shadowStep (scaled by
// Elevation) wider and lower than the last, fading outward.
const (
	shadowLayers         = 3
	shadowStep   unit.Dp = 2
	// shadowFade sets each layer's share of the scrim's alpha: layer i
	// (1 innermost) gets 1/(shadowFade*(i+1)) of it.
	shadowFade = 3
	// shadowMaxStrength caps how far Elevation darkens the layers.
	shadowMaxStrength = 1.6
)

// backdropBounds is a clip area large enough to cover the window wherever a
// deferred layer's origin lies. It is smaller than noEdges because it is a
// real clip rectangle rather than layout arithmetic.
var backdropBounds = image.Rect(-1<<16, -1<<16, 1<<16, 1<<16)

// floating is the lifecycle Popup and Dialog share: whether the panel is
// open, focus to move into it on the next layout, a user dismissal not yet
// reported, the backdrop's gesture, the panel's focus and its fade. Each
// owner keeps its own input and dismissal policy.
type floating struct {
	open      bool
	focusNext bool // opened since the last layout: focus the panel
	dismissed bool // closed by the user since the owner last reported it
	backdrop  gesture.Click
	focus     Focus // the panel's focus; its address is an event tag
	fade      Tween
}

// show opens the panel and asks for focus to move into it; an open panel is
// left as it is.
func (f *floating) show() {
	if !f.open {
		f.open, f.focusNext = true, true
	}
}

// dismiss closes the panel on the user's behalf.
func (f *floating) dismiss() { f.open, f.dismissed = false, true }

// takeDismissed reports and clears a pending dismissal.
func (f *floating) takeDismissed() bool {
	d := f.dismissed
	f.dismissed = false
	return d
}

// Popup is the state of a floating panel laid out by Theme.Popover (and the
// menus, selects and pickers built on it). The zero value is closed.
//
// A Popup identifies its anchor and panel areas to Gio by the addresses of
// its fields, like Gio's own widget state: do not copy one after first use.
type Popup struct {
	// MatchWidth makes the panel at least as wide as its anchor (selects).
	MatchWidth bool
	// KeepFocus leaves keyboard focus where it is when the panel opens (a
	// combobox keeps typing in its field).
	KeepFocus bool
	// Role is the panel's role (RoleMenu, RoleListbox, RoleDialog); 0 for
	// none.
	Role Role

	floating
	byKey    bool // the pending dismissal was by Escape (return focus with a ring)
	point    image.Point
	atPoint  bool
	entering bool        // just opened: replay the fade-in
	anchor   int         // event tag: only its address matters, naming the anchor area
	origin   image.Point // the anchor's window position in the last frame
	located  bool        // origin is known
	focusTo  *Focus      // focused on open instead of the panel (a menu's group)
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
		p.atPoint = false
	}
	p.show()
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
	k := p.byKey
	p.byKey = false
	return p.takeDismissed(), k
}

// update handles outside presses and focuses the panel on open. It runs
// before anything of the popup is drawn (rule 1), and asks for events only
// while open. Escape is an action (ActCancel) declared by Popover.
//
// A popup closes on the press, as native menus do: the press that closes it
// goes nowhere else. Dialog, by contrast, closes on a full click.
func (p *Popup) update(gtx layout.Context) {
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

func (p *Popup) close(gtx layout.Context, byKey bool) {
	p.dismiss()
	p.byKey = byKey
	Notify(gtx)
}

// Popover lays out anchor, and while p is open a floating panel holding
// content below it. Open it from your handle pass (p.Show, or Toggle on a
// trigger's click); Escape or a press outside closes it. Popover is the
// primitive the other floating components (menus, selects, context menus)
// build on.
//
// The panel is drawn in a deferred layer, so it lands above everything,
// positioned against its anchor's window position in the last frame
// (input.Source.Bounds). It opens below its anchor and flips above it, or
// shifts sideways, to stay inside the window.
//
// While the panel is open, a backdrop covers the window: a press outside
// the panel closes it and goes nowhere else. The backdrop's id is
// "<id>/backdrop"; kittest's audit treats widgets under a backdrop as
// intentionally unreachable, not occluded.
//
// Anchor and panel share one KeyContext nested in the opener's, so Escape
// reaches the innermost open popup first (also with focus on its trigger or,
// for KeepFocus, its field), and anything it does not handle bubbles on.
func (t *Theme) Popover(gtx layout.Context, p *Popup, id string, anchor layout.Widget, content layout.Widget) layout.Dimensions {
	return KeyContext(gtx, "popover", func(gtx layout.Context) layout.Dimensions {
		p.update(gtx)
		if p.open {
			OnAction(gtx, ActCancel, func() { p.close(gtx, true) })
		}
		if r, ok := gtx.Source.Bounds(&p.anchor); ok {
			p.origin, p.located = r.Min, true
		}
		m := op.Record(gtx.Ops)
		dims := anchor(gtx)
		call := m.Stop()
		area := clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops)
		event.Op(gtx.Ops, &p.anchor)
		call.Add(gtx.Ops)
		area.Pop()
		if p.open {
			t.float(gtx, p, id, dims.Size, func(gtx layout.Context) layout.Dimensions {
				return t.FloatingPanel(gtx, content)
			})
		}
		return dims
	})
}

// FloatingPanel is the floating surface of menus, popovers, dialogs and
// toasts: overlay fill, hairline border, a shadow scaled by Elevation.
func (t *Theme) FloatingPanel(gtx layout.Context, content layout.Widget) layout.Dimensions {
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
func (t *Theme) shadow(gtx layout.Context, sz image.Point, r int) {
	if t.Elevation <= 0 {
		return
	}
	c := t.Scrim
	r = min(r, min(sz.X, sz.Y)/2)
	for i := shadowLayers; i >= 1; i-- {
		d := int(float32(gtx.Dp(shadowStep)*i) * t.Elevation)
		rect := image.Rectangle{Max: sz}.Inset(-d).Add(image.Pt(0, d))
		// Layer i gets 1/(shadowFade*(i+1)) of the scrim's alpha.
		a := min(255, int(float32(int(c.A)/(shadowFade+shadowFade*i))*min(t.Elevation, shadowMaxStrength)))
		paint.FillShape(gtx.Ops, WithAlpha(c, uint8(a)), clip.UniformRRect(rect, r+d).Op(gtx.Ops))
	}
}

// float lays out a panel in a deferred layer above the page, placed against
// the anchor (size anchorSz at the current origin), with the backdrop.
func (t *Theme) float(gtx layout.Context, p *Popup, id string, anchorSz image.Point, panel layout.Widget) {
	// The anchor in window space: this frame's size at last frame's
	// position. Not yet located (opened on the anchor's first frame): no
	// edges, so the panel opens below it.
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
	margin, gap := gtx.Dp(popupMargin), gtx.Dp(popupGap)
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
	// Backdrop: catches presses outside the panel.
	bd := clip.Rect(backdropBounds).Push(gtx.Ops)
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
	Tag(gtx, id)
	if p.Role != 0 {
		p.Role.Add(gtx.Ops)
	}
	FocusID(gtx, &p.focus, id)
	pcall.Add(gtx.Ops)
	pa.Pop()
	off.Pop()
	o.Pop()
	op.Defer(gtx.Ops, m.Stop())
}

// Dialog is the state of a modal dialog (th.Modal). Open it with Show from
// your handle pass. The zero value is closed. Like Popup, it identifies its
// areas by address: do not copy one after first use.
type Dialog struct {
	// Persistent dialogs ignore Escape and backdrop clicks: only your
	// buttons close them (unsaved-changes prompts).
	Persistent bool

	floating
}

// Opened reports whether the dialog is open.
func (d *Dialog) Opened() bool { return d.open }

// Show opens the dialog; on its next layout it takes focus and fades in.
// Showing an open dialog does nothing.
func (d *Dialog) Show() { d.show() }

// Hide closes the dialog. It is not a dismissal: DidDismiss stays false.
func (d *Dialog) Hide() { d.open = false }

// DidDismiss reports whether the user closed the dialog with Escape or a
// backdrop click since the last call (your buttons call Hide themselves).
func (d *Dialog) DidDismiss() bool { return d.takeDismissed() }

// Modal lays out page, and while d is open a dialog above it. The page is
// laid out disabled meanwhile, without dimming it twice (the scrim does):
// nothing behind the dialog takes input or focus, so Tab cycles inside the
// dialog for free. Call it near the root, around everything the dialog
// should cover. Escape or a click on the backdrop closes the dialog unless
// it is Persistent. Unlike a Popup, a dialog closes on a full click (press
// and release) on the backdrop, so a drag that starts in the dialog and ends
// outside it does not close it.
func (t *Theme) Modal(gtx layout.Context, d *Dialog, id string, page layout.Widget, dialog layout.Widget) layout.Dimensions {
	if d.open {
		for {
			e, ok := d.backdrop.Update(gtx.Source)
			if !ok {
				break
			}
			if e.Kind == gesture.KindClick && !d.Persistent {
				d.dismiss()
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
	dims := disabledArea(gtx.Disabled(), page) // inert to screen readers too

	m := op.Record(gtx.Ops)
	a := d.fade.To(gtx, 1, t.Dur(DurNormal))
	o := paint.PushOpacity(gtx.Ops, a)
	full := image.Rectangle{Max: gtx.Constraints.Max}
	bd := clip.Rect(full).Push(gtx.Ops)
	paint.Fill(gtx.Ops, t.Scrim)
	d.backdrop.Add(gtx.Ops)
	Tag(gtx, Part(id, "backdrop"))
	bd.Pop()
	// The card: centered, at most DialogMaxWidth wide, sliding up a little
	// as it fades in. A window smaller than the margins gives it no room,
	// never negative constraints.
	margin := gtx.Dp(dialogMargin)
	cgtx := gtx
	cgtx.Constraints = layout.Constraints{Max: image.Pt(
		max(0, min(gtx.Constraints.Max.X-2*margin, gtx.Dp(t.DialogMaxWidth))),
		max(0, gtx.Constraints.Max.Y-2*margin))}
	cgtx.Constraints.Min.X = cgtx.Constraints.Max.X
	cm := op.Record(gtx.Ops)
	// The dialog is a modal key context: Escape closes it unless Persistent,
	// and with focus lost, keys still resolve here rather than behind it.
	cd := KeyContext(cgtx, "dialog", func(gtx layout.Context) layout.Dimensions {
		markModal(gtx)
		trackFocus(gtx, &d.focus)
		if !d.Persistent {
			OnAction(gtx, ActCancel, d.dismiss)
		}
		return t.FloatingPanel(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(t.Space(Space5)).Layout(gtx, dialog)
		})
	})
	call := cm.Stop()
	pos := full.Max.Sub(cd.Size).Div(2)
	pos.Y -= int(float32(gtx.Dp(dialogSlide)) * (1 - a))
	off := op.Offset(pos).Push(gtx.Ops)
	pa := clip.Rect(image.Rectangle{Max: cd.Size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &d.focus)
	Tag(gtx, id)
	FocusID(gtx, &d.focus, id)
	RoleDialog.Add(gtx.Ops)
	call.Add(gtx.Ops)
	pa.Pop()
	off.Pop()
	o.Pop()
	op.Defer(gtx.Ops, m.Stop())
	return dims
}
