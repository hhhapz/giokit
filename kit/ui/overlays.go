package ui

import (
	"image"
	"image/color"
	"strconv"
	"time"

	"github.com/hhhapz/giokit/gio/gesture"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"

	"github.com/hhhapz/giokit/kit"
)

// DialogContent is the standard dialog body for kit's Modal: a title, a
// body, and buttons right-aligned at the bottom (pass the primary action
// last).
type DialogContent struct {
	Title   string          // the dialog's heading
	Body    layout.Widget   // the content under the heading; nil for none
	Actions []layout.Widget // buttons, laid out left to right at the bottom right
}

// dialogTitleScale sets a dialog's heading size relative to Type.Text: a
// step above the body, well below a page Title.
const dialogTitleScale = 1.15

// Layout draws the heading, body and action row; th.Modal provides the
// panel, focus trap and dismissal around it. It takes no input itself.
func (dc DialogContent) Layout(gtx layout.Context, th *kit.Theme) layout.Dimensions {
	acts := []layout.FlexChild{kit.Flexed(1, kit.Grow)}
	for _, a := range dc.Actions {
		acts = append(acts, kit.Rigid(a))
	}
	return kit.Col(gtx,
		kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			return th.Text(gtx, dc.Title, th.Type.Text*dialogTitleScale, th.Fg, kit.SemiBold)
		}),
		kit.Rigid(kit.Gap(0, th.Space(kit.Space2p5))),
		kit.Rigid(dc.Body),
		kit.Rigid(kit.Gap(0, th.Space(kit.Space5))),
		kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Flex{Align: layout.Middle, Gap: kit.Space2}.Layout(gtx, th, acts...)
		}),
	)
}

// Tooltip shows Text below Content after the pointer rests on it for
// TooltipDelay, and at once while focus is inside Content (a focused
// button shows its tooltip). Escape hides it until the pointer leaves and
// focus moves on. Clicks still reach the content.
//
// While showing, the tooltip is a node of its own: id ID, role tooltip,
// labelled Text. It supplements Content, which still needs its own label.
type Tooltip struct {
	ID      string        // the tooltip node's id while it shows
	Text    string        // the tip, in the user's language
	Content layout.Widget // the control the tip describes
}

// TooltipState is a Tooltip's hover timing and fade. The zero value is
// hidden; it holds an event tag, so do not copy it after first use.
type TooltipState struct {
	hover     gesture.Hover
	since     time.Time // when the pointer came to rest on the content
	hovering  bool      // the delay has passed
	dismissed bool      // Escape hid it until hover and focus end
	shown     bool      // drawn last frame
	fade      kit.Tween
}

// TooltipDelay is how long the pointer must rest before a tooltip shows.
const TooltipDelay = 500 * time.Millisecond

// Layout draws Content and, while the tip shows, a fading bubble below it
// in a kit.Layer. It handles hover timing and Escape itself, scheduling the
// frame that shows the tip; there is no Update.
func (tt Tooltip) Layout(gtx layout.Context, th *kit.Theme, s *TooltipState) layout.Dimensions {
	hovered := s.hover.Update(gtx.Source)
	switch {
	case !hovered:
		s.since, s.hovering = time.Time{}, false
	case s.since.IsZero():
		s.since = gtx.Now
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(TooltipDelay)})
	case !s.hovering && gtx.Now.Sub(s.since) >= TooltipDelay:
		s.hovering = true
	case !s.hovering:
		gtx.Execute(op.InvalidateCmd{At: s.since.Add(TooltipDelay)})
	}
	rec := op.Record(gtx.Ops)
	dims, focused := kit.FocusWithin(gtx, func(gtx layout.Context) layout.Dimensions {
		if s.shown {
			kit.OnAction(gtx, kit.ActCancel, func() { s.dismissed = true })
		}
		return tt.Content(gtx)
	})
	child := rec.Stop()
	area := clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops)
	s.hover.Add(gtx.Ops) // an ancestor of the content's areas, so clicks still reach it
	child.Add(gtx.Ops)
	area.Pop()
	if !hovered && !focused {
		s.dismissed = false
	}
	show := (s.hovering || focused) && !s.dismissed
	if show && !s.shown {
		s.fade.From(gtx, 0, th.Dur(kit.DurFast))
	}
	s.shown = show
	if !show {
		return dims
	}
	a := s.fade.To(gtx, 1, th.Dur(kit.DurFast))
	kit.Layer(gtx, image.Pt(0, dims.Size.Y+gtx.Dp(th.Space(kit.Space1p5))), func(gtx layout.Context) layout.Dimensions {
		defer paint.PushOpacity(gtx.Ops, a).Pop()
		gtx.Constraints.Min = image.Point{}
		return kit.Group(gtx, tt.ID, tt.Text, kit.RoleTooltip, func(gtx layout.Context) layout.Dimensions {
			return th.Styled(kit.Surface{Bg: th.Overlay, Border: th.Border, Radius: th.Radius}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return kit.Flex{PadX: kit.Space2, PadY: kit.Space1}.Layout(gtx, th, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return th.Text(gtx, tt.Text, th.Type.Small, th.Fg, kit.Regular)
				}))
			})
		})
	})
	return dims
}

// ToastDuration is how long a toast stays, by default, before it
// dismisses itself.
const ToastDuration = 5 * time.Second

// Toasts is a stack of transient notifications shown by a ToastHost. Push
// from the handle pass or a Spawn callback; the UI goroutine owns it. The
// zero value is empty; do not copy it after first use.
type Toasts struct {
	items []*toast
	seq   int
	close kit.Pool[kit.Clickable]
}

