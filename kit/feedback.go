package kit

import (
	"image"
	"image/color"
	"time"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"
	"github.com/hhhapz/giokit/gio/unit"
)

// Progress bar geometry and timing.
const (
	progressHeight unit.Dp = 4
	// progressPeriod is one sweep of the indeterminate segment.
	progressPeriod = 1400 * time.Millisecond
	// progressSegment is the indeterminate segment's share of the width.
	progressSegment = 3 // a third
)

// Spinner geometry and timing, on the icon design grid.
const (
	spinnerPeriod = 900 * time.Millisecond // one turn
	spinnerWidth  = 2.5                    // line width, grid units
	spinnerRadius = 9                      // around the grid's center
	spinnerSweep  = 100                    // degrees of the moving arc
	spinnerStart  = -90                    // degrees: the arc starts at 12 o'clock
	// spinnerTrackAlpha is the alpha of the full ring behind the arc.
	spinnerTrackAlpha = 50
)

// Progress is a thin full-width bar. value in 0..1; a negative value draws an
// indeterminate bar (a sliding segment; static when Motion is 0).
func (t *Theme) Progress(gtx layout.Context, value float32) layout.Dimensions {
	h := gtx.Dp(progressHeight)
	w := gtx.Constraints.Max.X
	sz := image.Pt(w, h)
	paint.FillShape(gtx.Ops, t.Surface, clip.UniformRRect(image.Rectangle{Max: sz}, h/2).Op(gtx.Ops))
	var x0, x1 int
	if value < 0 {
		p := Loop(gtx, t.Dur(progressPeriod))
		seg := w / progressSegment
		x0 = int(float32(w+seg)*EaseInOut(p)) - seg
		x1 = x0 + seg
	} else {
		x1 = int(float32(w) * min(1, value))
	}
	defer clip.UniformRRect(image.Rectangle{Max: sz}, h/2).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, t.Accent, clip.Rect(image.Rect(max(0, x0), 0, min(w, x1), h)).Op())
	return layout.Dimensions{Size: sz}
}

// Spinner is a rotating arc for "working…" states, size×size. It schedules
// frames with Loop, so a UI showing it still settles in kittest.
func (t *Theme) Spinner(gtx layout.Context, size unit.Dp) layout.Dimensions {
	p := Loop(gtx, t.Dur(spinnerPeriod))
	const mid = iconGrid / 2
	arc(gtx, size, WithAlpha(t.Accent, spinnerTrackAlpha), spinnerWidth, mid, mid, spinnerRadius, 0, 360)
	return arc(gtx, size, t.Accent, spinnerWidth, mid, mid, spinnerRadius, 360*p+spinnerStart, spinnerSweep)
}

// Status selects colors for banners and badges.
type Status int

const (
	StatusInfo Status = iota // neutral information (accent colours)
	StatusGood               // success, done
	StatusWarn               // needs attention
	StatusBad                // failure, error, destructive
)

// StatusColors are a status's foreground, background and icon (badges,
// banners, toasts). A value outside the defined statuses draws as
// StatusInfo.
func (t *Theme) StatusColors(s Status) (fg, bg color.NRGBA, icon Icon) {
	switch s {
	case StatusGood:
		return t.Good, t.GoodBg, IconCircleCheck
	case StatusWarn:
		return t.Warn, t.WarnBg, IconAlert
	case StatusBad:
		return t.Bad, t.BadBg, IconCircleX
	default: // StatusInfo
		return t.Accent, t.AccentBg, IconInfo
	}
}
