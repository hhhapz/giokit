package ui

import (
	"image"
	"image/color"
	"strconv"
	"time"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/hhhapz/giokit/kit"
)

// DialogContent is the standard dialog body for kit's Modal: a title, a
// body, and buttons right-aligned at the bottom (pass the primary action
// last).
type DialogContent struct {
	Title   string
	Body    kit.W
	Actions []kit.W
}

func (dc DialogContent) Layout(gtx C, th *kit.Theme) D {
	acts := []layout.FlexChild{kit.Flexed(1, kit.Grow)}
	for _, a := range dc.Actions {
		acts = append(acts, kit.Rigid(a))
	}
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D { return th.Text(gtx, dc.Title, th.Type.Text*1.15, th.Fg, kit.SemiBold) }),
		kit.Rigid(kit.Gap(0, th.Space(kit.Space2p5))),
		kit.Rigid(dc.Body),
		kit.Rigid(kit.Gap(0, th.Space(kit.Space5))),
		kit.Rigid(func(gtx C) D { return kit.Flex{Align: layout.Middle, Gap: kit.Space2}.Layout(gtx, th, acts...) }),
	)
}

// Tooltip shows Text below Content after the pointer rests on it for
// TooltipDelay. Clicks still reach the content.
type Tooltip struct {
	Text    string
	Content kit.W
}

// TooltipState is a Tooltip's hover timing.
type TooltipState struct {
	hover   gesture.Hover
	since   time.Time
	showing bool
	fade    kit.Tween
}

// TooltipDelay is how long the pointer must rest before a tooltip shows.
const TooltipDelay = 500 * time.Millisecond

func (tt Tooltip) Layout(gtx C, th *kit.Theme, s *TooltipState) D {
	hovered := s.hover.Update(gtx.Source)
	switch {
	case !hovered:
		s.since, s.showing = time.Time{}, false
	case s.since.IsZero():
		s.since = gtx.Now
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(TooltipDelay)})
	case !s.showing && gtx.Now.Sub(s.since) >= TooltipDelay:
		s.showing = true
		s.fade.From(gtx, 0, th.Dur(kit.DurFast))
	case !s.showing:
		gtx.Execute(op.InvalidateCmd{At: s.since.Add(TooltipDelay)})
	}
	rec := op.Record(gtx.Ops)
	dims := tt.Content(gtx)
	child := rec.Stop()
	area := clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops)
	s.hover.Add(gtx.Ops) // an ancestor of the content's areas, so clicks still reach it
	child.Add(gtx.Ops)
	area.Pop()
	if !s.showing {
		return dims
	}
	a := s.fade.To(gtx, 1, th.Dur(kit.DurFast))
	kit.Layer(gtx, image.Pt(0, dims.Size.Y+gtx.Dp(th.Space(kit.Space1p5))), func(gtx C) D {
		defer paint.PushOpacity(gtx.Ops, a).Pop()
		gtx.Constraints.Min = image.Point{}
		return th.Styled(kit.Surface{Bg: th.Overlay, Border: th.Border, Radius: th.Radius}).Layout(gtx, func(gtx C) D {
			return layout.Inset{Top: th.Space(kit.Space1), Bottom: 5, Left: th.Space(kit.Space2), Right: th.Space(kit.Space2)}.Layout(gtx, func(gtx C) D {
				return th.Text(gtx, tt.Text, th.Type.Small, th.Fg, kit.Regular)
			})
		})
	})
	return dims
}

// ToastDuration is how long a toast stays before it dismisses itself.
const ToastDuration = 5 * time.Second

// Toasts is a stack of transient notifications shown by a ToastHost. Push
// from the handle pass or a Spawn callback; the UI goroutine owns it.
type Toasts struct {
	items []*toast
	seq   int
	close kit.Pool[kit.Clickable]
}

type toast struct {
	id          string
	status      kit.Status
	title, body string
	at          time.Time // first shown (stamped at layout, on the frame clock)
	fade        kit.Tween
}

// Push adds a toast; it shows on the next frame. Its close button's id is
// "toast:<n>/close", n counting from 1.
func (ts *Toasts) Push(s kit.Status, title, body string) {
	ts.seq++
	ts.items = append(ts.items, &toast{id: "toast:" + strconv.Itoa(ts.seq), status: s, title: title, body: body})
}

// Len is the number of toasts showing.
func (ts *Toasts) Len() int { return len(ts.items) }

// ToastHost lays out Page with the toasts stacked above it, bottom right,
// the newest at the bottom. Each closes itself after ToastDuration, or with
// its close button. Wrap the whole window with it.
type ToastHost struct {
	Page kit.W
}

func (host ToastHost) Layout(gtx C, th *kit.Theme, ts *Toasts) D {
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
	slide := gtx.Dp(th.Space(kit.Space6)) // how far a new toast slides in from
	y := gtx.Constraints.Max.Y - margin
	for i := len(ts.items) - 1; i >= 0; i-- {
		it := ts.items[i]
		a := it.fade.To(gtx, 1, th.Dur(kit.DurNormal))
		cgtx := gtx
		cgtx.Constraints = layout.Exact(image.Pt(width, 0))
		cgtx.Constraints.Max.Y = gtx.Constraints.Max.Y
		rec := op.Record(gtx.Ops)
		cd := toastCard(cgtx, th, ts, it)
		call := rec.Stop()
		y -= cd.Size.Y
		at := image.Pt(gtx.Constraints.Max.X-margin-width+int(float32(slide)*(1-a)), y)
		kit.Layer(gtx, at, func(gtx C) D {
			defer paint.PushOpacity(gtx.Ops, a).Pop()
			call.Add(gtx.Ops)
			return cd
		})
		y -= gtx.Dp(th.Space(kit.Space2))
	}
	return dims
}

func toastCard(gtx C, th *kit.Theme, ts *Toasts, it *toast) D {
	fg, _, icon := th.StatusColors(it.status)
	return th.FloatingPanel(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return kit.Group(gtx, it.id, it.title, func(gtx C) D { return toastBody(gtx, th, ts, it, fg, icon) })
	})
}

func toastBody(gtx C, th *kit.Theme, ts *Toasts, it *toast, fg color.NRGBA, icon kit.Icon) D {
	return layout.Inset{Top: th.Space(kit.Space2p5), Bottom: th.Space(kit.Space2p5), Left: th.Space(kit.Space3), Right: th.Space(kit.Space2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Start}.Layout(gtx,
			kit.Rigid(func(gtx C) D {
				return layout.Inset{Top: 1, Right: th.Space(kit.Space2p5)}.Layout(gtx, func(gtx C) D { return icon(gtx, th.IconSizeLg, fg) })
			}),
			kit.Flexed(1, func(gtx C) D {
				return kit.Col(gtx,
					kit.Rigid(func(gtx C) D { return th.Text(gtx, it.title, th.Type.Body, th.Fg, kit.SemiBold) }),
					kit.Rigid(func(gtx C) D {
						if it.body == "" {
							return D{}
						}
						return layout.Inset{Top: 2}.Layout(gtx, func(gtx C) D { return th.Paragraph(gtx, it.body, th.Muted) })
					}),
				)
			}),
			kit.Rigid(func(gtx C) D {
				return IconButton{ID: kit.Part(it.id, "close"), Icon: kit.IconX, Label: th.Strings.Close, Active: true}.Layout(gtx, th, ts.close.Get(it.id))
			}),
		)
	})
}