// toast is one shown notification of a Toasts host.
type toast struct {
	seq         int // its number, from 1: its id is <host ID>/<seq>
	status      kit.Status
	title, body string
	at          time.Time // first shown (stamped at layout, on the frame clock)
	fade        kit.Tween
}

// key is the toast's key in the close button pool.
func (it *toast) key() string { return strconv.Itoa(it.seq) }

// Push adds a toast; it shows on the next frame. Toasts are numbered from
// 1 in push order: the host shows toast n as <host ID>/n with its dismiss
// button <host ID>/n/dismiss.
func (ts *Toasts) Push(s kit.Status, title, body string) {
	ts.seq++
	ts.items = append(ts.items, &toast{seq: ts.seq, status: s, title: title, body: body})
}

// Len is the number of toasts showing.
func (ts *Toasts) Len() int { return len(ts.items) }

// ToastHost lays out Page with the toasts stacked above it, bottom right,
// the newest at the bottom. Each closes itself after Duration, or with its
// dismiss button. Wrap the whole window with it.
//
// Toasts are ToastWidth wide, narrowed to fit a smaller window inside the
// margins; a window with no room inside the margins shows none, though
// they still expire.
type ToastHost struct {
	ID       string        // the toasts' ids are ID/<n>; "" is "toasts"
	Page     layout.Widget // the window's content, drawn under the toasts
	Duration time.Duration // how long each toast stays; 0 is ToastDuration
}

// Layout drops expired and dismissed toasts from ts, draws Page, then the
// remaining toasts in a kit.Layer. It schedules a frame for the next
// expiry, so toasts leave on time without input; there is no Update.
func (host ToastHost) Layout(gtx layout.Context, th *kit.Theme, ts *Toasts) layout.Dimensions {
	id := host.ID
	if id == "" {
		id = "toasts"
	}
	dur := host.Duration
	if dur == 0 {
		dur = ToastDuration
	}
	// Expire and close first, before drawing.
	kept := ts.items[:0]
	for _, it := range ts.items {
		if it.at.IsZero() {
			it.at = gtx.Now
			it.fade.To(gtx, 0, 0) // jump to transparent, so it fades in below
		}
		if ts.close.Get(it.key()).Clicked(gtx) || gtx.Now.Sub(it.at) >= dur {
			ts.close.Delete(it.key())
			continue
		}
		gtx.Execute(op.InvalidateCmd{At: it.at.Add(dur)})
		kept = append(kept, it)
	}
	if len(kept) != len(ts.items) {
		kit.Notify(gtx)
	}
	clear(ts.items[len(kept):])
	ts.items = kept
	dims := host.Page(gtx)
	if len(ts.items) == 0 {
		return dims
	}
	margin := gtx.Dp(th.Space(kit.Space4))
	width := min(gtx.Dp(th.ToastWidth), gtx.Constraints.Max.X-2*margin)
	if width <= 0 {
		return dims
	}
	slide := gtx.Dp(th.Space(kit.Space6)) // how far a new toast slides in from
	y := gtx.Constraints.Max.Y - margin
	for i := len(ts.items) - 1; i >= 0; i-- {
		it := ts.items[i]
		a := it.fade.To(gtx, 1, th.Dur(kit.DurNormal))
		cgtx := gtx
		cgtx.Constraints = layout.Exact(image.Pt(width, 0))
		cgtx.Constraints.Max.Y = gtx.Constraints.Max.Y
		rec := op.Record(gtx.Ops)
		cd := toastCard(cgtx, th, ts, kit.Part(id, it.key()), it)
		call := rec.Stop()
		y -= cd.Size.Y
		at := image.Pt(gtx.Constraints.Max.X-margin-width+int(float32(slide)*(1-a)), y)
		kit.Layer(gtx, at, func(gtx layout.Context) layout.Dimensions {
			defer paint.PushOpacity(gtx.Ops, a).Pop()
			call.Add(gtx.Ops)
			return cd
		})
		y -= gtx.Dp(th.Space(kit.Space2))
	}
	return dims
}

// toastCard is one toast, named id.
func toastCard(gtx layout.Context, th *kit.Theme, ts *Toasts, id string, it *toast) layout.Dimensions {
	fg, _, icon := th.StatusColors(it.status)
	return th.FloatingPanel(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return kit.Group(gtx, id, it.title, kit.RoleStatus, func(gtx layout.Context) layout.Dimensions { return toastBody(gtx, th, ts, id, it, fg, icon) })
	})
}

// toastBody draws toast it of host ts (whose id is id): the status icon in
// fg, the title and optional body, and the dismiss button.
func toastBody(gtx layout.Context, th *kit.Theme, ts *Toasts, id string, it *toast, fg color.NRGBA, icon kit.Icon) layout.Dimensions {
	return layout.Inset{Top: th.Space(kit.Space2p5), Bottom: th.Space(kit.Space2p5), Left: th.Space(kit.Space3), Right: th.Space(kit.Space2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Start}.Layout(gtx,
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: statusIconNudge, Right: th.Space(kit.Space2p5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return icon(gtx, th.IconSizeLg, fg) })
			}),
			kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return kit.Col(gtx,
					kit.Rigid(func(gtx layout.Context) layout.Dimensions {
						return th.Text(gtx, it.title, th.Type.Body, th.Fg, kit.SemiBold)
					}),
					kit.Rigid(func(gtx layout.Context) layout.Dimensions {
						if it.body == "" {
							return layout.Dimensions{}
						}
						return layout.Inset{Top: th.Space(kit.Space0p5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return th.Paragraph(gtx, it.body, th.Muted) })
					}),
				)
			}),
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return IconButton{ID: kit.Part(id, "dismiss"), Icon: kit.IconX, Label: th.Strings.Dismiss, Active: true}.Layout(gtx, th, ts.close.Get(it.key()))
			}),
		)
	})
}
