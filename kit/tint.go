package kit

import "image/color"

// PressTint is how far a pressed control's fill moves toward Fg (hover goes
// half way).
const PressTint = 0.16

// StateColor is a control's fill base as drawn in st: hover moves it half
// way to a press, a press all the way. Buttons and fields use it.
func (t *Theme) StateColor(base color.NRGBA, st State) color.NRGBA {
	amt := PressTint / 2 * st.Hover
	if st.Pressed {
		amt = PressTint
	}
	return t.Tint(base, amt)
}

// Highlight is the background of a row or cell lit by lit, from 0 (none)
// to 1: hover animates it, focus, press or a keyboard choice light it
// fully. Values outside 0..1 are clamped. Menu rows, calendar cells and
// list rows use it.
func (t *Theme) Highlight(lit float32) color.NRGBA {
	return WithAlpha(t.Tint(t.Overlay, PressTint), opacity(lit))
}

// Tint mixes c toward the foreground by amount, from 0 to 1 (PressTint is
// a full press): a hover/press colour that works on light and dark themes
// alike. A transparent c gives a translucent foreground. Values outside
// 0..1 are clamped.
func (t *Theme) Tint(c color.NRGBA, amount float32) color.NRGBA {
	if c.A == 0 {
		return WithAlpha(t.Fg, opacity(amount))
	}
	return LerpColor(c, t.Fg, amount)
}

// opacity converts f, clamped to 0..1 (NaN counts as 0), to an 8-bit
// alpha, truncating.
func opacity(f float32) uint8 {
	switch {
	case !(f > 0):
		return 0
	case f >= 1:
		return 255
	}
	return uint8(f * 255)
}
