package kit

import (
	"image"
	"strconv"
	"time"

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
// positioned against its anchor's window-space bounds from the last frame
// (the patched Source.Bounds; anchors are a frame behind only while they
// move). It opens below its anchor and flips above it, or shifts sideways,
// to stay inside the window (kit.Root marks the window).
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
	anchor    int    // tag: its address identifies the anchor area
	focus     Focus  // the panel's focus
	focusTo   *Focus // focused on open instead of the panel (a menu's group)
	fade      Tween
}

// Opened reports whether the panel is open.
func (p *Popup) Opened() bool { return p.open }

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
		m := op.Record(gtx.Ops)
		dims := anchor(gtx)
		call := m.Stop()
		area := clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops)
		event.Op(gtx.Ops, &p.anchor)
		call.Add(gtx.Ops)
		area.Pop()
		if p.open {
			t.float(gtx, p, id, dims.Size, func(gtx C) D {
				return t.panel(gtx, content)
			})
		}
		return dims
	})
}

// panel is the floating surface: overlay fill, hairline border, soft shadow.
func (t *Theme) panel(gtx C, content W) D {
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
	win := windowBounds(gtx)
	ab, ok := gtx.Source.Bounds(&p.anchor)
	if !ok { // first frame: assume the anchor sits where it is drawn
		ab = image.Rectangle{Max: anchorSz}
	}
	ab.Max = ab.Min.Add(anchorSz) // this frame's size, last frame's position
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
		max(0, min(win.Dx()-2*margin, gtx.Dp(420))),
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
		min(gtx.Constraints.Max.X-2*gtx.Dp(24), gtx.Dp(480)),
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
		return t.panel(gtx, func(gtx C) D {
			return layout.UniformInset(t.Space(20)).Layout(gtx, dialog)
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
	call.Add(gtx.Ops)
	pt.Pop()
	pa.Pop()
	off.Pop()
	o.Pop()
	op.Defer(gtx.Ops, m.Stop())
	return dims
}

// DialogContent is the standard dialog body: a title, a body, and buttons
// right-aligned at the bottom (pass the primary action last).
func (t *Theme) DialogContent(gtx C, title string, body W, actions ...W) D {
	var acts []layout.FlexChild
	acts = append(acts, Flexed(1, Grow))
	for i, a := range actions {
		if i > 0 {
			acts = append(acts, Rigid(Gap(t.Space(8), 0)))
		}
		acts = append(acts, Rigid(a))
	}
	return Col(gtx,
		Rigid(func(gtx C) D { return t.Text(gtx, title, t.Type.Text*1.15, t.Fg, SemiBold) }),
		Rigid(Gap(0, t.Space(10))),
		Rigid(body),
		Rigid(Gap(0, t.Space(20))),
		Rigid(func(gtx C) D { return Row(gtx, acts...) }),
	)
}

// ToastDuration is how long a toast stays before it dismisses itself.
const ToastDuration = 5 * time.Second

// Toasts is a stack of transient notifications (th.ToastHost). Push from
// your handle pass or a Spawn callback; the UI goroutine owns it.
type Toasts struct {
	items []*toast
	seq   int
	close Pool[Clickable]
}

type toast struct {
	id          string
	status      Status
	title, body string
	at          time.Time // first shown (stamped at layout, on the frame clock)
	fade        Tween
}

// Push adds a toast; it shows on the next frame. Its close button's id is
// "toast:<n>/close", n counting from 1.
func (ts *Toasts) Push(s Status, title, body string) {
	ts.seq++
	ts.items = append(ts.items, &toast{id: "toast:" + strconv.Itoa(ts.seq), status: s, title: title, body: body})
}

// Len is the number of toasts showing.
func (ts *Toasts) Len() int { return len(ts.items) }

// ToastHost lays out w with the toasts stacked above it, bottom right, the
// newest at the bottom. Each one closes itself after ToastDuration, or with
// its close button. Wrap your whole window with it.
func (t *Theme) ToastHost(gtx C, ts *Toasts, w W) D {
	// Expire and close first (rule 1).
	kept := ts.items[:0]
	for _, it := range ts.items {
		if it.at.IsZero() {
			it.at = gtx.Now
			it.fade.To(gtx, 0, 0) // start transparent; fades in below
		}
		if ts.close.Get(it.id).Clicked(gtx) || gtx.Now.Sub(it.at) >= ToastDuration {
			ts.close.Delete(it.id)
			continue
		}
		gtx.Execute(op.InvalidateCmd{At: it.at.Add(ToastDuration)})
		kept = append(kept, it)
	}
	if len(kept) != len(ts.items) {
		Notify(gtx)
	}
	clear(ts.items[len(kept):])
	ts.items = kept
	dims := w(gtx)
	if len(ts.items) == 0 {
		return dims
	}
	m := op.Record(gtx.Ops)
	margin := gtx.Dp(16)
	width := min(gtx.Dp(340), gtx.Constraints.Max.X-2*margin)
	y := gtx.Constraints.Max.Y - margin
	for i := len(ts.items) - 1; i >= 0; i-- {
		it := ts.items[i]
		a := it.fade.To(gtx, 1, t.Dur(DurNormal))
		cgtx := gtx
		cgtx.Constraints = layout.Exact(image.Pt(width, 0))
		cgtx.Constraints.Max.Y = gtx.Constraints.Max.Y
		rec := op.Record(gtx.Ops)
		cd := t.toastCard(cgtx, ts, it)
		call := rec.Stop()
		y -= cd.Size.Y
		o := paint.PushOpacity(gtx.Ops, a)
		off := op.Offset(image.Pt(gtx.Constraints.Max.X-margin-width+int(float32(gtx.Dp(24))*(1-a)), y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		off.Pop()
		o.Pop()
		y -= gtx.Dp(8)
	}
	op.Defer(gtx.Ops, m.Stop())
	return dims
}

func (t *Theme) toastCard(gtx C, ts *Toasts, it *toast) D {
	fg, _, icon := t.statusColors(it.status)
	return t.panel(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		m := op.Record(gtx.Ops)
		dims := layout.Inset{Top: t.Space(10), Bottom: t.Space(10), Left: t.Space(12), Right: t.Space(8)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Start}.Layout(gtx,
				Rigid(func(gtx C) D {
					return layout.Inset{Top: 1, Right: 10}.Layout(gtx, func(gtx C) D { return icon(gtx, 16, fg) })
				}),
				Flexed(1, func(gtx C) D {
					return Col(gtx,
						Rigid(func(gtx C) D { return t.Text(gtx, it.title, t.Type.Body, t.Fg, SemiBold) }),
						Rigid(func(gtx C) D {
							if it.body == "" {
								return D{}
							}
							return layout.Inset{Top: 2}.Layout(gtx, func(gtx C) D { return t.Paragraph(gtx, it.body, t.Muted) })
						}),
					)
				}),
				Rigid(func(gtx C) D { return t.IconButton(gtx, ts.close.Get(it.id), Part(it.id, "close"), IconX, true) }),
			)
		})
		call := m.Stop()
		defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
		Tag(gtx, it.id)
		call.Add(gtx.Ops)
		return dims
	})
}
