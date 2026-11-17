package kit

import (
	"image"
	"image/color"
	"time"

	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Progress is a thin full-width bar. value in 0..1; a negative value draws an
// indeterminate bar (a sliding segment; static when Motion is 0).
func (t *Theme) Progress(gtx C, value float32) D {
	h := gtx.Dp(4)
	w := gtx.Constraints.Max.X
	sz := image.Pt(w, h)
	paint.FillShape(gtx.Ops, t.Surface, clip.UniformRRect(image.Rectangle{Max: sz}, h/2).Op(gtx.Ops))
	var x0, x1 int
	if value < 0 {
		p := Loop(gtx, t.Dur(1400*time.Millisecond))
		seg := w / 3
		x0 = int(float32(w+seg)*EaseInOut(p)) - seg
		x1 = x0 + seg
	} else {
		x1 = int(float32(w) * min(1, value))
	}
	defer clip.UniformRRect(image.Rectangle{Max: sz}, h/2).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, t.Accent, clip.Rect(image.Rect(max(0, x0), 0, min(w, x1), h)).Op())
	return D{Size: sz}
}

// Spinner is a rotating arc for "working…" states. It schedules frames with
// Loop, so a UI showing it still settles in kittest.
func (t *Theme) Spinner(gtx C, size unit.Dp) D {
	p := Loop(gtx, t.Dur(900*time.Millisecond))
	arc(gtx, size, WithAlpha(t.Accent, 50), 2.5, 12, 12, 9, 0, 360)
	return arc(gtx, size, t.Accent, 2.5, 12, 12, 9, 360*p-90, 100)
}

// Status selects colors for banners and badges.
type Status int

const (
	StatusInfo Status = iota
	StatusGood
	StatusWarn
	StatusBad
)

// StatusColors are a status's foreground, background and icon (badges,
// banners, toasts).
func (t *Theme) StatusColors(s Status) (fg, bg color.NRGBA, icon Icon) {
	switch s {
	case StatusGood:
		return t.Good, t.GoodBg, IconCircleCheck
	case StatusWarn:
		return t.Warn, t.WarnBg, IconAlert
	case StatusBad:
		return t.Bad, t.BadBg, IconCircleX
	}
	return t.Accent, t.AccentBg, IconInfo
}
